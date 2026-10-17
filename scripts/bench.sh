#!/bin/bash
# Benchmark harness for the transfer paths.
#
# Measures wall clock for the operations whose speed a user notices: CSV export,
# CSV import and a database migration, each next to the server-side COPY or
# pg_dump/pg_restore that is its floor. Prints a markdown table.
#
# Usage: scripts/bench.sh
#
# Requires the test stack from scripts/docker-compose.yml, with a bench_users
# table seeded. BENCHMARKS.md has the seed statement.

# No errexit: a step that fails should be reported, not abort the run, since
# the numbers recorded in BENCHMARKS.md are the comparison being made.
set -uo pipefail

DB_HOST="${PGTRANSFER_TEST_DB_HOST:-localhost}"
DB_PORT="${PGTRANSFER_TEST_DB_PORT:-5433}"
DB_USER="${PGTRANSFER_TEST_DB_USER:-testuser}"
DB_PASS="${PGTRANSFER_BENCH_PASSWORD:-testpass}"
DB_NAME="${PGTRANSFER_TEST_DB_NAME:-testdb}"
DEST_NAME="bench_target"
MIGRATE_NAME="bench_migrate"

WORK="$(mktemp -d)"
cleanup() {
  for db in "$DEST_NAME" "$MIGRATE_NAME"; do
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -qtAc \
      "DROP DATABASE IF EXISTS $db" >/dev/null 2>&1 || true
  done
  rm -rf "$WORK"
}
trap cleanup EXIT

export PGPASSWORD="$DB_PASS"

BIN="$WORK/pgtransfer"
echo "building..." >&2
( cd "$(dirname "$0")/.." && go build -o "$BIN" . )

ROWS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
  -qtAc "select count(*) from bench_users")

psql_src() { psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -qtAc "$1"; }
psql_dst() { psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DEST_NAME" -qtAc "$1"; }

# Recreate a database with an empty copy of the source table, so every run
# starts from the same state.
reset_db() {
  local db="$1"
  psql_src "DROP DATABASE IF EXISTS $db" >/dev/null 2>&1 || true
  psql_src "CREATE DATABASE $db" >/dev/null
  pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -s -t bench_users "$DB_NAME" \
    | psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$db" -q >/dev/null
}

# The CLI reaches a database through a profile, so point HOME at one.
export HOME="$WORK/home"
mkdir -p "$HOME/.pgtransfer"
cat > "$HOME/.pgtransfer/config.yaml" <<EOF
active_profile: bench
profiles:
  bench:
    name: bench
    host: $DB_HOST
    port: $DB_PORT
    database: $DB_NAME
    user: $DB_USER
    password: $DB_PASS
    sslmode: disable
  bench_target:
    name: bench_target
    host: $DB_HOST
    port: $DB_PORT
    database: $DEST_NAME
    user: $DB_USER
    password: $DB_PASS
    sslmode: disable
  bench_migrate:
    name: bench_migrate
    host: $DB_HOST
    port: $DB_PORT
    database: $MIGRATE_NAME
    user: $DB_USER
    password: $DB_PASS
    sslmode: disable
EOF
chmod 600 "$HOME/.pgtransfer/config.yaml"

now() { python3 -c 'import time;print(time.time())'; }

printf '| Operation | Time | Rows/s |\n| --- | --- | --- |\n'

report() {
  local label="$1" start="$2"
  python3 - "$label" "$start" "$ROWS" <<'PY'
import sys, time
label, start, rows = sys.argv[1], float(sys.argv[2]), int(sys.argv[3])
elapsed = time.time() - start
rate = rows / elapsed if elapsed > 0 else 0
print("| %s | %.2fs | %s |" % (label, elapsed, format(int(rate), ",")))
PY
}

EXPORT="$WORK/out.csv"

# --- CSV export -------------------------------------------------------------
start=$(now)
"$BIN" export csv bench --overwrite bench_users "$EXPORT" >/dev/null 2>&1
report "export csv" "$start"

start=$(now)
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
  -c "\\copy bench_users to '$WORK/copy.csv' csv header" >/dev/null
report "COPY TO (floor)" "$start"

# --- CSV import -------------------------------------------------------------
reset_db "$DEST_NAME"
start=$(now)
"$BIN" import csv bench_target bench_users "$EXPORT" >/dev/null 2>&1
report "import csv" "$start"
IMPORTED=$(psql_dst "select count(*) from bench_users")

reset_db "$DEST_NAME"
start=$(now)
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DEST_NAME" \
  -c "\\copy bench_users from '$EXPORT' csv header" >/dev/null
report "COPY FROM (floor)" "$start"

# --- migrate database -------------------------------------------------------
reset_db "$MIGRATE_NAME"
start=$(now)
"$BIN" migrate database bench bench_migrate >/dev/null 2>&1
report "migrate database" "$start"
MIGRATED=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$MIGRATE_NAME" \
  -qtAc "select count(*) from bench_users")

start=$(now)
pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -Fc -Z1 \
  --file="$WORK/raw.dump" "$DB_NAME" >/dev/null 2>&1
reset_db "$MIGRATE_NAME"
pg_restore -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$MIGRATE_NAME" \
  "$WORK/raw.dump" >/dev/null 2>&1
report "pg_dump + pg_restore (floor)" "$start"

echo
echo "rows in source: $ROWS"
echo "export size: $(du -h "$EXPORT" | cut -f1)"
echo "rows imported: $IMPORTED / migrated: $MIGRATED"

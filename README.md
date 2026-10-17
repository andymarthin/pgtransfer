# PGTransfer

[![Go Version](https://img.shields.io/badge/Go-1.25+-blue.svg)](https://golang.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-13+-blue.svg)](https://postgresql.org)
[![CI](https://github.com/andymarthin/pgtransfer/actions/workflows/ci.yml/badge.svg)](https://github.com/andymarthin/pgtransfer/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Move PostgreSQL data between databases without writing scripts. Export tables to
CSV, dump and restore whole databases, and migrate between environments — through
an SSH bastion if you need one.

```bash
pgtransfer profile add prod          # save a connection once
pgtransfer export csv prod users users.csv
pgtransfer migrate database prod staging
```

---

## Contents

- [Install](#install)
- [Quick start](#quick-start)
- [Commands](#commands)
  - [Profiles](#profiles)
  - [Test a connection](#test-a-connection)
  - [Export to CSV](#export-to-csv)
  - [Import from CSV](#import-from-csv)
  - [Dump a database](#dump-a-database)
  - [Restore a dump](#restore-a-dump)
  - [Migrate between databases](#migrate-between-databases)
  - [Roll back a migration](#roll-back-a-migration)
  - [Read the operation log](#read-the-operation-log)
- [SSH tunnels](#ssh-tunnels)
- [JSON output and MCP](#json-output-and-mcp)
- [Data types and CSV format](#data-types-and-csv-format)
- [Performance](#performance)
- [Configuration](#configuration)
- [Security notes](#security-notes)
- [Development](#development)
- [License](#license)

---

## Install

### Prebuilt binary

Download the archive for your platform from the
[releases page](https://github.com/andymarthin/pgtransfer/releases/latest):

| Platform | Asset |
| --- | --- |
| macOS (Apple Silicon) | `pgtransfer-<version>-darwin-arm64.tar.gz` |
| macOS (Intel) | `pgtransfer-<version>-darwin-amd64.tar.gz` |
| Linux x86-64 | `pgtransfer-<version>-linux-amd64.tar.gz` |
| Linux ARM64 | `pgtransfer-<version>-linux-arm64.tar.gz` |

```bash
V=v1.2.0
A=darwin-arm64                       # linux-amd64, linux-arm64, darwin-amd64
curl -fsSLO "https://github.com/andymarthin/pgtransfer/releases/download/$V/pgtransfer-$V-$A.tar.gz"
tar xzf "pgtransfer-$V-$A.tar.gz"
sudo mv pgtransfer /usr/local/bin/
pgtransfer version
```

Verify against the published checksums if you want to be sure:

```bash
curl -fsSLO "https://github.com/andymarthin/pgtransfer/releases/download/$V/SHA256SUMS"
shasum -a 256 -c SHA256SUMS --ignore-missing   # Linux: sha256sum -c ...
```

### Install script

Downloads the right binary, verifies it against the release checksums, and
installs it.

```bash
curl -fsSL https://raw.githubusercontent.com/andymarthin/pgtransfer/main/scripts/install.sh | sh
```

Installs to `/usr/local/bin` when writable, otherwise to `~/.local/bin` (and tells
you how to add it to `PATH`). The script also checks for `pg_dump`, `pg_restore`,
`psql` and an OpenSSH client, and prints the install command for your platform if
any are missing.

| Variable | Purpose |
| --- | --- |
| `PGTRANSFER_VERSION` | Release tag to install, e.g. `v1.2.0` (default: latest) |
| `PGTRANSFER_INSTALL` | Install directory (default: `/usr/local/bin`, else `~/.local/bin`) |
| `PGTRANSFER_NO_SUDO` | Set to `1` to never use `sudo` |

Read it before piping to a shell if you prefer:

```bash
curl -fsSL https://raw.githubusercontent.com/andymarthin/pgtransfer/main/scripts/install.sh | less
```

### Homebrew

```bash
brew install andymarthin/pgtransfer/pgtransfer
```

Upgrade later with `brew update && brew upgrade pgtransfer`.

### From source

Requires Go 1.25 or higher.

```bash
go install github.com/andymarthin/pgtransfer@latest   # into $(go env GOPATH)/bin
```

or clone and build:

```bash
git clone https://github.com/andymarthin/pgtransfer.git
cd pgtransfer
go build -o pgtransfer .
```

### Requirements

`pgtransfer` itself needs nothing beyond a PostgreSQL server. Two optional
dependencies, checked by the install script:

- **`pg_dump` / `pg_restore` / `psql`** — needed by `export dump`, `import dump`
  and `migrate database`. CSV export and import do not use them.
- **An OpenSSH client** — needed only for SSH tunnels.

Client tools should be the same major version as the server, or newer.

---

## Quick start

```bash
# 1. Save a connection
pgtransfer profile add prod

# 2. Check it works
pgtransfer test-connection prod

# 3. Export a table
pgtransfer export csv prod users users.csv

# 4. Copy a whole database to staging
pgtransfer migrate database prod staging
```

Every command accepts `--json` for scripted use, and `--help` for its full flag
list:

```bash
pgtransfer --help
pgtransfer export csv --help
```

---

## Commands

### Profiles

A profile is a saved connection stored in `~/.pgtransfer/config.yaml`. Create one
interactively:

```bash
pgtransfer profile add prod
```

You will be prompted for the database host, port, name, user, password and SSL
mode, and optionally for SSH tunnel settings. Every prompt shows a default in
brackets; press Enter to accept it. Port and timeout answers are validated as
you give them, and a password prompt that cannot read from a terminal fails
instead of saving an empty password — pass `--password` when the command runs
without one.

Create one in a single command instead:

```bash
pgtransfer profile add prod \
  --user app_user \
  --password 'secret' \
  --host db.example.com \
  --port 5432 \
  --database myapp \
  --sslmode require
```

Passwords and database names are URL-encoded when the profile is turned into a
connection string, so spaces and other special characters work without escaping.
An IPv6 host is written in brackets automatically.

List, inspect and remove profiles:

```bash
pgtransfer profile list
pgtransfer profile list --json          # includes has_password, never the password
pgtransfer profile delete prod
```

Passing an existing profile name to `profile add` shows the current values as
defaults and asks before overwriting. `--force` skips the prompt, `--skip-test`
skips the connection check afterwards.

### Test a connection

```bash
pgtransfer test-connection prod
```

Reports whether the connection is direct or tunnelled, the server version, the
latency, and — for a tunnel — the local port you can point other tools at.

```bash
pgtransfer test-connection prod --json
```

### Export to CSV

```bash
# Whole table
pgtransfer export csv prod public.users users.csv

# Positional form also works: [profile] [table] [output]
pgtransfer export csv prod users users.csv

# No header row
pgtransfer export csv prod users users.csv --headers=false

# Another schema
pgtransfer export csv prod users users.csv --schema reporting

# Only some rows
pgtransfer export csv prod active.csv \
  --query "SELECT id, email, created_at FROM users WHERE active = true"
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `--headers` | on | Write a header row as the first line |
| `--schema` | `public` | Schema for an unqualified table name |
| `--batch-size` | `20000` | Rows fetched per query |
| `--query` | — | Export the result of a SQL query instead of a table |
| `--overwrite` | off | Replace the file if it already exists |

The output file gets a `.csv` extension if you omit one. Export refuses to
overwrite an existing file unless `--overwrite` is given, and the check applies
to the name it will actually write: exporting to `users` while `users.csv`
exists is refused, because that is the file that would be replaced.

`--schema` supplies a schema for an unqualified table name. Names that need quoting
(mixed case, spaces, reserved words) work whether or not you quote them yourself.

`--query` runs your SQL as written, so it can be any `SELECT` — joins, aggregates,
expressions. It is executed as a single statement.

### Import from CSV

```bash
# Header row names the columns
pgtransfer import csv prod public.users users.csv

# Header-less file: columns are matched by position
pgtransfer import csv prod public.users users.csv --headers=false

# Replace the table contents first
pgtransfer import csv prod public.users users.csv --overwrite
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `--headers` | on | Treat the first row as column names |
| `--schema` | `public` | Schema for an unqualified table name |
| `--batch-size` | `20000` | Rows inserted per transaction |
| `--overwrite` | off | `TRUNCATE` the table before importing |

**Headers behaviour:** with the default, the first row names the columns and is not
imported — column order in the file does not need to match the table. With
`--headers=false`, every row is data and columns are matched positionally against
the target table. The defaults on both `export csv` and `import csv` match, so a
file exported without flags imports without flags.

**Empty fields** become SQL `NULL`, which is what `export csv` writes for a `NULL`.
A genuinely empty string and a `NULL` are indistinguishable in CSV, so an empty
field always round-trips as `NULL`.

Table and column names are quoted for you. A quoted name such as `"public"."Order"`
and a bare one such as `public.order` mean the same thing, so names that are case
sensitive or contain spaces work either way.

**Overwrite** truncates the table before the first batch. If the import then fails,
the table is left empty — check the row count in the result before assuming success.

### Dump a database

Wraps `pg_dump`. Use this when you want a real backup or a full transfer including
sequences, indexes, constraints and ownership.

```bash
# Plain SQL
pgtransfer export dump prod backup_$(date +%Y%m%d).sql

# Custom format, compressed — smaller and restores faster
pgtransfer export dump prod backup.dump --format custom --compress

# Schema only
pgtransfer export dump prod schema.sql --schema-only

# Data only
pgtransfer export dump prod data.sql --data-only

# Selected tables
pgtransfer export dump prod users_backup.sql --table users --table orders
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `--format` | `plain` | `plain`, `custom`, `directory` or `tar` |
| `--compress` | off | Compress the archive (not valid for `plain`) |
| `--schema-only` | off | Structure only, no rows |
| `--data-only` | off | Rows only, no structure |
| `--table` | — | Include a table; repeatable |
| `--exclude-table` | — | Exclude a table; repeatable |
| `--schema` | — | Limit to one schema |
| `--timeout` | none | Abort after this many seconds |
| `--overwrite` | off | Replace an existing output file |
| `--verbose` | off | Print the underlying `pg_dump` command |

Formats in short: `plain` is readable SQL you can edit and feed to `psql`;
`custom` is compressed and lets `pg_restore` pick objects selectively; `directory`
allows parallel restore; `tar` is the legacy archive format.

The dump file is flushed to disk before the command reports success, and a dump
that fails partway is deleted rather than left behind as a truncated file that
looks like a backup. `--verbose` prints the `pg_dump` command with the password
in the connection URL replaced by `***`.

### Restore a dump

```bash
pgtransfer import dump staging backup.sql

# Custom format, dropping existing objects first
pgtransfer import dump staging backup.dump --clean

# Fail on the first error instead of tolerating version skew
pgtransfer import dump staging backup.sql --strict --timeout 300

# See what pg_restore is doing
pgtransfer import dump staging backup.dump --verbose
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `--clean` | off | Drop existing objects before restoring (binary formats) |
| `--strict` | off | Stop at the first error |
| `--timeout` | none | Abort after this many seconds |
| `--verbose` | off | Print the underlying command |

The format is detected from the file, so you do not have to say whether it is plain
SQL or a `pg_dump` archive.

Without `--strict`, a restore is allowed to report problems that come from client and
server version differences — an older `psql` meeting a newer dump, for example — and
still counts as successful. The summary says how many messages were ignored. Anything
that is not a known version-skew message (a failed `COPY`, a permission error, a missing
table) fails the command and is reported, even without `--strict`. Use `--strict` when a
partially restored database is worse than no database.

### Migrate between databases

Copies a whole database from one place to another. Two ways to name the endpoints:

```bash
# Two profiles
pgtransfer migrate database prod staging

# One profile, two database names on the same server
pgtransfer migrate database dev --source-database app_old --target-database app_new

# Mixed: override one side of a two-profile migration
pgtransfer migrate database prod staging --target-database staging_copy
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `--schema-only` | off | Structure only |
| `--data-only` | off | Rows only; the target schema must already exist |
| `--tables` | all | Comma-separated list of tables |
| `--overwrite` | off | Replace existing objects in the target |
| `--strict` | off | Abort on the first restore error instead of tolerating it |
| `--dry-run` | off | Report what would happen and change nothing |
| `--validate` | off | Pre-flight check: reach both ends, and confirm any `--tables` exist in the source |
| `--enable-rollback` | off | Back up the target before migrating, so you can roll back |
| `--timeout` | `3600` | Abort after this many seconds |
| `--batch-size` | `1000` | Rows per batch for data migration |
| `--verbose` | off | Print the underlying commands |

```bash
# Schema only, for a fresh development database
pgtransfer migrate database prod dev --schema-only

# Some tables, with a backup you can roll back to
pgtransfer migrate database prod staging --tables users,orders --enable-rollback

# Fail before touching anything if a table is missing
pgtransfer migrate database prod staging --tables users,orders --validate

# Treat any restore error as a failed migration
pgtransfer migrate database prod staging --strict

# See what would happen first
pgtransfer migrate database prod staging --dry-run
```

#### Check the plan first

`--dry-run` answers "what is about to happen to the target?" without touching
either database. It connects, reads the table lists and column types, and prints
the plan:

```bash
pgtransfer migrate database prod staging --dry-run --overwrite
```

```
🔍 Migration plan (dry run)
From:      prod
To:        staging
Mode:      schema and data
Overwrite: on - existing target tables are dropped and recreated
Strict:    false

TABLE          ROWS     TARGET
orders         842,113  dropped and recreated
products         1,204  dropped and recreated
users          250,000  dropped and recreated
```

It reports three things you cannot see by reading the flags:

- **What `--overwrite` would drop.** Existing target tables are listed as
  "dropped and recreated", so a migration aimed at the wrong database is visible
  before it runs.
- **Which columns have incompatible types.** A differing type is not fixed by the
  restore — an existing target column keeps its type — so the copy either fails or
  coerces values. These are listed with both types.
- **Tables present on one side only.** A table that exists only in the source with
  `--data-only` is a guaranteed failure, and the report says so.

Row counts come from the planner's statistics, so they are estimates, and they are
free: counting exactly would read every table, which is the cost a dry run exists
to avoid.

With `--json` the report is a structured object under `data.report`:

```bash
pgtransfer --json migrate database prod staging --dry-run 2>/dev/null \
  | jq '.data.report.type_conflicts'
```

`2>/dev/null` drops the connection chatter, which goes to stderr; stdout carries
exactly one JSON object.

The data reaches the target by dumping the source and restoring the dump, so the
restore flags apply here too: without `--strict` a version-skew message is counted
as a warning, and `--overwrite` drops existing objects before restoring.

### Roll back a migration

```bash
pgtransfer migrate rollback staging /path/to/backup.dump
```

Restores a backup taken before a migration, undoing the changes it made. The backup
is the file named by `--enable-rollback` when the migration ran, under
`~/.pgtransfer/backups/`.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--strict` | off | Abort on the first restore error instead of tolerating it |
| `--timeout` | `0` | Abort after this many seconds; `0` means no limit |
| `--verbose` | off | Print the underlying restore command |

`--strict` matters most here: a rollback that carries on past a failed statement
leaves the target in a state that is neither the old one nor the new one. Pass it
when a partial rollback would be worse than a failed one.

```bash
# Stop at the first error rather than leaving the database half-restored
pgtransfer migrate rollback staging ~/.pgtransfer/backups/rollback_staging_20261017_091500.sql --strict
```

### Read the operation log

```bash
pgtransfer log
```

`migrate database` records what it did in `~/.pgtransfer/logs/`, one file per day.
This reads it back: today by default, or a day you name.

```bash
pgtransfer log --date 2026-10-17       # a specific day
pgtransfer log --limit 5               # the last 5 entries today
pgtransfer --json log                  # machine-readable
```

```
📘 Operations on 2026-10-17

TIME      STATUS  COMMAND           PROFILE  DETAIL
16:15:00  ok      migrate database  prod     migrated public to staging (4.2s)
16:20:00  fail    export csv        prod     connection reset
```

With `--json` you get `date`, `total` and an `entries` array (always an array, even
when empty), so a script can assert on what the last run did:

```bash
pgtransfer --json log --limit 1 | jq -e '.data.entries[0].status == "success"'
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `--date` | today | Day to show, as `YYYY-MM-DD` |
| `--limit` | all | Show only the last N entries |

A day with no file is reported as no operations, not an error. If today is empty,
the command names the days that do have entries so you are not left guessing. A
single malformed line is skipped rather than hiding the rest of the day.

---

## SSH tunnels

A profile can reach its database through a bastion host. `pgtransfer` opens the
tunnel for the duration of the command and closes it afterwards — nothing to
supervise.

The tunnel binds a free port on `127.0.0.1` and starts forwarding before the
command runs, so a tunnel that cannot be set up fails immediately and names the
reason (unreachable bastion, rejected credentials) rather than leaving `pg_dump`
or `pg_restore` to time out against a port that was never listening.

```bash
pgtransfer profile add prod \
  --host localhost \
  --port 5432 \
  --database myapp \
  --user app_user \
  --ssh-host bastion.example.com \
  --ssh-user deploy \
  --ssh-key ~/.ssh/production_key
```

The database host is resolved **from the bastion's point of view**. `localhost`
means the bastion itself; use the database's real hostname or private IP if it is
elsewhere on that network.

Three ways to authenticate to the bastion, tried in this order:

| Method | How | Notes |
| --- | --- | --- |
| SSH agent | `ssh-add` (uses `SSH_AUTH_SOCK`) | Tried automatically; no flag needed |
| Key file | `--ssh-key ~/.ssh/id_rsa` | Add `--ssh-passphrase` if the key is encrypted |
| Password | `--ssh-password` | Least secure; omit the flag to be prompted |

Verify a tunnel:

```bash
pgtransfer test-connection prod
```

The output includes the local port the tunnel is forwarding on, e.g.
`localhost:5433`, so you can point `psql`, pgAdmin or DBeaver at the same database
while that command runs.

---

## JSON output and MCP

Both are for programs rather than people: one for scripts and CI, one for AI agents.

### JSON output

Pass `--json` before the subcommand, or set `PGTRANSFER_JSON=1`:

```bash
pgtransfer --json export csv prod users users.csv
PGTRANSFER_JSON=1 pgtransfer profile list
```

Every command returns the same envelope:

```json
{
  "ok": true,
  "command": "export csv",
  "data": { "rows": 250, "output_file": "users.csv" },
  "error": null
}
```

On failure `ok` is `false`, `data` is omitted, and `error` carries a stable code:

```json
{
  "ok": false,
  "command": "test-connection",
  "error": {
    "code": "profile_not_found",
    "message": "profile \"ghost\" not found"
  }
}
```

Exactly one object is written per invocation — including for usage errors such as an
unknown subcommand or a bad flag — and the exit status is non-zero on failure.

Codes you can branch on: `usage_error`, `profile_not_found`, `no_active_profile`,
`config_load_failed`, `connection_failed`, `export_failed`, `import_failed`,
`dump_failed`, `restore_failed`, `migration_failed`, `dry_run_failed`, `rollback_failed`,
`file_exists`, `file_not_found`, `truncate_failed`, `invalid_input`. `pgtransfer log`
adds `usage_error` for a malformed `--date`.

Passwords never appear in JSON output. Profiles report `has_password: true/false`.

### MCP server

`pgtransfer mcp` runs a [Model Context Protocol](https://modelcontextprotocol.io)
server over stdio, exposing the commands above as typed tools that an AI host can
call directly.

```bash
pgtransfer mcp
```

For Claude Desktop, add this to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "pgtransfer": {
      "command": "pgtransfer",
      "args": ["mcp"]
    }
  }
}
```

| Tool | Purpose |
| --- | --- |
| `list_profiles` | List saved profiles (never returns passwords) |
| `test_connection` | Test connectivity, optionally through a tunnel |
| `export_csv` | Export a table or a query to CSV |
| `import_csv` | Import a CSV, optionally truncating first |
| `export_dump` | Create a `pg_dump` backup |
| `import_dump` | Restore a dump |
| `migrate_database` | Migrate between databases or profiles |

The CSV tools take the same defaults as the CLI: `headers` is on unless set to
false, and `batch_size` is 20000. `import_dump` and `migrate_database` both take
`strict`, with the same meaning as `--strict` on the commands they mirror.

Tool failures come back in-band with `isError: true` and the same
`{"ok": false, "error": {...}}` payload, so a model can read the problem and correct
itself rather than seeing an opaque protocol error.

stdout carries the protocol, so any incidental library output is redirected to
stderr. Only protocol frames are ever written to stdout.

---

## Data types and CSV format

Values are written in a form PostgreSQL reads back without help.

| PostgreSQL type | Written as | Example |
| --- | --- | --- |
| `date` | `YYYY-MM-DD` | `2008-07-06` |
| `timestamp` | `YYYY-MM-DD HH:MM:SS` | `2025-10-28 01:59:38` |
| `timestamptz` | Same, converted to the session time zone | `2025-10-28 01:59:38` |
| `numeric` / `decimal` | Decimal notation, scale preserved | `63942.00` |
| `integer` / `bigint` | Plain number | `457719` |
| `real` / `double precision` | Shortest round-trippable form | `3.14159` |
| `text` / `varchar` | As-is, quoted only when the value needs it | `user_0457719` |
| `boolean` | `true` / `false` | `true` |
| `NULL` | Empty field | |

A value that is exactly midnight is written as a bare date (`2008-07-06`), which
PostgreSQL reads back into either a `date` or a `timestamp` column.

Round-tripping through `export csv` and `import csv` preserves all of these,
including `NULL` versus empty string (see [Import from CSV](#import-from-csv)).

Sub-second precision is **not** preserved: timestamps are written to whole seconds.
If you need exact values, use `export dump`, which uses `COPY` and is byte-exact.

---

## Performance

Export is paginated by the table's primary key rather than by `OFFSET`, so a large
export costs the same per row at the end of the table as at the start.

Measured on a 5,000,000 row, 8-column table on an M1 Pro with PostgreSQL 15:

| Batch size | Time | Throughput |
| --- | --- | --- |
| 500 | 15.3 s | ~327,000 rows/s |
| 20,000 (default) | **9.8 s** | ~510,000 rows/s |
| 50,000 | 5.5 s | ~909,000 rows/s |

Memory stays flat at about 20–25 MB regardless of table size, because batches are
streamed to disk rather than buffered. For reference, server-side
`COPY ... TO STDOUT` on the same table takes 5.3 s — the practical floor for a
client that has to receive, format and write every value.

Practical notes:

- **Raise `--batch-size` on a fast local network.** 50,000 is usually faster than
  the default. Lower it on a high-latency link where a big batch risks a timeout.
- **Import is slower than export.** Each row is a parameterised `INSERT`, batched
  per transaction. Raise `--batch-size` for bulk loads into an empty table.
- **`export dump` is the fastest way to move everything.** It uses `pg_dump` and
  `COPY`, and restores in parallel with `--format directory`.
- **CSV export is for interchange, not backup.** It carries data only — no indexes,
  constraints, sequences or ownership. Use `export dump` for a real backup.

Full numbers, including per-type formatting costs, are in
[BENCHMARKS.md](BENCHMARKS.md).

---

## Configuration

Profiles live in `~/.pgtransfer/config.yaml`:

```yaml
profiles:
  local:
    name: local
    user: postgres
    host: localhost
    port: 5432
    database: myapp_dev
    sslmode: disable
    ssh:
      enabled: false

  production:
    name: production
    user: app_user
    host: localhost
    port: 5432
    database: myapp_prod
    sslmode: require
    ssh:
      enabled: true
      host: bastion.example.com
      user: ubuntu
      port: 22
      key_path: /home/user/.ssh/id_rsa
      timeout: 10
```

The file is created with mode `0600`. Prefer `profile add` over editing it by hand —
that way the password is entered without being echoed and the connection is tested.

### Logging

`migrate database` appends an entry to `~/.pgtransfer/logs/`, one file per day:

```json
{
  "timestamp": "2024-01-15T10:30:45Z",
  "command": "migrate database",
  "profile": "production",
  "status": "success",
  "message": "Successfully migrated to staging",
  "duration": "1.25s"
}
```

Passwords are never written to the log. Read it back with
[`pgtransfer log`](#read-the-operation-log); the files are plain JSONL, so `tail -f`
works too.

---

## Security notes

- **Credentials** are stored in `~/.pgtransfer/config.yaml` with mode `0600`, and
  never echoed to the terminal, written to logs, or included in `--json` or MCP
  output. `--verbose` prints the underlying `pg_dump` command with the password in
  the connection URL replaced by `***`.
- **Table and column names** are quoted as SQL identifiers wherever they reach a
  statement, so a name containing a quote, a semicolon or SQL text cannot alter the
  statement it appears in.
- **SSH host keys** are not currently verified — connections accept any host key, to
  keep first-time setup simple. If you rely on a tunnel to protect production
  credentials, verify the bastion's host key out of band first, or use a network
  path you already trust.
- **`--password` on the command line** is visible to other local users in `ps`.
  Prefer the interactive prompt for profiles you will keep.
- **`--query` runs arbitrary SQL.** It is your SQL and it runs as your user, so it
  carries whatever privileges that user has. The same applies to the MCP
  `export_csv` tool.
- **Backups from `--enable-rollback`** are full copies of the target database and
  contain real data. They are stored under `~/.pgtransfer/backups/`; delete them
  once a migration is confirmed good.

---

## Development

```bash
git clone https://github.com/andymarthin/pgtransfer.git
cd pgtransfer
go mod download
go build -o pgtransfer .
```

```bash
make test        # unit tests, no database needed
make lint        # gofmt + go vet
make bench       # benchmarks
```

### Tests

Unit tests need no database:

```bash
go test ./...
```

Integration tests need a live PostgreSQL and an SSH server, and are driven by
`scripts/docker-compose.yml`:

```bash
cd scripts && docker compose up -d
cd ..
export PGTRANSFER_TEST_DSN='postgres://testuser:testpass@localhost:5433/testdb?sslmode=disable'
export PGTRANSFER_TEST_REQUIRE=1
go test ./internal/io/
```

| Variable | Default | Purpose |
| --- | --- | --- |
| `PGTRANSFER_TEST_DSN` | — | Database DSN; tests skip when unset |
| `PGTRANSFER_TEST_REQUIRE` | *(unset)* | Set to `1` to fail instead of skipping |
| `PGTRANSFER_TEST_SSH_HOST` | *(unset)* | SSH server for tunnel tests |
| `PGTRANSFER_TEST_SSH_PORT` | `2222` | SSH server port |
| `PGTRANSFER_TEST_SSH_USER` | *(unset)* | SSH user |
| `PGTRANSFER_TEST_SSH_PASSWORD` | *(unset)* | SSH password |
| `PGTRANSFER_TEST_DB_HOST` | `postgres` | Database host **as the SSH server sees it** |
| `PGTRANSFER_TEST_DB_PORT` | `5432` | Database port as the SSH server sees it |
| `PGTRANSFER_TEST_DB_USER` | `testuser` | Database user |
| `PGTRANSFER_TEST_DB_PASSWORD` | `testpass` | Database password |
| `PGTRANSFER_TEST_DB_NAME` | `testdb` | Database name |

The compose stack publishes PostgreSQL on `localhost:5433` and the SSH server on
`localhost:2222`:

```bash
cd scripts
docker compose up -d
cd ..
./pgtransfer profile add test-docker \
  --host localhost --port 5433 \
  --user testuser --password testpass \
  --database testdb
./pgtransfer test-connection test-docker
```

Tear down with `docker compose down`, or `docker compose down -v` to remove the data
volume too.

To check a different PostgreSQL major version, run it standalone on its own port:

```bash
docker run -d --name pgt-pg17 \
  -e POSTGRES_DB=testdb -e POSTGRES_USER=testuser -e POSTGRES_PASSWORD=testpass \
  -p 5417:5432 postgres:17-alpine

./pgtransfer profile add pg17 \
  --host localhost --port 5417 \
  --user testuser --password testpass --database testdb
```

Export and import are tested against PostgreSQL 15, 16, 17 and 18. Client tools
(`pg_dump`, `pg_restore`, `psql`) should match or exceed the server's major version.

### Benchmarks

```bash
make bench            # run
make bench-save       # record a baseline
make bench-compare    # compare against it
```

They exercise formatting and encoding only, and never touch a database — the query
layer is I/O bound and covered by the integration tests instead.

---

## Contributing

Issues and pull requests are welcome. Fork the repository, create a branch, add
tests for your change, and open a PR. `make lint && make test` should pass.

## License

MIT — see [LICENSE](LICENSE).

## Acknowledgments

Built with [Go](https://golang.org/), [PostgreSQL](https://postgresql.org/),
[Cobra](https://github.com/spf13/cobra), [lib/pq](https://github.com/lib/pq),
[golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh) and
[gopkg.in/yaml.v3](https://gopkg.in/yaml.v3), plus the
[Model Context Protocol](https://modelcontextprotocol.io) Go SDK.

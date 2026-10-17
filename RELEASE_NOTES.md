# PGTransfer v1.3.0

Two capabilities the tool was missing: it recorded every migration but could not
show you what it recorded, and it could migrate a database but not tell you what
that would do until it had already done it.

## ✨ New features

### `pgtransfer log`

`migrate database` has always appended an entry per run to `~/.pgtransfer/logs/`,
and nothing read it back — finding out what the last migration did meant knowing
the file naming and `cat`-ing JSONL. `pgtransfer log` shows it:

```
📘 Operations on 2026-10-17

TIME      STATUS  COMMAND           PROFILE  DETAIL
16:15:00  ok      migrate database  prod     migrated public to staging (4.2s)
16:20:00  fail    export csv        prod     connection reset
```

- `--date YYYY-MM-DD` shows a specific day; no flag shows today.
- `--limit N` keeps the newest N entries.
- `--json` returns `{date, total, entries}`, with `entries` always an array —
  `[]`, never `null` — so this works even on a machine that has never migrated:

  ```bash
  pgtransfer --json log --limit 1 | jq -e '.data.entries[0].status == "success"'
  ```

A day with no file is reported as no operations rather than an error, and the
command names the days that do have entries so you aren't left guessing. A single
malformed line is skipped instead of hiding the rest of the day — an interrupted
command must not make the log unreadable.

### `migrate database --dry-run`

Connects to both databases and reports the plan. Nothing is dumped, written or
dropped.

```
🔍 Migration plan (dry run)
From:      prod
To:        staging
Mode:      schema and data
Overwrite: on - existing target tables are dropped and recreated

TABLE          ROWS     TARGET
orders         842,113  dropped and recreated
users          250,000  dropped and recreated

1,092,113 rows (estimated) in 2 table(s)
```

It answers the three questions the flag list can't:

- **What `--overwrite` would drop.** Aimed at the wrong database, that's visible
  before it runs instead of after.
- **Which columns have incompatible types.** The restore does not alter an
  existing column's type, so those rows fail or coerce silently. Both types are
  shown side by side.
- **Which tables exist on only one side.** `--data-only` into a table that isn't
  in the target is now a stated warning rather than a failure 30 seconds later.

Row counts come from Postgres's own statistics, so the whole report costs two
catalog queries instead of scanning every table — which is the cost a dry run
exists to avoid.

With `--json` the report is a structured object, so a pipeline can assert on it:

```bash
pgtransfer --json migrate database prod staging --dry-run 2>/dev/null \
  | jq -e '.data.report.type_conflicts | length == 0'
```

## 🔧 Fixes

### `migrate rollback` gains `--strict`

`import dump` and `migrate database` both accepted `--strict`; rollback did not.
That is the route where it matters most: a rollback that carries on past a failed
statement leaves the target **neither the old database nor the new one**, and the
tolerant default still reports success, because a non-zero `pg_restore` status
with no recognisable error lines is treated as a warning.

```bash
pgtransfer migrate rollback staging ~/.pgtransfer/backups/rollback_20261017.sql --strict
```

### `--verbose` now shows the restore command

`--verbose` printed the `pg_dump` command but not the `pg_restore` one, so the
command you'd most want when a restore misbehaves was the one you couldn't see.
It prints now, with the password replaced by `***`.

### MCP `migrate_database` accepts `strict`

The MCP tool omitted `strict` entirely, so an agent driving a migration through
the protocol got weaker error semantics than the CLI for the same operation.
It is available alongside `overwrite` and `enable_rollback`.

## 📚 Docs

`README.md` gains sections for `pgtransfer log` and `--dry-run`, a flag table for
`migrate rollback`, and the new `dry_run_failed` error code. Every example in the
new sections was run against a live PostgreSQL 16 instance before publication.

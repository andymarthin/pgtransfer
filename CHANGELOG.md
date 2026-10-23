# Changelog

All notable changes to PGTransfer are documented here, in the style of
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The project follows
[semantic versioning](https://semver.org/spec/v2.0.0.html).

Each release has its own `## [x.y.z] - YYYY-MM-DD` section, newest first, and the
release workflow publishes the section matching a pushed tag as the GitHub release
body. Add a new section at the top for the version you are about to tag, replacing
`unreleased` with the release date.

## [1.4.0] - 2026-10-23

Import and migration both got a different path to the database, and the numbers
moved by an order of magnitude.

### ⚡ Performance

#### `import csv` streams through `COPY`

Each batch is now one `COPY ... FROM STDIN` instead of a statement per row, so the
server parses values with its own bulk input functions. On a 1,000,000 row,
8-column table the same load went from **214 s to 3.1 s** — under a second above
the server's own `COPY FROM`, which is the floor for any client that still has to
receive the bytes.

A file whose header does not match the table's columns still loads, through
batched `INSERT`s, which is slower but can fill a subset of the columns.

#### `migrate database` moves a custom-format archive

The transfer writes a custom-format archive from the source and restores it into
the target. That carries the data through `COPY` in both directions rather than
replaying it as statements, and lands a 1,000,000 row database in **3.6 s**,
ahead of `pg_dump` + `pg_restore` at 4.3 s.

The archive is written at gzip level 1: it is written and read back inside one
command, so a higher level spends most of a second compressing for a smaller file
that never reaches a disk it will be read from again.

### ✨ New features

#### `export dump --compress-level`

The compression level of an archive format is now settable from `1` to `9`. The
default is `1`.

### 🐛 Correctness

#### A carriage return inside a value survives a CSV round trip

`export csv` and `import csv` now write and read CSV with a parser that keeps a
`\r\n` inside a quoted field intact. A text column holding a Windows line ending
comes back with the carriage return it went in with, and the exported bytes are
exactly the imported bytes. A damaged file — an unclosed quote, a quote in the
middle of an unquoted field — is reported with its line number instead of being
guessed at.

Both sides are faster as well: the parser reads in runs and returns an unquoted
field as a slice of its buffer, at about 230 MB/s with no database involved.

#### A missing client tool is reported before the transfer starts

Every command that drives a PostgreSQL client tool checks for it first. A
machine without `pg_dump` now says so instead of failing once the output path has
been created, and a command that needs several tools names all of the missing
ones at once, with the command that needs each:

```console
$ pgtransfer export dump prod backup.dump
Error: pg_dump (needed by export dump and migrate database) not found in PATH — install the PostgreSQL client tools
```

`import dump` and `migrate rollback` read the dump's own format first, so they
ask for `psql` or `pg_restore` — whichever will actually run — instead of always
asking for `pg_restore`.

For a profile with SSH enabled, the check is on the identity rather than a
binary: the tunnel runs in-process, so no `ssh` executable is involved. A
`key_path` that is missing or unreadable, or a profile with no key, password or
agent to authenticate with, is reported by profile name before anything is
transferred.

```console
$ pgtransfer migrate database prod staging
Error: profile "prod": SSH key /home/me/.ssh/prod_key is not readable: stat /home/me/.ssh/prod_key: no such file or directory
```

In `--json` mode these failures carry a code, `missing_tool` or `ssh_identity`,
and nothing has been written or connected to.

#### A plain SQL dump without `pg_dump`'s header is still recognised

Format detection now falls back to reading the file as text with a statement
terminator, so a dump that was filtered, concatenated or written by hand is
restored with `psql` rather than handed to `pg_restore`, which refuses anything
that is not an archive.

### ✅ Tests

- `TestImportCSVRoundTripsThroughCopy` — exports a table of awkward values (tab,
  newline, CRLF, backslash, quote, comma, leading space, unicode, `NULL`) and
  compares the target's rows against the source's after a `COPY` import.
- `TestImportCSVPreservesNulls`, `TestImportCSVRejectsRaggedCopyRows` — an empty
  field is `NULL` and not an empty string; a short row is refused rather than
  padded.
- `TestCSVParserRoundTrips`, `TestCSVParserAcceptsExportShapes`,
  `TestCSVParserReportsDamage`, `TestCSVParserHandlesLargeRecords` — the reader's
  grammar and its behaviour on damaged input.
- `TestCheckReportsEveryMissingTool`, `TestCheckPartialInstall` — every absent
  tool named in one message, each with the command that needs it.
- `TestDumpExportFailsBeforeItWritesAnything`,
  `TestDumpImportChecksTheToolForTheFileFormat`,
  `TestRollbackChecksTheToolForTheFileFormat`,
  `TestDatabaseMigrationNeedsAllThreeTools` — the pre-flight check in each
  command, including that no output file is created and that the format decides
  which tool is asked for.
- `TestCheckSSHIdentity`, `TestCheckProfiles` — a missing or unreadable key, a
  profile with nothing to authenticate with, and both ends of a migration being
  checked with the failing one named.
- `TestIsPlainTextDump` — the format markers, four archive shapes, and a SQL file
  with no `pg_dump` header.
- `TestAppendCompressionArgs`, `TestCompressionLevelIsCheapByDefault`,
  `TestMigrationTempFileIsAnArchive` — the `--compress` arguments and the staging
  file a migration uses.

### 🔧 Under the hood

- `scripts/bench.sh` measures export, import and migration next to the
  `pg_dump`/`pg_restore` or `COPY` floor for each, and prints a markdown table.
- The parser's read window is deliberately small. Both parse paths scan from the
  parse position to the end of the window, so buffering far ahead of the record
  being read makes every field scan bytes that are not its own: an import took
  2.9 s with a 256-byte window and 21 s with a 64 KB one. A buffered reader under
  the parser keeps the syscall count low while the window stays short.

## [1.3.0] - 2026-10-17

Two capabilities the tool was missing: it recorded every migration but could not
show you what it recorded, and it could migrate a database but not tell you what
that would do until it had already done it.

### ✨ New features

#### `pgtransfer log`

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

#### `migrate database --dry-run`

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

### 🔧 Fixes

#### `migrate rollback` gains `--strict`

`import dump` and `migrate database` both accepted `--strict`; rollback did not.
That is the route where it matters most: a rollback that carries on past a failed
statement leaves the target **neither the old database nor the new one**, and the
tolerant default still reports success, because a non-zero `pg_restore` status
with no recognisable error lines is treated as a warning.

```bash
pgtransfer migrate rollback staging ~/.pgtransfer/backups/rollback_20261017.sql --strict
```

#### `--verbose` now shows the restore command

`--verbose` printed the `pg_dump` command but not the `pg_restore` one, so the
command you'd most want when a restore misbehaves was the one you couldn't see.
It prints now, with the password replaced by `***`.

#### MCP `migrate_database` accepts `strict`

The MCP tool omitted `strict` entirely, so an agent driving a migration through
the protocol got weaker error semantics than the CLI for the same operation.
It is available alongside `overwrite` and `enable_rollback`.

### 📚 Docs

`README.md` gains sections for `pgtransfer log` and `--dry-run`, a flag table for
`migrate rollback`, and the new `dry_run_failed` error code. Every example in the
new sections was run against a live PostgreSQL 16 instance before publication.

## [1.2.1] - 2026-10-17

A correctness release. Four audits of the data paths turned up ways the tool could
lose rows, overwrite a file it said it would not touch, or report a success it had
not achieved. All of them are fixed, and the README documents the behaviour that
resulted.

Upgrade if you use CSV export or import, restores, SSH tunnels, or passwords that
contain spaces.

### 🔧 Fixes

#### Export no longer loses rows

Pagination used `LIMIT`/`OFFSET`, which PostgreSQL does not promise to keep stable
between queries — rows could be duplicated and, worse, silently skipped. Export now
pages on the table's single-column unique key with `ORDER BY`, so every batch is a
disjoint window. Measured on 10,000,000 rows:

| | Before | After |
| --- | --- | --- |
| 1M rows | 24.88s, 367,791 rows lost | 1.64s, complete |
| 5M rows | 14m05s, 1,839,548 rows lost | 9.79s, complete |
| 10M rows | — | 16.97s, complete |

Both export paths also failed to check `rows.Err()`, so a dropped connection or a
cancelled statement ended the loop exactly as a finished table does. A truncated
file was reported as a successful export.

#### A failed restore no longer reports success

`pg_restore` exits with status 1 both for benign client/server version skew and for
a table that failed to load. Every exit status of 1 used to be treated as "restored
with warnings", so a restore that silently dropped a table printed a warning and
returned success. Restore output is classified line by line instead: known
version-skew messages stay warnings and are counted, and anything else fails the
command and names the offending line. `--strict` also passes `--exit-on-error`.

#### Table and column names are quoted

Names arrive from the command line, from profiles and from MCP tool arguments, and
were interpolated into SQL unquoted. `lib/pq` sends zero-argument queries over the
simple query protocol, which accepts several statements separated by semicolons, so
a crafted table name could append its own statements. Every identifier is quoted
now, and names that need quoting — mixed case, spaces, reserved words, dots — work
as a side effect.

#### `--headers` defaults to on

`export csv` and `import csv` both defaulted to off, so the obvious round trip wrote
a header row that the importer then inserted as data. Both default to on;
`--headers=false` imports or exports a header-less file. **If you rely on the old
default, pass `--headers=false` explicitly.**

#### The overwrite guard checks the file it will write

Export appends `.csv` when the path has no extension, but the guard checked the path
as typed. Exporting to `users` looked for a file called `users`, found none, and
overwrote `users.csv`.

#### Credentials stay out of verbose output

`--verbose` printed the whole `pg_dump` argument list, and the connection URL is one
of the arguments. Passwords are replaced with `***` before printing.

#### Passwords containing spaces work

Connection strings were built with `url.QueryEscape`, which encodes a space as `+`.
`lib/pq` reads the userinfo block with path semantics, where `+` is a literal plus,
so the password silently became a different string. `url.UserPassword` is used now,
and an IPv6 host is bracketed automatically.

#### SSH tunnels are opened once, and verified

Dumping or restoring through a bastion dialled SSH twice — once to check the
connection, once to build the tunnel — and the tunnel was rebound on a guessed free
port after a fixed sleep. If another process took the port in between, the command
was handed a port that accepted nothing and failed several steps later with an
error that did not mention the tunnel. The tunnel now binds before it returns,
listens on `127.0.0.1` only, and closes its listener, client and goroutine together.

#### Other fixes

- A dump is flushed to disk before success is reported, and a failed dump is
  removed instead of being left behind looking like a backup.
- `migrate database --strict` now affects the restore. It was accepted and ignored.
- Import counted rows with `ReadAll`, needing several gigabytes of memory to draw a
  progress bar on a large file. It streams now.
- Import of a header-less file turned `NULL` into an empty string; the batched path
  did not. Both match export now.
- Every progress bar leaked a goroutine. One-shot CLI runs never noticed; a process
  hosting the MCP server did.
- Interactive `profile add` saved an empty password when the terminal read failed,
  and stored `0` for an unparseable port. Both are errors now.

### 📚 Docs

The README is reordered around the tasks you actually perform — export, import,
dump, restore, migrate — with a per-command flag table and an index, rather than
around the source layout. Measured numbers live in
[BENCHMARKS.md](https://github.com/andymarthin/pgtransfer/blob/main/BENCHMARKS.md).

## [1.2.0] - 2026-10-17

An automation- and distribution-focused release. This version makes pgtransfer
scriptable and AI-drivable with machine-readable `--json` output and a Model
Context Protocol server, and makes it far easier to install with prebuilt
binaries, a one-line install script, and a Homebrew tap.

### ✨ New features

#### Machine-readable `--json` output

Every command accepts a global `--json` flag (or `PGTRANSFER_JSON=1`) and emits
a single, stable JSON object on stdout instead of human-friendly text:

```json
{
  "ok": true,
  "command": "export csv",
  "data": { "rows": 250, "output_file": "out.csv" },
  "error": null
}
```

On failure `ok` is `false` and `error` carries a stable code and message:

```json
{
  "ok": false,
  "command": "test-connection",
  "error": { "code": "profile_not_found", "message": "profile \"ghost\" not found" }
}
```

- Wired into `version`, `test-connection`, `profile list|active|use|delete`,
  `export csv|dump`, `import csv|dump` and `migrate database|rollback`.
- Exactly **one** JSON object is written per invocation, including parser-level
  failures such as an unknown subcommand or an invalid flag
  (`code: "usage_error"`).
- The process still exits non-zero on failure.
- Stable error codes: `usage_error`, `profile_not_found`, `no_active_profile`,
  `config_load_failed`, `connection_failed`, `export_failed`, `import_failed`,
  `dump_failed`, `restore_failed`, `migration_failed`, `rollback_failed`,
  `file_exists`, `file_not_found`, `truncate_failed`, `invalid_input`.
- **Passwords are never emitted.** Profiles report `has_password` instead.

#### MCP server for AI agents

`pgtransfer mcp` runs a
[Model Context Protocol](https://modelcontextprotocol.io) server over stdio, so
AI hosts such as Claude Desktop, Cursor and custom agent runtimes can drive
pgtransfer as typed tools.

Exposed tools: `list_profiles`, `test_connection`, `export_csv`, `import_csv`,
`export_dump`, `import_dump`, `migrate_database`.

```json
{
  "mcpServers": {
    "pgtransfer": { "command": "pgtransfer", "args": ["mcp"] }
  }
}
```

Tool failures are reported in-band (`isError: true` plus a structured payload)
rather than as protocol errors, so a model can read the problem and correct
itself. Because stdout carries the protocol, incidental library output is
redirected to stderr.

#### Install script

A POSIX `sh` installer downloads the binary for your platform, verifies it
against the release's `SHA256SUMS` and installs it:

```bash
curl -fsSL https://raw.githubusercontent.com/andymarthin/pgtransfer/main/scripts/install.sh | sh
```

It installs to `/usr/local/bin` when writable, otherwise `~/.local/bin`, and is
configurable via `PGTRANSFER_VERSION`, `PGTRANSFER_INSTALL` and
`PGTRANSFER_NO_SUDO`. **It refuses to install an archive whose checksum does not
match.**

#### Homebrew

```bash
brew tap andymarthin/pgtransfer
brew install pgtransfer
```

The formula lives in the
[homebrew-pgtransfer](https://github.com/andymarthin/homebrew-pgtransfer) tap and
updates itself from each release.

### 🔧 Tooling & CI

- **Releases are now published immediately** instead of being created as drafts,
  so artifacts are downloadable as soon as the workflow finishes.
- The release workflow notifies the Homebrew tap, which rewrites its formula
  against the new `SHA256SUMS`.
- **New performance-friendly CI triggers:** CI runs on pushes to `main` and on
  all pull requests. Topic branches are no longer built twice (previously a push
  to a branch with an open PR triggered two identical runs).
- CI and Release share the same reusable pipeline (`reusable-ci.yml`), now with
  an **Install script** job that verifies POSIX syntax, the checksum gate, and a
  real install.
- Runners pinned to `ubuntu-24.04`; GitHub Actions bumped to Node 24 runtimes.

### ⬆️ Requirements

- **Go 1.25 or higher** to build from source (the MCP SDK requires it). Prebuilt
  binaries are unaffected.
- `pg_dump`, `pg_restore` and `psql` on your `PATH` for dump/restore.
- An OpenSSH client for SSH tunnels.

### 📄 License

The project is now shipped with its `LICENSE` file (MIT), which the README had
always claimed but the repository was missing.

## [1.1.0] - 2026-10-16

A robustness, correctness and tooling release. This version hardens backup and
restore against PostgreSQL client/server version skew, makes connections far
more reliable, adds a proper automated test suite with CI, and documents the
full local testing stack.

### ⚠️ Behaviour changes

- **Restores are now tolerant by default.** `pgtransfer import dump` no longer
  fails when `pg_restore` reports benign errors such as *"unrecognized
  configuration parameter"* that occur when the client tools are newer than the
  server (a very common real-world case). These are reported as warnings and
  the restore completes. Pass the new **`--strict`** flag to fail on the first
  error instead.
- **`import csv` without `--headers` now imports every row as data.** Previously
  the first line could be silently skipped; now the whole file is treated as
  data unless `--headers` is given. If your CSV has a header row, pass
  `--headers`.

### ✨ New features

- **`pgtransfer import dump`** gains four flags:
  - `--clean` — drop existing objects before restoring (`pg_restore` only).
  - `--verbose` — verbose output from the restore tools.
  - `--timeout <seconds>` — abort the restore after the given time (`0` = none).
  - `--strict` — fail on the first restore error instead of tolerating
    version-skew noise.
- **New `--version` / `-v` support** on the root command, sharing the output of
  the `version` subcommand.

### 🐛 Fixes & robustness

- **Connection handling reworked** (`internal/db`): direct connections now
  configure the connection pool consistently, surface ping failures correctly,
  and no longer leak an open pool when a connection attempt fails.
- **SSH support consolidated** into a new `internal/sshx` package with a single
  `BuildAuthMethods` path covering key files, passphrases, `ssh-agent` and
  passwords. Key paths now expand `$ENV` variables. The old
  `internal/utils.ReadPrivateKey` is kept as a thin, deprecated shim.
- **Dump/restore rewritten** to reliably auto-detect plain-text vs. custom-format
  archives, normalise output filenames per format (`--format` → `.sql`, `.dump`,
  `.tar`, directory), and report clear timing information.
- **CSV import/export hardened**: correct header handling, correct value
  formatting for timestamps, numerics, booleans and byte arrays, and a progress
  bar that degrades gracefully when `COUNT(*)` is unavailable.
- **Version metadata** (`Version`, `Commit`, `Date`) is now injected at build
  time and printed consistently.

### 🧪 Testing & CI

- **New GitHub Actions CI** (`.github/workflows/ci.yml`) runs on every push and
  pull request:
  - **Build, vet & unit tests** — `gofmt`, `go vet`, `go build`, and
    `go test -race` with a coverage artifact.
  - **Integration tests** — provisions PostgreSQL and the repository's SSH
    image, installs PostgreSQL 17 client tools (to exercise the version-skew
    path), and runs the build-tagged integration suite.
- **Unit tests added** across `internal/config`, `internal/io`, `internal/sshx`,
  `internal/utils`, `internal/log`, and the `cmd/*` packages. Unit coverage rose
  from ~11% to ~25%, with `internal/log` at 96%, `internal/utils` at 94%,
  `internal/sshx` at 83% and `internal/config` at 79%.
- **Integration tests added** (build-tagged, `-tags=integration`) covering CSV
  export/import round-trips, plain and custom dump/restore with format
  auto-detection, connection-aware dumps, and the SSH tunnel path. They skip
  cleanly when no database is available, and `PGTRANSFER_TEST_REQUIRE=1`
  turns missing prerequisites into failures in CI.
- **Local test stack refreshed** (`scripts/`): the SSH test server is now a
  small Alpine image with TCP forwarding enabled, and the SSH user/password are
  overridable via build args. See `scripts/README.md`.

### 📚 Docs

- README updated with a CI badge, a full **Test Suite** section (unit vs.
  integration, environment-variable reference, how to run everything), and the
  new `import dump` flags.
- `scripts/README.md` documents the SSH image and integration-test workflow.

## [1.0.1] - 2025-10-29

Initial release of PGTransfer — a PostgreSQL data transfer tool with SSH tunnel
support.

## [1.0.0] - 2025-10-29

Initial release of PGTransfer — a PostgreSQL data transfer tool with SSH tunnel
support.

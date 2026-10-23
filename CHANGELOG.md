# Changelog

All notable changes to PGTransfer are documented here, in the style of
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The project follows
[semantic versioning](https://semver.org/spec/v2.0.0.html).

Each release has its own `## [x.y.z] - YYYY-MM-DD` section, newest first, and the
release workflow publishes the section matching a pushed tag as the GitHub release
body. Add a new section at the top for the version you are about to tag, replacing
`unreleased` with the release date.

## [1.4.0] - unreleased

Import and migration both got a different path to the database, and the numbers
moved by an order of magnitude.

## ⚡ Performance

### `import csv` streams through `COPY`

Each batch is now one `COPY ... FROM STDIN` instead of a statement per row, so the
server parses values with its own bulk input functions. On a 1,000,000 row,
8-column table the same load went from **214 s to 3.1 s** — under a second above
the server's own `COPY FROM`, which is the floor for any client that still has to
receive the bytes.

A file whose header does not match the table's columns still loads, through
batched `INSERT`s, which is slower but can fill a subset of the columns.

### `migrate database` moves a custom-format archive

The transfer writes a custom-format archive from the source and restores it into
the target. That carries the data through `COPY` in both directions rather than
replaying it as statements, and lands a 1,000,000 row database in **3.6 s**,
ahead of `pg_dump` + `pg_restore` at 4.3 s.

The archive is written at gzip level 1: it is written and read back inside one
command, so a higher level spends most of a second compressing for a smaller file
that never reaches a disk it will be read from again.

## ✨ New features

### `export dump --compress-level`

The compression level of an archive format is now settable from `1` to `9`. The
default is `1`.

## 🐛 Correctness

### A carriage return inside a value survives a CSV round trip

`export csv` and `import csv` now write and read CSV with a parser that keeps a
`\r\n` inside a quoted field intact. A text column holding a Windows line ending
comes back with the carriage return it went in with, and the exported bytes are
exactly the imported bytes. A damaged file — an unclosed quote, a quote in the
middle of an unquoted field — is reported with its line number instead of being
guessed at.

Both sides are faster as well: the parser reads in runs and returns an unquoted
field as a slice of its buffer, at about 230 MB/s with no database involved.

### A missing client tool is reported before the transfer starts

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

### A plain SQL dump without `pg_dump`'s header is still recognised

Format detection now falls back to reading the file as text with a statement
terminator, so a dump that was filtered, concatenated or written by hand is
restored with `psql` rather than handed to `pg_restore`, which refuses anything
that is not an archive.

## ✅ Tests

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

## 🔧 Under the hood

- `scripts/bench.sh` measures export, import and migration next to the
  `pg_dump`/`pg_restore` or `COPY` floor for each, and prints a markdown table.
- The parser's read window is deliberately small. Both parse paths scan from the
  parse position to the end of the window, so buffering far ahead of the record
  being read makes every field scan bytes that are not its own: an import took
  2.9 s with a 256-byte window and 21 s with a 64 KB one. A buffered reader under
  the parser keeps the syscall count low while the window stays short.

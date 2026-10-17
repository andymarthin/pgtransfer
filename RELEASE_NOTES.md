# PGTransfer v1.4.0

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

# PGTransfer v1.2.1

A correctness release. Four audits of the data paths turned up ways the tool could
lose rows, overwrite a file it said it would not touch, or report a success it had
not achieved. All of them are fixed, and the README documents the behaviour that
resulted.

Upgrade if you use CSV export or import, restores, SSH tunnels, or passwords that
contain spaces.

## 🔧 Fixes

### Export no longer loses rows

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

### A failed restore no longer reports success

`pg_restore` exits with status 1 both for benign client/server version skew and for
a table that failed to load. Every exit status of 1 used to be treated as "restored
with warnings", so a restore that silently dropped a table printed a warning and
returned success. Restore output is classified line by line instead: known
version-skew messages stay warnings and are counted, and anything else fails the
command and names the offending line. `--strict` also passes `--exit-on-error`.

### Table and column names are quoted

Names arrive from the command line, from profiles and from MCP tool arguments, and
were interpolated into SQL unquoted. `lib/pq` sends zero-argument queries over the
simple query protocol, which accepts several statements separated by semicolons, so
a crafted table name could append its own statements. Every identifier is quoted
now, and names that need quoting — mixed case, spaces, reserved words, dots — work
as a side effect.

### `--headers` defaults to on

`export csv` and `import csv` both defaulted to off, so the obvious round trip wrote
a header row that the importer then inserted as data. Both default to on;
`--headers=false` imports or exports a header-less file. **If you rely on the old
default, pass `--headers=false` explicitly.**

### The overwrite guard checks the file it will write

Export appends `.csv` when the path has no extension, but the guard checked the path
as typed. Exporting to `users` looked for a file called `users`, found none, and
overwrote `users.csv`.

### Credentials stay out of verbose output

`--verbose` printed the whole `pg_dump` argument list, and the connection URL is one
of the arguments. Passwords are replaced with `***` before printing.

### Passwords containing spaces work

Connection strings were built with `url.QueryEscape`, which encodes a space as `+`.
`lib/pq` reads the userinfo block with path semantics, where `+` is a literal plus,
so the password silently became a different string. `url.UserPassword` is used now,
and an IPv6 host is bracketed automatically.

### SSH tunnels are opened once, and verified

Dumping or restoring through a bastion dialled SSH twice — once to check the
connection, once to build the tunnel — and the tunnel was rebound on a guessed free
port after a fixed sleep. If another process took the port in between, the command
was handed a port that accepted nothing and failed several steps later with an
error that did not mention the tunnel. The tunnel now binds before it returns,
listens on `127.0.0.1` only, and closes its listener, client and goroutine together.

### Other fixes

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

## 📚 Docs

The README is reordered around the tasks you actually perform — export, import,
dump, restore, migrate — with a per-command flag table and an index, rather than
around the source layout. Measured numbers live in
[BENCHMARKS.md](https://github.com/andymarthin/pgtransfer/blob/main/BENCHMARKS.md).

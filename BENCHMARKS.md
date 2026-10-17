# Benchmarks

Measured performance for the operations where throughput and latency matter:
CSV export, CSV import, database migration, value formatting and command
startup.

Numbers come from real runs against a live PostgreSQL instance and from the Go
benchmarks in `internal/*/`. They describe this project's performance; they are
not a comparison with other tools.

## Environment

| | |
|---|---|
| Machine | Apple M1 Pro, macOS |
| PostgreSQL | 15.14 (also verified on 16.15, 17.11 and 18.6) |
| Dataset | 8 columns: `bigserial` key, `text`, `text`, `numeric(12,2)`, `integer`, `boolean`, `timestamptz`, nullable `text` |
| Rows per table | 1M, 5M and 10M |

Reproduce with `scripts/bench.sh` against the test stack, or run the Go
benchmarks with `make bench`. Reseed the table with:

```sql
CREATE TABLE bench_users (
  id bigserial PRIMARY KEY,
  email text NOT NULL,
  full_name text,
  balance numeric(12,2),
  age integer,
  active boolean,
  created_at timestamptz,
  notes text
);
INSERT INTO bench_users (email, full_name, balance, age, active, created_at, notes)
SELECT 'user' || g || '@example.com',
       'User Number ' || g,
       (random()*10000)::numeric(12,2),
       (random()*80+18)::int,
       g % 2 = 0,
       now() - (g || ' seconds')::interval,
       CASE WHEN g % 7 = 0 THEN NULL ELSE 'note for ' || g END
FROM generate_series(1, 1000000) g;
ANALYZE bench_users;
```

## Summary (1,000,000 rows)

| Operation | Time | Throughput | Floor |
|---|---|---|---|
| CSV export | 1.4 s | ~710,000 rows/s | 1.1 s (`COPY TO`) |
| CSV import | 3.1 s | ~320,000 rows/s | 2.2 s (`COPY FROM`) |
| `migrate database` | 3.6 s | ~280,000 rows/s | 4.3 s (`pg_dump` + `pg_restore`) |

The floor is the same work done by PostgreSQL's own client tools against the
same server and disk. Export and import sit close to it; a migration beats it,
because the intermediate archive is written at a compression level that keeps
the round trip cheap.

Peak memory during export and import stays around 20-25 MB regardless of table
size: rows are streamed in batches of 20,000, not collected.

## CSV export

Default settings (batch size 20,000):

| Rows | Time | Throughput | Output size | Floor (`COPY TO`) |
|------|------|------------|-------------|-------------------|
| 1,000,000 | 1.4 s | ~710,000 rows/s | ~100 MB | 1.1 s |
| 5,000,000 | 5.8 s | ~860,000 rows/s | ~490 MB | 4.8 s |

### Effect of batch size (5,000,000 rows)

| Batch size | Time | Throughput |
|------------|------|------------|
| 500 | 14.1 s | ~355,000 rows/s |
| 5,000 | 7.5 s | ~670,000 rows/s |
| 20,000 (default) | 5.9 s | ~850,000 rows/s |
| 50,000 | 5.6 s | ~895,000 rows/s |

Batch size is the main tuning knob up to about 20,000. Beyond that the gain is
small and memory per batch grows with it, so the default stays at 20,000.

### Reference: server-side COPY

`COPY ... TO STDOUT` on the same table is the practical floor for a client that
still has to receive, format and write the data.

## CSV import

Import sends each batch as one `COPY ... FROM STDIN`, so the server parses the
values with its own bulk input functions instead of executing a statement per
row. A batch whose columns do not match the file's header falls back to
individual `INSERT` statements, which is much slower but loads a subset of the
columns.

| Rows | Time | Throughput | Floor (`COPY FROM`) |
|---|---|---|---|
| 1,000,000 | 3.1 s | ~320,000 rows/s | 2.2 s |
| 5,000,000 | 16 s | ~310,000 rows/s | 10.3 s |

Import costs about 0.7 s above the server's own COPY on these tables. The
remainder is the CSV parse on the client, which has to read every field before
the value can be sent.

## Database migration

`migrate database` takes a custom-format archive from the source and restores it
into the target. Custom format is what `pg_restore` streams with COPY; a plain
SQL dump is replayed statement by statement instead.

| Rows | Time |
|---|---|
| 1,000,000 | 3.6 s |

### Compression level

The archive is written at gzip level 1. Measured on the 1,000,000 row table:

| Level | Dump size | Dump time | Restore time |
|---|---|---|---|
| 0 | 112 MB | 0.87 s | 2.33 s |
| 1 | 21 MB | 0.82 s | 2.20 s |
| 3 | 20 MB | 1.05 s | 2.12 s |
| 6 | 19 MB | 1.54 s | 2.37 s |

Levels above 1 spend most of a second compressing two megabytes that never
matter, so the default is 1. `export dump --compress-level` sets it explicitly.

## CSV parsing and writing

Measured in-process with no database, on rows of five fields totalling about
90 bytes:

| Operation | Throughput |
|---|---|
| Parse (import side) | ~230 MB/s |
| Write (export side) | ~680 MB/s |

The parser reads in short runs and returns an unquoted field as a slice of the
read buffer, which costs one allocation per field. Its window size is a trade
off: every scan runs from the parse position to the end of the buffer, so a
larger window is scanned more often. On this dataset an import took 2.9 s with a
256-byte window, 4.3 s at 4 KB, 7.8 s at 16 KB and 21 s at 64 KB.

## Value formatting

Cost per value in `FormatCSVValue`, no database involved:

| Type | Time | Allocations |
|------|------|-------------|
| `nil` | 2.1 ns | 0 |
| `string` | 2.1 ns | 0 |
| `bool` | 2.1 ns | 0 |
| `int` | 3.0 ns | 0 |
| `[]byte` (text) | 16.7 ns | 1 |
| `[]byte` (numeric) | 15.1 ns | 1 |
| `int64` | 23.3 ns | 1 |
| `date` | 34.4 ns | 1 |
| `timestamp` | 43.5 ns | 1 |
| `float64` | 61.9 ns | 1 |

A representative 8-column row costs 227 ns (~35 MB/s of formatted output).

## Command startup overhead

| Operation | Time |
|-----------|------|
| Load config, 1 profile | 32 µs |
| Load config, 10 profiles | 117 µs |
| Load config, 50 profiles | 440 µs |
| Build a DSN | 318 ns |

## JSON output

| Operation | Time | Allocations |
|-----------|------|-------------|
| Successful result envelope | 63 ns | 1 |
| Error envelope | 126 ns | 2 |
| Mode check | 14 ns | 0 |

JSON output adds no measurable cost per command.

## MCP server

| Operation | Time |
|-----------|------|
| Server construction (7 tools) | 165 µs |
| Decode tool arguments | 907 ns |
| Build a result | 1.0 µs |

MCP tool call overhead is dominated by the database work, not the protocol
handling.

## PostgreSQL version compatibility

Export and import were verified against PostgreSQL 16.15, 17.11 and 18.6
(alpine images), covering:

- a table with a `bigserial` primary key (key-based pagination)
- a table with no primary key (falls back to offset pagination)
- a table whose unique key is not the first column
- header-less and headered CSV round-trips, with `NULL` preserved

All combinations produced identical data after a round-trip.

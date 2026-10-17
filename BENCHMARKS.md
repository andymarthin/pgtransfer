# Benchmarks

Measured performance for the operations where throughput and latency matter:
CSV export, CSV import, value formatting, and command startup.

Numbers come from real runs against a live PostgreSQL instance and from the
Go benchmarks in `internal/*/`. They describe this project's performance; they
are not a comparison with other tools.

## Environment

| | |
|---|---|
| Machine | Apple M1 Pro, macOS |
| PostgreSQL | 15.14 (also verified on 16.15, 17.11 and 18.6) |
| Dataset | 8 columns: `bigserial` key, `text`, `text`, `numeric(12,2)`, `integer`, `boolean`, `timestamptz`, nullable `text` |
| Rows per table | 1M, 5M and 10M |

Reproduce the Go benchmarks with `make bench`, or record your own baseline with
`make bench-save` and compare later with `make bench-compare`.

## CSV export

Default settings (batch size 20,000):

| Rows | Time | Throughput | Output size |
|------|------|------------|-------------|
| 1,000,000 | 3.6 s | ~277,000 rows/s | 91 MB |
| 5,000,000 | 9.8 s | ~510,000 rows/s | 456 MB |
| 10,000,000 | 17.0 s | ~588,000 rows/s | 912 MB |

Peak memory stays around 20-25 MB regardless of table size.

### Effect of batch size (5,000,000 rows)

| Batch size | Time | Throughput |
|------------|------|------------|
| 500 | 15.3 s | ~327,000 rows/s |
| 5,000 | 5.9 s | ~847,000 rows/s |
| 20,000 (default) | 9.8 s | ~510,000 rows/s |
| 50,000 | 5.5 s | ~909,000 rows/s |

Batch size is the main tuning knob. Very large batches add memory pressure
without further gain on this hardware.

### Reference: server-side COPY

`COPY ... TO STDOUT` on the same 5,000,000 row table completes in 5.27 s
(~949,000 rows/s). That is the practical floor for a client that must still
receive, format and write the data.

## CSV import

Import of a 200,000 row file into an empty table with a matching schema, at the
default batch size, on PostgreSQL 16:

| Operation | Time |
|-----------|------|
| Import 200,000 rows | ~4 s |

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

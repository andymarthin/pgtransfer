# Benchmark results: 1,000,000 rows

Environment: macOS, Apple M1 Pro, PostgreSQL 15.14 (Docker, alpine),
`testdb` on `localhost:5433`. Hardware and database are local, so absolute
numbers are optimistic — the *ratios* are the meaningful part.

Dataset (`bench_1m`): 1,000,000 rows, 133 MB on disk, 91 MB as CSV.

| Column | Type | Notes |
|---|---|---|
| `id` | `bigserial PRIMARY KEY` | dense, 1..1000000 |
| `customer` | `text` | ~16 chars |
| `email` | `text` | ~22 chars |
| `amount` | `numeric(12,2)` | arrives as `[]byte` |
| `quantity` | `integer` | |
| `active` | `boolean` | |
| `created_at` | `timestamptz` | formatted per row |
| `notes` | `text` | NULL on a third of rows |

## CSV export

| Run | Time | Throughput |
|---|---|---|
| Default (`--batch-size 500`) | **24.88 s** | ~40,200 rows/s |
| `--batch-size 5000` | **3.76 s** | ~266,000 rows/s |
| `--batch-size 50000` | **1.62 s** | ~617,000 rows/s |
| First run, 1M rows (cold) | 33.77 s | — |
| `psql \copy` (native COPY, reference) | **1.05 s** | ~950,000 rows/s |

Peak RSS for the default export: 21.9 MB for 91 MB of output, confirming the
batching keeps memory flat regardless of table size.

**Batch size is the dominant factor: 500 → 50,000 is a 15x speedup.**

## Why the default is slow: `LIMIT/OFFSET` is quadratic

`ExportCSVWithOptions` builds each batch as:

```sql
SELECT * FROM t LIMIT <batch> OFFSET <n>
```

PostgreSQL has no way to skip to row *n* without reading the rows before it, so
the cost of a batch grows with its offset. Measured on `bench_1m`:

| Query | Time |
|---|---|
| `LIMIT 500 OFFSET 0` | 6.1 ms |
| `LIMIT 500 OFFSET 500000` | 81.2 ms |
| `LIMIT 500 OFFSET 999500` | 101.3 ms |
| `LIMIT 50000 OFFSET 0` | 3.5 ms |
| `LIMIT 50000 OFFSET 950000` | 39.8 ms |
| Full scan (`SELECT count(*)`) | 22.8 ms |

At batch size 500 the export issues 2,000 queries, each rescanning up to 1M
rows: roughly 100 seconds of scanning in total if it were not for caching and
pipelining. This is the entire gap between 24.88 s and 1.62 s.

## Correctness bug found: exports can lose and duplicate rows

This is more serious than the speed problem.

### What happened

Three exports of the *same* unchanged table produced three different files:

| Batch size | Rows written | Distinct `id` values | Rows lost |
|---|---|---|---|
| 500 | 1,000,000 | 632,209 | **367,791** |
| 5,000 | 1,000,000 | 653,511 | **346,489** |
| 50,000 | 1,000,000 | 614,240 | **385,760** |

The row count is always right, the *contents* are not: 263,760 `id` values were
written twice, and the rest were never written at all. Nothing warns the user,
and the exit status is 0.

### Root cause

Batches are sliced with `LIMIT/OFFSET` and **no `ORDER BY`**. PostgreSQL makes
no ordering guarantee for such a query, and with `synchronize_seqscans = on`
(the default) a sequential scan may start part-way through the heap and wrap
around, so that a backend shares I/O with others.

The wrap point moves. On the same table, `SELECT id FROM bench_1m LIMIT 3` gave:

```
138952, 138953, 138954     <- synchronize_seqscans = on
1, 2, 3                    <- synchronize_seqscans = off
```

Because every batch is a *separate query*, each one can be sliced from a
differently-rotated ordering. Consecutive batches then overlap in one place and
leave a hole in another:

```
batch 0      (offset 0)        ids 140072 .. 140571
batch 1      (offset 500)      ids 140572 .. 141071
batch 2      (offset 1000)     ids 141072 .. 141571
last batch   (offset 999500)   ids 140692 .. 141191   <- overlaps batches 0 and 1
```

Rows immediately before the wrap point (around id 140071) are never returned by
any batch, and the rows around 140692-141191 are returned by two different
batches. That is exactly the pattern in the corrupted files.

The physical order is fine (`ctid (0,1)` holds `id 1`) and a full scan returns
all 1,000,000 distinct rows; it is only the *slicing* that is unstable.

### The fix

Batches must be ordered deterministically, so that every batch is a true,
disjoint window of the same sequence:

```sql
SELECT * FROM t ORDER BY <stable key> LIMIT <batch> OFFSET <n>
```

Repeating the batched read with `ORDER BY id` produced 200,000 rows / 200,000
distinct / 0 missing on the smaller table, versus the lossy result without it.

Two further changes are worth making at the same time:

1. **Keyset pagination instead of `OFFSET`.** Order by the primary key and
   continue with `WHERE key > <last key> ORDER BY key LIMIT <batch>`. This is
   both correct and linear: no row is ever rescanned, so it removes the
   quadratic cost as well as the correctness bug.
2. **Server-side streaming.** `COPY ... TO STDOUT` (which is what makes
   `psql \copy` 24x faster) or a single cursor-backed query would avoid batch
   round trips entirely.

## Other measurements

### Value formatting (`FormatCSVValue`)

Per value, no database involved:

| Type | Time | Allocations |
|---|---|---|
| `nil` | 2.1 ns | 0 |
| `string` | 2.1 ns | 0 |
| `bool` | 2.1 ns | 0 |
| `[]byte` (text) | 16.8 ns | 1 |
| `[]byte` (numeric) | 15.2 ns | 1 |
| `int64` | **53.0 ns** | 1 |
| `int` | 43.0 ns | 1 |
| `float64` | 89.0 ns | 1 |
| `date` | 70.3 ns | 1 |
| `timestamp` | **104.9 ns** | 1 |

An 8-column row costs ~342 ns and 4 allocations. Numbers and timestamps go
through `fmt.Sprintf` / `time.Format`, which is 25-50x the cost of passing a
string through. With 8 columns x 1M rows that is roughly 4M allocations per
export, which is why the full pipeline reports ~40,000 allocations per run in
the micro-benchmark.

### JSON output envelope

| Operation | Time | Allocations |
|---|---|---|
| `Success` (encode 4-field result) | 63 ns | 1 |
| `Fail` (error envelope) | 126 ns | 2 |
| `IsJSON` | 14 ns | 0 |
| `PrintErrorLine` (text mode) | 71 ns | 0 |

Negligible: JSON mode costs nothing measurable per command.

### Configuration loading

| Operation | Time |
|---|---|
| `LoadConfig`, 1 profile | 32.3 µs |
| `LoadConfig`, 10 profiles | 117.2 µs |
| `LoadConfig`, 50 profiles | 440.4 µs |
| `ProfileExists` (re-reads the file) | 28.1 µs |
| `BuildDSN` (plain) | 318 ns |
| `BuildDSN` (needs escaping) | 534 ns |
| `BuildDSN` (full URL configured) | 5.7 ns |

`LoadConfig` is YAML parsing and is super-linear in profile count. It runs once
per command, so it is ~30 µs of startup latency — acceptable, but it is the
largest non-database fixed cost.

### MCP server helpers

| Operation | Time |
|---|---|
| `New` (server + 7 tool registrations) | 165 µs |
| `decodeInput` (tool arguments) | 907 ns |
| `okResult` | 979 ns |
| `failResult` | 1.35 µs |
| `splitCSV` (8 columns) | 158 ns |
| `qualifyTable` | 3-15 ns |

All per-call costs are sub-microsecond; the server's cost is dominated by
database work, not protocol handling.

## Summary of what was fixed, in order

1. **Correctness (urgent, done):** batch export now paginates on the primary key
   with `ORDER BY` instead of `OFFSET`, so every batch is a true, disjoint
   window. Verified by comparing row count *and* distinct key count against the
   source at 1M, 5M and 10M rows.
2. **Performance (done):** keyset pagination replaced `OFFSET`, which was
   quadratic, and the default batch size was raised. The default export of
   5,000,000 rows went from 14 minutes to under 10 seconds.
3. **Formatting (done):** `int64`, other integers, floats and timestamps now
   avoid `fmt.Sprintf` and `time.Format`, removing the per-cell allocations.

## Final results after the fixes

Measured on the same machine, same tables, with the shipped defaults
(batch size 20000, keyset pagination):

| Rows | Before | After | Duplicates | Missing |
|---|---|---|---|---|
| 1,000,000 | 24.88 s *(367,791 rows lost)* | **3.56 s** | 0 | 0 |
| 5,000,000 | 14 m 5 s *(1,839,548 rows lost)* | **9.79 s** | 0 | 0 |
| 10,000,000 | — | **16.97 s** | 0 | 0 |

For reference, a server-side `COPY TO STDOUT` of the 5M table takes 5.27 s, so
the client is within about 1.9x of the theoretical floor.

Formatting after the change:

| Type | Before | After |
|---|---|---|
| `int64` | 53.0 ns, 1 alloc | **23.3 ns**, 1 alloc |
| `int` | 43.0 ns, 1 alloc | **3.0 ns**, 0 allocs |
| `float64` | 89.0 ns, 1 alloc | **61.9 ns**, 1 alloc |
| date | 70.3 ns | **34.4 ns** |
| timestamp | 104.9 ns | **43.5 ns** |
| 8-column row | 323.4 ns | **227.1 ns** |

### Why concurrency was not used

Splitting the work across connections was measured and did not help on a local
database: four concurrent range scans over 5,000,000 rows took 5.07 s, against
4.72 s for a single serial scan. One connection already saturates the server,
so parallel readers add contention instead of throughput. The remaining gap to
`COPY` is per-row client-side work, which is addressed by the formatting changes
rather than by more goroutines. On a high-latency remote database the picture
would differ, but that is a measurement to make per deployment, not a default.

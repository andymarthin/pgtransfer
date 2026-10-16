# PGTransfer v1.1.0

A robustness, correctness and tooling release. This version hardens backup and
restore against PostgreSQL client/server version skew, makes connections far
more reliable, adds a proper automated test suite with CI, and documents the
full local testing stack.

## ⚠️ Behaviour changes

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

## ✨ New features

- **`pgtransfer import dump`** gains four flags:
  - `--clean` — drop existing objects before restoring (`pg_restore` only).
  - `--verbose` — verbose output from the restore tools.
  - `--timeout <seconds>` — abort the restore after the given time (`0` = none).
  - `--strict` — fail on the first restore error instead of tolerating
    version-skew noise.
- **New `--version` / `-v` support** on the root command, sharing the output of
  the `version` subcommand.

## 🐛 Fixes & robustness

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

## 🧪 Testing & CI

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

## 📖 Documentation

- README updated with a CI badge, a full **Test Suite** section (unit vs.
  integration, environment-variable reference, how to run everything), and the
  new `import dump` flags.
- `scripts/README.md` documents the SSH image and integration-test workflow.

## 📥 Downloads

Choose the appropriate binary for your platform:

- **Linux (x64)**: `pgtransfer-v1.1.0-linux-amd64.tar.gz`
- **Linux (ARM64)**: `pgtransfer-v1.1.0-linux-arm64.tar.gz`
- **macOS (Intel)**: `pgtransfer-v1.1.0-darwin-amd64.tar.gz`
- **macOS (Apple Silicon)**: `pgtransfer-v1.1.0-darwin-arm64.tar.gz`

## 🚀 Upgrade

Download the archive for your platform, extract `pgtransfer`, and replace your
existing binary. Existing profiles and configuration under `~/.pgtransfer/` are
fully compatible.

**Full changelog:** [`v1.0.1...v1.1.0`](https://github.com/andymarthin/pgtransfer/compare/v1.0.1...v1.1.0)

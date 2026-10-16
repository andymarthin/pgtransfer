# homebrew-pgtransfer

Homebrew tap for [pgtransfer](https://github.com/andymarthin/pgtransfer) — transfer
PostgreSQL data between databases or CSV files.

## Install

```bash
brew tap andymarthin/pgtransfer
brew install pgtransfer
```

Or in one line:

```bash
brew install andymarthin/pgtransfer/pgtransfer
```

Verify:

```bash
pgtransfer version
```

## Upgrade

```bash
brew update
brew upgrade pgtransfer
```

## Uninstall

```bash
brew uninstall pgtransfer
brew untap andymarthin/pgtransfer
```

## Prerequisites

`pg_dump`, `pg_restore` and `psql` must be on your `PATH` for dump/restore
operations, and an OpenSSH client is required for SSH tunnels:

```bash
brew install postgresql@17
```

## How this tap stays up to date

`Formula/pgtransfer.rb` points at the prebuilt release archives from
[pgtransfer releases](https://github.com/andymarthin/pgtransfer/releases).
The `Update formula` workflow re-runs on every pgtransfer release (and daily as a
safety net), downloads the published `SHA256SUMS`, rewrites the version and
per-platform checksums, and commits the change.

Nothing in this tap is built from source: the formula installs the released
binary for your platform.

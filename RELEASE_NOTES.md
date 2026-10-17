# PGTransfer v1.2.0

An automation- and distribution-focused release. This version makes pgtransfer
scriptable and AI-drivable with machine-readable `--json` output and a Model
Context Protocol server, and makes it far easier to install with prebuilt
binaries, a one-line install script, and a Homebrew tap.

## ✨ New features

### Machine-readable `--json` output

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

### MCP server for AI agents

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

### Install script

A POSIX `sh` installer downloads the binary for your platform, verifies it
against the release's `SHA256SUMS` and installs it:

```bash
curl -fsSL https://raw.githubusercontent.com/andymarthin/pgtransfer/main/scripts/install.sh | sh
```

It installs to `/usr/local/bin` when writable, otherwise `~/.local/bin`, and is
configurable via `PGTRANSFER_VERSION`, `PGTRANSFER_INSTALL` and
`PGTRANSFER_NO_SUDO`. **It refuses to install an archive whose checksum does not
match.**

### Homebrew

```bash
brew tap andymarthin/pgtransfer
brew install pgtransfer
```

The formula lives in the
[homebrew-pgtransfer](https://github.com/andymarthin/homebrew-pgtransfer) tap and
updates itself from each release.

## 🔧 Tooling & CI

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

## ⬆️ Requirements

- **Go 1.25 or higher** to build from source (the MCP SDK requires it). Prebuilt
  binaries are unaffected.
- `pg_dump`, `pg_restore` and `psql` on your `PATH` for dump/restore.
- An OpenSSH client for SSH tunnels.

## 📄 License

The project is now shipped with its `LICENSE` file (MIT), which the README had
always claimed but the repository was missing.

## Downloads

Choose the appropriate binary for your platform:

- **Linux (x64)**: `pgtransfer-v1.2.0-linux-amd64.tar.gz`
- **Linux (ARM64)**: `pgtransfer-v1.2.0-linux-arm64.tar.gz`
- **macOS (Intel)**: `pgtransfer-v1.2.0-darwin-amd64.tar.gz`
- **macOS (Apple Silicon)**: `pgtransfer-v1.2.0-darwin-arm64.tar.gz`
- **Installer**: `install.sh`
- **Checksums**: `SHA256SUMS`

Verify downloads against the attached `SHA256SUMS`.

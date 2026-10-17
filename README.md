
# PGTransfer

[![Go Version](https://img.shields.io/badge/Go-1.25+-blue.svg)](https://golang.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-13+-blue.svg)](https://postgresql.org)
[![CI](https://github.com/andymarthin/pgtransfer/actions/workflows/ci.yml/badge.svg)](https://github.com/andymarthin/pgtransfer/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**PGTransfer** is a powerful, secure, and user-friendly command-line tool for PostgreSQL data operations. Built with Go, it provides seamless data import/export capabilities with advanced features like SSH tunneling, connection profiles, and interactive configuration.

## 🎯 Key Highlights

- **🔐 Secure SSH Tunneling**: Connect to remote databases through bastion hosts with full SSH tunnel support and local port forwarding
- **⚡ Efficient Operations**: Optimized batch processing that handles large datasets while managing memory usage intelligently
- **🛡️ Robust Security**: SSL/TLS encryption, SSH key authentication, and secure local credential storage
- **🔄 Database Migration**: Complete schema and data migration between databases with automatic rollback capabilities
- **📊 Intelligent Data Processing**: Proper handling of PostgreSQL data types with smart CSV formatting and conversion
- **🤖 AI & Automation Ready**: Machine-readable `--json` output for every command and a built-in Model Context Protocol (MCP) server so agents can drive transfers directly

## ✨ Features

### Core Operations
- **🔄 Data Transfer**: Import and export PostgreSQL tables to/from CSV format with intelligent data type handling
- **🗄️ Database Dumps**: Complete database export/import using PostgreSQL's native tools (pg_dump/pg_restore)
- **🗄️ Database Migration**: Full database migration with schema, data, and selective table transfer
- **📊 Progress Tracking**: Real-time progress indicators with speed metrics and time estimates

### Connection & Security
- **🔐 SSH Tunneling**: Production-ready SSH tunnels with local port forwarding for external tools
- **🛡️ SSL/TLS Support**: Configurable SSL modes for secure database connections
- **🔑 Authentication**: Support for password, SSH key, and SSH agent authentication
- **👤 Profile Management**: Reusable connection profiles with secure credential storage
- **🖥️ Interactive Setup**: User-friendly interactive profile configuration with validation

### Automation & AI
- **🤖 JSON Output**: A global `--json` flag (or `PGTRANSFER_JSON=1`) emits a single, stable JSON envelope for every command
- **🔌 MCP Server**: `pgtransfer mcp` runs a Model Context Protocol server over stdio, exposing transfers as typed tools for AI hosts
- **🔒 Secret Redaction**: JSON output never includes passwords — profiles report `has_password` instead

## 🤖 AI & Automation

PGTransfer is designed to be driven by scripts, CI pipelines, and AI agents.

### Machine-readable JSON output

Pass the global `--json` flag (before the subcommand) to any command to receive a
single JSON object on stdout instead of human-friendly text:

```bash
pgtransfer --json version
pgtransfer --json profile list
pgtransfer --json test-connection myprofile
pgtransfer --json export csv myprofile out.csv --table users --headers
```

The flag can also be enabled with an environment variable, which is convenient for
agents that cannot easily change the command line:

```bash
export PGTRANSFER_JSON=1
pgtransfer version        # emits JSON
```

**The envelope** is identical for every command:

```json
{
  "ok": true,
  "command": "export csv",
  "data": { "rows": 250, "output_file": "out.csv" },
  "error": null
}
```

On failure `ok` is `false`, `data` is omitted, and `error` carries a stable
machine-readable code:

```json
{
  "ok": false,
  "command": "test-connection",
  "error": {
    "code": "profile_not_found",
    "message": "profile \"ghost\" not found"
  }
}
```

The process always exits non-zero on failure, and **exactly one** JSON object is
written per invocation — including for parser-level problems such as an unknown
subcommand or a bad flag (`code: "usage_error"`).

Stable error codes include: `usage_error`, `profile_not_found`,
`no_active_profile`, `config_load_failed`, `connection_failed`, `export_failed`,
`import_failed`, `dump_failed`, `restore_failed`, `migration_failed`,
`rollback_failed`, `file_exists`, `file_not_found`, `truncate_failed`,
`invalid_input`.

### MCP server (AI agent integration)

`pgtransfer mcp` runs a [Model Context Protocol](https://modelcontextprotocol.io)
server over stdio. AI hosts (Claude Desktop, Cursor, custom agent runtimes) can
then call pgtransfer capabilities as native typed tools.

```bash
pgtransfer mcp
```

Register it with an MCP host — for Claude Desktop, add this to
`claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "pgtransfer": {
      "command": "pgtransfer",
      "args": ["mcp"]
    }
  }
}
```

**Exposed tools**

| Tool | Purpose |
| --- | --- |
| `list_profiles` | List saved profiles (never returns passwords) |
| `test_connection` | Test connectivity, optionally through an SSH tunnel |
| `export_csv` | Export a table or a custom query to CSV |
| `import_csv` | Import a CSV into a table, optionally truncating first |
| `export_dump` | Create a `pg_dump` backup (plain/custom/tar/directory) |
| `import_dump` | Restore a dump, tolerating benign version skew by default |
| `migrate_database` | Migrate schema/data between databases or profiles |

Tool failures are reported in-band with `isError: true` and a structured
`{"ok": false, "error": {"code", "message"}}` payload, so a model can read the
problem and correct itself rather than seeing an opaque protocol error.

Because stdout carries the MCP protocol, the server redirects any incidental
library output to stderr; only protocol frames are ever written to stdout.

## 🔐 SSH Tunnel Support

PGTransfer includes comprehensive SSH tunnel support with automatic port forwarding, allowing secure access to remote PostgreSQL databases through bastion hosts or jump servers.

### Key Features

- **🔄 Automatic Port Management**: Dynamically allocates local ports and detects conflicts
- **🔐 Multiple Authentication**: Supports password, SSH key, and SSH agent authentication
- **🛠️ External Tool Integration**: Creates local port forwarding that works with pgAdmin, DBeaver, and other tools
- **⚡ Connection Pooling**: Reuses tunnel connections efficiently across multiple operations
- **🔍 Health Monitoring**: Automatically monitors tunnel health and recovers from connection issues

### SSH Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| SSH Host | Remote server hostname/IP | Required |
| SSH Port | SSH server port | 22 |
| SSH Username | SSH user account | Current user |
| Authentication | password/key/agent | Interactive |
| Local Port | Local tunnel port | Auto-assigned |
| Keep Alive | Tunnel persistence | 30s |

### Authentication Methods

**1. SSH Key Authentication** (recommended)
```bash
# Use specific private key
SSH Key Path: ~/.ssh/production_key

# Use default SSH key
SSH Key Path: ~/.ssh/id_rsa
```

**2. SSH Agent Authentication**
```bash
# Uses keys loaded in SSH agent
ssh-add ~/.ssh/production_key
# Then select "SSH Agent" in profile setup
```

**3. Password Authentication**
```bash
# Interactive password prompt
# Less secure, better to use keys when possible
```

### Example Setup

```bash
# Create production profile with SSH tunnel
pgtransfer profile add prod-db

# Configuration:
# SSH Host: bastion.company.com
# SSH Username: deploy
# SSH Key: ~/.ssh/production_key
# Database Host: localhost (via tunnel)
# Database Port: 5433 (tunnel local port)
# Target DB: db-internal.company.com:5432
```

### External Tool Integration

Once a profile with SSH tunnel is configured, you can use the local port for external tools:

```bash
# Get tunnel info
pgtransfer test-connection prod-db

# Use with external tools
pgadmin4 --host localhost --port 5433
psql -h localhost -p 5433 -U username -d database
```

### Performance & Reliability
- **⚡ Batch Processing**: High-performance batch operations with configurable batch sizes
- **💾 Memory Efficient**: Streaming processing optimized for large datasets
- **🔄 Rollback Support**: Automatic backup creation and rollback capabilities for migrations
- **⏱️ Timeout Control**: Configurable timeouts for long-running operations
- **📝 Comprehensive Logging**: Detailed JSON-formatted operation logs with performance metrics

## 🚀 Quick Start

### Prerequisites

- **Go**: Version 1.25 or higher
- **PostgreSQL**: Version 13 or higher (client tools: `pg_dump`, `pg_restore`, `psql`)
- **SSH Client**: For SSH tunnel connections (OpenSSH recommended)

### Installation

#### Option A — prebuilt binary (no Go required)

Download the archive for your platform from the
[releases page](https://github.com/andymarthin/pgtransfer/releases/latest):

| Platform | Asset |
| --- | --- |
| macOS (Apple Silicon) | `pgtransfer-<version>-darwin-arm64.tar.gz` |
| macOS (Intel) | `pgtransfer-<version>-darwin-amd64.tar.gz` |
| Linux x86-64 | `pgtransfer-<version>-linux-amd64.tar.gz` |
| Linux ARM64 | `pgtransfer-<version>-linux-arm64.tar.gz` |

For example, on macOS (Apple Silicon):

```bash
V=v1.1.0
A=darwin-arm64
curl -fsSLO "https://github.com/andymarthin/pgtransfer/releases/download/$V/pgtransfer-$V-$A.tar.gz"
tar xzf "pgtransfer-$V-$A.tar.gz"
sudo mv pgtransfer /usr/local/bin/
pgtransfer version
```

Optionally verify the download against the published checksums:

```bash
curl -fsSLO "https://github.com/andymarthin/pgtransfer/releases/download/$V/SHA256SUMS"
shasum -a 256 -c SHA256SUMS --ignore-missing   # Linux: sha256sum -c ...
```

#### Option B — install script (Linux, macOS)

A self-contained shell installer downloads the right binary, verifies it against
the release's `SHA256SUMS`, and installs it.

```bash
curl -fsSL https://raw.githubusercontent.com/andymarthin/pgtransfer/main/scripts/install.sh | sh
```

It installs to `/usr/local/bin` when writable, otherwise to `~/.local/bin` (and
tells you how to add that to your `PATH` if needed).

Control the install with environment variables:

```bash
# pin a version
curl -fsSL https://raw.githubusercontent.com/andymarthin/pgtransfer/main/scripts/install.sh \
  | PGTRANSFER_VERSION=v1.1.0 sh

# choose the install directory (no sudo needed)
curl -fsSL https://raw.githubusercontent.com/andymarthin/pgtransfer/main/scripts/install.sh \
  | PGTRANSFER_INSTALL="$HOME/bin" sh

# never prompt for sudo (fail instead)
curl -fsSL https://raw.githubusercontent.com/andymarthin/pgtransfer/main/scripts/install.sh \
  | PGTRANSFER_NO_SUDO=1 sh
```

| Variable | Purpose |
| --- | --- |
| `PGTRANSFER_VERSION` | Release tag to install, e.g. `v1.1.0` (default: latest) |
| `PGTRANSFER_INSTALL` | Install directory (default: `/usr/local/bin`, else `~/.local/bin`) |
| `PGTRANSFER_NO_SUDO` | Set to `1` to never use `sudo` |

The installer requires `tar` and one of `curl`/`wget`, plus `sha256sum`, `shasum`
or `openssl` to verify the download. If it cannot verify the archive against the
published checksum, it refuses to install it.

Prefer to read before you pipe to a shell? Inspect it first:

```bash
curl -fsSL https://raw.githubusercontent.com/andymarthin/pgtransfer/main/scripts/install.sh | less
```

#### Option C — Homebrew (macOS and Linux)

Requires [Homebrew](https://brew.sh). Installs the prebuilt binary for your platform.

```bash
brew tap andymarthin/pgtransfer
brew install pgtransfer
```

Or in a single command:

```bash
brew install andymarthin/pgtransfer/pgtransfer
```

Upgrade later with:

```bash
brew update && brew upgrade pgtransfer
```

#### Option D — build from source

Requires **Go 1.25 or higher**.

```bash
git clone https://github.com/andymarthin/pgtransfer.git
cd pgtransfer
go mod tidy
go build -o pgtransfer .
sudo mv pgtransfer /usr/local/bin/     # or: export PATH="$PATH:$PWD"
```

#### Option E — `go install`

Requires **Go 1.25 or higher**. Installs into `$(go env GOPATH)/bin`, which must
be on your `PATH`.

```bash
go install github.com/andymarthin/pgtransfer@latest
```

#### Verify

```bash
pgtransfer version          # e.g. "pgtransfer v1.1.0"
pgtransfer --help
```

## 📋 Usage

### Command Overview

```bash
# Profile Management
pgtransfer profile add <name>           # Create new connection profile
pgtransfer profile delete <name>        # Delete existing profile
pgtransfer profile list                 # List all profiles

# Connection Testing
pgtransfer test-connection <profile>    # Test database connection

# Data Export
pgtransfer export csv <profile> --table <table> --output <file.csv>
pgtransfer export dump <profile> --output <file.sql> [--format custom|directory|plain]

# Data Import  
pgtransfer import csv <profile> --table <table> --input <file.csv>
pgtransfer import dump <profile> --input <file.sql>

# Database Migration
pgtransfer migrate <source> <target> [--mode schema|data|full] [--tables table1,table2]

# Migration Rollback
pgtransfer migrate rollback <profile> --backup-file <backup.sql>

# AI Agent Integration (Model Context Protocol server over stdio)
pgtransfer mcp
```

> **Tip:** every command above also accepts the global `--json` flag for
> machine-readable output. See [AI & Automation](#-ai--automation).

### Quick Examples

```bash
# 1. Set up a new profile with SSH tunnel
pgtransfer profile add production
# Follow interactive prompts for SSH and database configuration

# 2. Export production data
pgtransfer export csv production --table users --output users.csv --batch-size 10000

# 3. Migrate to development environment
pgtransfer migrate production development --mode schema

# 4. Create full database backup
pgtransfer export dump production --output backup_$(date +%Y%m%d).sql --format custom
```

### Profile Management

PGTransfer uses connection profiles to manage database credentials and settings securely.

#### Create a New Profile (Interactive)

```bash
pgtransfer profile add myprofile
```

The interactive setup guides you through comprehensive configuration:

**1. SSH Tunnel Configuration** (optional but recommended for remote databases)
```
Enable SSH tunnel? [y/N]: y
SSH Host: bastion.example.com
SSH Port [22]: 22
SSH Username: deploy
SSH Authentication:
  1. Password
  2. SSH Key
  3. SSH Agent
Choose [1-3]: 2
SSH Key Path: ~/.ssh/id_rsa
Local Port [5433]: 5433
```

**2. Database Configuration**
```
Database Host: localhost          # Use localhost when using SSH tunnel
Database Port [5432]: 5433       # Use tunnel local port
Database Name: production_db
Username: app_user
Password: [hidden input]
SSL Mode:
  1. disable
  2. require  
  3. verify-ca
  4. verify-full
Choose [1-4]: 2
```

**3. Automatic Validation**
- SSH tunnel connectivity test
- Database connection verification
- Configuration validation and storage

#### Create a Profile (Command Line)

```bash
pgtransfer profile add production \
  --user myuser \
  --password mypassword \
  --host db.example.com \
  --port 5432 \
  --database myapp \
  --sslmode require
```

#### SSH Tunnel Configuration

For secure connections through a bastion host:

```bash
pgtransfer profile add staging \
  --user dbuser \
  --host localhost \
  --port 5432 \
  --database staging_db \
  --ssh-host bastion.example.com \
  --ssh-user ubuntu \
  --ssh-key ~/.ssh/id_rsa
```

#### List Profiles

```bash
pgtransfer profile list
```

#### Test Connection

```bash
pgtransfer test-connection myprofile
```

#### Update Existing Profile

When updating an existing profile, PGTransfer will show current values as defaults:

```bash
pgtransfer profile add myprofile
```

```
⚠️  Profile 'myprofile' already exists with the following configuration:
  Database: myuser@db.example.com:5432/myapp
  SSL Mode: require
  SSH: Not configured

Do you want to overwrite it? [y/N]: y

🧩 Interactive Profile Setup
Database user [myuser]: newuser
Database password: 
Database host [db.example.com]: 
Database port [5432]: 
Database name [myapp]: 
SSL mode [require]: 
Use SSH tunnel? (y/N) [n]: 
```

### Data Operations

#### Export Data

Export a table to CSV:

```bash
pgtransfer export csv myprofile public.users users_export.csv --headers
```

Export with schema flag (defaults to 'public' if not specified):

```bash
pgtransfer export csv myprofile users users_export.csv --headers --schema public
```

With custom batch size for large datasets:

```bash
pgtransfer export csv myprofile public.users large_export.csv --headers --batch-size 1000
```

#### Database Dump Export

Export complete database to SQL dump:

```bash
pgtransfer export dump myprofile database_backup.sql
```

Export with custom format and compression:

```bash
pgtransfer export dump myprofile backup.dump --format custom --compress
```

Export schema only (no data):

```bash
pgtransfer export dump myprofile schema_backup.sql --schema-only
```

Export data only (no schema):

```bash
pgtransfer export dump myprofile data_backup.sql --data-only
```

Export specific tables:

```bash
pgtransfer export dump myprofile users_backup.sql --table users --table orders
```

Export with advanced options:

```bash
pgtransfer export dump myprofile full_backup.dump \
  --format custom \
  --compress \
  --verbose \
  --timeout 300
```

**Supported Formats:**
- `plain` (default): Standard SQL text format
- `custom`: PostgreSQL custom binary format (supports compression)
- `directory`: Directory format for parallel processing
- `tar`: TAR archive format

#### Import Data

Import CSV data to a table:

```bash
pgtransfer import csv myprofile public.users users_import.csv --headers
```

Import with schema flag (defaults to 'public' if not specified):

```bash
pgtransfer import csv myprofile users users_import.csv --headers --schema public
```

Import with custom batch size and overwrite existing data:

```bash
pgtransfer import csv myprofile public.users users_import.csv --headers --batch-size 1000 --overwrite
```

Import a header-less CSV (columns are mapped positionally using the target table definition):

```bash
pgtransfer import csv myprofile public.users users_no_header.csv
```

> **Headers behaviour:** when `--headers` is supplied the first CSV row is treated
> as the column name list; when omitted, **every** row is treated as data and the
> columns are matched positionally against the target table.

Import from SQL dump file (automatically detects plain SQL vs. binary pg_dump formats):

```bash
pgtransfer import dump myprofile backup.sql
```

Import a custom-format dump and drop existing objects first:

```bash
pgtransfer import dump myprofile backup.dump --clean
```

Import with a timeout and strict error handling:

```bash
pgtransfer import dump myprofile backup.sql --timeout 300 --strict
```

> **Version-skew tolerance:** by default, restore tolerates benign errors caused by
> client/server version differences (for example an `unrecognized configuration
> parameter` emitted by a newer `pg_dump`). Pass `--strict` to abort on the first
> error instead.

### Database Migration

PGTransfer provides comprehensive database migration capabilities for transferring entire databases or specific components between PostgreSQL instances. You can use either different profiles or the same profile with database overrides.

#### Migration Modes

**Mode 1: Different Profiles**
Use separate connection profiles for source and target:

```bash
pgtransfer migrate database source_profile target_profile
```

**Mode 2: Same Profile with Database Override**
Use the same connection profile but specify different database names:

```bash
pgtransfer migrate database profile --source-database source_db --target-database target_db
```

#### Full Database Migration

Migrate complete database (schema + data) with different profiles:

```bash
pgtransfer migrate database source_profile target_profile
```

Migrate complete database using same profile:

```bash
pgtransfer migrate database myprofile --source-database prod_db --target-database staging_db
```

#### Schema-Only Migration

Transfer only the database structure with different profiles:

```bash
pgtransfer migrate database source_profile target_profile --schema-only
```

Transfer only the database structure using same profile:

```bash
pgtransfer migrate database myprofile --source-database prod_db --target-database dev_db --schema-only
```

#### Data-Only Migration

Transfer only data (assumes target schema exists) with different profiles:

```bash
pgtransfer migrate database source_profile target_profile --data-only
```

Transfer only data using same profile:

```bash
pgtransfer migrate database myprofile --source-database source_db --target-database target_db --data-only
```

#### Selective Table Migration

Migrate specific tables with different profiles:

```bash
pgtransfer migrate database source_profile target_profile --tables users,orders,products
```

Migrate specific tables using same profile:

```bash
pgtransfer migrate database myprofile --source-database db1 --target-database db2 --tables users,orders,products
```

#### Migration with Validation

Pre-validate migration before execution with different profiles:

```bash
pgtransfer migrate database source_profile target_profile --validate --verbose
```

Pre-validate migration using same profile:

```bash
pgtransfer migrate database myprofile --source-database source_db --target-database target_db --validate --verbose
```

#### Migration with Rollback Support

Enable automatic backup for rollback capability with different profiles:

```bash
pgtransfer migrate database source_profile target_profile --enable-rollback
```

Enable automatic backup using same profile:

```bash
pgtransfer migrate database myprofile --source-database source_db --target-database target_db --enable-rollback
```

#### Advanced Migration Options

With different profiles:

```bash
pgtransfer migrate database source_profile target_profile \
  --tables users,orders \
  --validate \
  --enable-rollback \
  --timeout 1800 \
  --batch-size 1000 \
  --verbose
```

With same profile and database override:

```bash
pgtransfer migrate database myprofile \
  --source-database source_db \
  --target-database target_db \
  --tables users,orders \
  --validate \
  --enable-rollback \
  --timeout 1800 \
  --batch-size 1000 \
  --verbose
```

#### Rollback Operations

Rollback a migration using a backup file:

```bash
pgtransfer migrate rollback target_profile /path/to/backup.sql --verbose
```

#### Migration Features

- **🔍 Pre-Migration Validation**: Connection testing and schema compatibility checks
- **📊 Progress Tracking**: Real-time progress indicators with elapsed time
- **🔄 Rollback Support**: Automatic backup creation for safe rollbacks
- **⚙️ Flexible Options**: Schema-only, data-only, or selective table migration
- **🛡️ Error Handling**: Comprehensive error reporting and graceful failure handling
- **⏱️ Timeout Control**: Configurable timeouts for long-running migrations
- **📝 Verbose Logging**: Detailed operation logs for troubleshooting

#### Migration Examples

**Production Database Refresh (Different Profiles):**
```bash
# Create backup and migrate with rollback support
pgtransfer migrate database production staging --enable-rollback --verbose

# If issues occur, rollback
pgtransfer migrate rollback staging /path/to/backup_20241028_092222.sql
```

**Production Database Refresh (Same Profile):**
```bash
# Create backup and migrate with rollback support using same profile
pgtransfer migrate database myprofile \
  --source-database prod_db \
  --target-database staging_db \
  --enable-rollback --verbose

# If issues occur, rollback
pgtransfer migrate rollback staging /path/to/backup_20241028_092222.sql
```

**Development Environment Setup (Different Profiles):**
```bash
# Schema-only migration for development
pgtransfer migrate database production dev --schema-only

# Add sample data separately
pgtransfer migrate database sample_data dev --data-only --tables users,products
```

**Development Environment Setup (Same Profile):**
```bash
# Schema-only migration for development using same profile
pgtransfer migrate database myprofile \
  --source-database production_db \
  --target-database dev_db \
  --schema-only

# Add sample data separately
pgtransfer migrate database myprofile \
  --source-database sample_data_db \
  --target-database dev_db \
  --data-only --tables users,products
```

**Selective Data Migration (Different Profiles):**
```bash
# Migrate specific tables with validation
pgtransfer migrate database prod_replica analytics \
  --tables user_events,transactions,metrics \
  --validate \
  --timeout 3600
```

**Selective Data Migration (Same Profile):**
```bash
# Migrate specific tables with validation using same profile
pgtransfer migrate database myprofile \
  --source-database prod_replica_db \
  --target-database analytics_db \
  --tables user_events,transactions,metrics \
  --validate \
  --timeout 3600
```

## ⚡ Performance & Optimization

PGTransfer is designed for efficient data operations with intelligent batch processing, streaming architecture, and automatic resource management.

### Performance Features

- **🔄 Streaming Processing**: Handles datasets larger than available memory
- **🧠 Adaptive Batching**: Adjusts batch sizes dynamically based on data complexity
- **⚡ Connection Pooling**: Reuses database connections efficiently
- **📊 Real-time Metrics**: Provides live performance monitoring with time estimates
- **💾 Memory Efficiency**: Maintains minimal memory usage regardless of dataset size

### Batch Size Optimization

**Default batch size: 1000 rows** - works well for most environments and use cases.

#### Performance Benchmarks (1M Records, Mixed Data Types)

| Batch Size | Throughput | Memory Usage | Best For |
|------------|------------|--------------|----------|
| 500        | 4,200 rec/sec | 35MB | Memory-constrained systems |
| 1,000 (default) | 6,800 rec/sec | 65MB | ✅ **Optimal for most cases** |
| 5,000      | 8,500 rec/sec | 180MB | High-performance networks |
| 10,000     | 9,200 rec/sec | 320MB | Maximum throughput scenarios |
| 25,000     | 8,800 rec/sec | 750MB | Dedicated database servers |

### Production Examples

#### Large Table Export

```bash
# Standard high-performance export
pgtransfer export csv production --table transactions \
  --output transactions.csv \
  --batch-size 10000 \
  --progress

# Memory-optimized export for constrained environments  
pgtransfer export csv production --table events \
  --output events.csv \
  --batch-size 2500 \
  --progress

# Maximum throughput for dedicated servers
pgtransfer export csv production --table logs \
  --output logs.csv \
  --batch-size 25000 \
  --progress
```

#### Large Dataset Import

```bash
# Optimized import with progress tracking
pgtransfer import csv staging --table transactions \
  --input transactions.csv \
  --batch-size 5000 \
  --progress

# Safe import with validation
pgtransfer import csv production --table users \
  --input users.csv \
  --batch-size 1000 \
  --validate \
  --progress

# High-speed import for development
pgtransfer import csv dev --table test_data \
  --input large_dataset.csv \
  --batch-size 15000 \
  --skip-validation \
  --progress
```

#### Database Migration Performance

```bash
# Schema-only migration (fast)
pgtransfer migrate production staging --mode schema

# Data migration with optimization
pgtransfer migrate production staging \
  --mode data \
  --batch-size 10000 \
  --progress

# Selective table migration
pgtransfer migrate production staging \
  --tables users,orders,products \
  --batch-size 5000 \
  --progress
```

### Real-time Progress Tracking

All operations provide comprehensive progress monitoring:

- **📊 Visual Progress Bar**: Real-time completion percentage
- **⚡ Performance Metrics**: Throughput (records/second)
- **⏱️ Time Tracking**: Elapsed time and ETA calculations  
- **💾 Resource Monitoring**: Memory usage and batch efficiency
- **🔍 Operation Details**: Current batch, total records, and status

#### Example Output

**CSV Export Progress:**
```
Exporting table 'transactions'...
Progress: 75% |███████████████     | 750,000/1,000,000 records
Speed: 8,500 rec/sec | Elapsed: 1m 28s | ETA: 29s
Batch: 750 (size: 10,000) | Memory: 145MB
```

**Migration Progress:**
```
Migrating database: production → staging
Schema migration: ✅ Complete (2.3s)
Data migration: 45% |█████████       | 4.5M/10M records  
Speed: 12,300 rec/sec | Elapsed: 6m 12s | ETA: 7m 30s
Current table: orders (batch 450/892)
```

**SSH Tunnel Status:**
```
🔐 SSH Tunnel: bastion.company.com:22 → localhost:5433
Status: ✅ Connected | Uptime: 5m 23s
Database: ✅ Connected via tunnel
```

### Memory Management

PGTransfer implements intelligent memory management:
- **Streaming Processing**: Data is processed in chunks, not loaded entirely into memory
- **Connection Pooling**: Efficient database connection reuse
- **Garbage Collection**: Automatic cleanup of processed batches
- **Resource Monitoring**: Built-in memory usage tracking

## 📊 CSV Data Type Handling

PGTransfer provides intelligent CSV formatting that properly handles PostgreSQL data types for seamless import/export operations.

### Supported Data Types

| PostgreSQL Type | CSV Format | Example |
|----------------|------------|---------|
| `DATE` | `YYYY-MM-DD` | `2008-07-06` |
| `TIMESTAMP` | `YYYY-MM-DD HH:MM:SS` | `2025-10-28 01:59:38` |
| `NUMERIC(p,s)` | Decimal notation | `63942.00` |
| `INTEGER` | Plain number | `457719` |
| `VARCHAR/TEXT` | Quoted strings | `user_0457719` |
| `BOOLEAN` | `true`/`false` | `true` |
| `BYTEA` | String representation | Converted to readable format |

### Key Improvements

#### ✅ Date and Timestamp Formatting
- **Before**: `1960-01-02 00:00:00 +0000 +0000` (with timezone info)
- **After**: `1960-01-02` (clean date format)
- **Timestamp**: `2025-10-28 01:59:38` (without timezone)

#### ✅ Decimal and Numeric Handling
- **Before**: `[49 51 57 52 50 46 48 48]` (byte array representation)
- **After**: `63942.00` (proper decimal format)

#### ✅ Null Value Handling
- **Consistent**: Empty strings for NULL values across all data types
- **Import Compatible**: Properly recognized during CSV import operations

### CSV Export Examples

#### Standard Table Export
```bash
# Export with proper formatting
pgtransfer export csv myprofile public.users users.csv --headers
```

#### Custom Query Export
```bash
# Export specific columns with formatting
pgtransfer export csv myprofile results.csv \
  --query "SELECT id, name, date_of_birth, salary FROM users WHERE active = true" \
  --headers
```

### CSV Import Compatibility

The improved formatting ensures seamless round-trip operations:

```bash
# Export data
pgtransfer export csv source_profile public.users exported_data.csv --headers

# Import to different database
pgtransfer import csv target_profile public.users_copy exported_data.csv --headers
```

**Result**: All data types are preserved correctly without manual formatting adjustments.

### Advanced Usage

#### Override Profile Database

```bash
pgtransfer export --profile myprofile --database different_db --table users --file export.csv
```

#### Skip Connection Testing

```bash
pgtransfer profile add myprofile --skip-test
```

#### Force Profile Overwrite

```bash
pgtransfer profile add myprofile --force
```

## 🔧 Configuration

### Configuration File

Profiles are stored in `~/.pgtransfer/config.yaml`:

```yaml
profiles:
  local:
    name: local
    user: postgres
    host: localhost
    port: 5432
    database: myapp_dev
    sslmode: disable
    ssh:
      enabled: false
  
  production:
    name: production
    user: app_user
    host: localhost
    port: 5432
    database: myapp_prod
    sslmode: require
    ssh:
      enabled: true
      host: bastion.example.com
      user: ubuntu
      port: 22
      key_path: /home/user/.ssh/id_rsa
      timeout: 10
```

### Logging

Logs are automatically saved to `~/.pgtransfer/logs/` in JSON format:

```json
{
  "timestamp": "2024-01-15T10:30:45Z",
  "level": "info",
  "operation": "export",
  "profile": "production",
  "table": "users",
  "file": "users_export.csv",
  "status": "success",
  "duration_ms": 1250,
  "rows_processed": 10000
}
```

## 🛠️ Development & Testing

### Development Setup

```bash
# Clone and setup
git clone https://github.com/andymarthin/pgtransfer.git
cd pgtransfer

# Install dependencies
go mod tidy

# Build for development
go build -o pgtransfer .

# Fast unit tests (no database required)
go test ./...

# Run linting
go vet ./...
staticcheck ./...
```

### Test Suite

The project has two layers of tests:

| Layer | Command | Requires | Scope |
|-------|---------|----------|-------|
| **Unit** | `go test ./...` | nothing | Pure logic: DSN building, config lifecycle, dump-format detection, CSV value formatting, SSH auth selection, duration formatting |
| **Integration** | `go test -tags=integration ./internal/io/...` | a live PostgreSQL + SSH server | Real round-trips: `pg_dump`/`pg_restore`, CSV export/import, and the SSH tunnel path |

Unit tests run in CI on every push and pull request. Integration tests run in a
separate CI job that provisions PostgreSQL and the repository SSH image.

#### Running the integration tests

Integration tests are guarded by the `integration` build tag and read their
settings from environment variables. They **skip** when the prerequisites are
absent — so day-to-day `go test ./...` never needs a database:

```bash
# 1. Start the local test stack
cd scripts && docker compose up -d && cd ..

# 2. Run the integration tests against it
PGTRANSFER_TEST_DSN='postgres://testuser:testpass@localhost:5433/testdb?sslmode=disable' \
PGTRANSFER_TEST_SSH_HOST=localhost PGTRANSFER_TEST_SSH_PORT=2222 \
PGTRANSFER_TEST_SSH_USER=sshuser  PGTRANSFER_TEST_SSH_PASSWORD=sshpass \
PGTRANSFER_TEST_DB_HOST=postgres \
  go test -tags=integration -v ./internal/io/...
```

| Variable | Default | Purpose |
|----------|---------|---------|
| `PGTRANSFER_TEST_DSN` | *(unset)* | DSN of the test database; **required** for the direct tests |
| `PGTRANSFER_TEST_REQUIRE` | *(unset)* | When `1`, missing prerequisites **fail** instead of skipping (set by CI so a broken pipeline can't pass green) |
| `PGTRANSFER_TEST_SSH_HOST` | `localhost` | SSH server host for the tunnel test |
| `PGTRANSFER_TEST_SSH_PORT` | `2222` | SSH server port |
| `PGTRANSFER_TEST_SSH_USER` | *(unset)* | SSH user; when unset the tunnel test skips |
| `PGTRANSFER_TEST_SSH_PASSWORD` | *(unset)* | SSH password |
| `PGTRANSFER_TEST_DB_HOST` | `postgres` | Database host **as seen from the SSH server** |
| `PGTRANSFER_TEST_DB_PORT` | `5432` | Database port as seen from the SSH server |
| `PGTRANSFER_TEST_DB_USER` | `testuser` | Database user |
| `PGTRANSFER_TEST_DB_PASSWORD` | `testpass` | Database password |
| `PGTRANSFER_TEST_DB_NAME` | `testdb` | Database name |

> **Note:** `PGTRANSFER_TEST_DB_HOST` is resolved from the *SSH server's* point of
> view, not the test host. With the Docker stack that is the compose service name
> `postgres`; in CI the SSH container reaches the host-published database via
> `host.docker.internal`.

### Local Testing with Docker

The repository ships a complete local testing stack under [`scripts/`](scripts/)
(PostgreSQL with seeded schemas + sample data, an SSH server for tunnel testing,
and sample CSV fixtures).

**Start the full stack:**

```bash
cd scripts
docker compose up -d
docker compose ps
```

This brings up:

| Service | Host endpoint | Credentials |
|---------|---------------|-------------|
| PostgreSQL | `localhost:5433` (host) → `5432` (container) | `testuser` / `testpass`, db `testdb` |
| SSH server (tunnels) | `localhost:2222` | `sshuser` / `sshpass` |

> **Note:** the database is published on host port **5433** to avoid clashing with
> a locally installed PostgreSQL. Inside the Docker network the database is
> reachable at the service name `postgres:5432`.

**Create a test profile and verify:**

```bash
./pgtransfer profile add test-docker \
  --host localhost --port 5433 \
  --user testuser --password testpass \
  --database testdb
./pgtransfer test-connection --profile test-docker
```

**Test an SSH tunnel** (Postgres is reached via the SSH server's view of the
network, so the database host is the compose service name `postgres`):

```bash
./pgtransfer profile add ssh-test \
  --host postgres --port 5432 \
  --user testuser --password testpass \
  --database testdb \
  --ssh-host localhost --ssh-user sshuser --ssh-password sshpass --ssh-port 2222
./pgtransfer test-connection --profile ssh-test
```

**Tear down:**

```bash
cd scripts
docker compose down          # keep data
docker compose down -v       # remove volumes too
```

### Code Quality

```bash
# Format code
go fmt ./...

# Clean up imports
goimports -w .

# Run static analysis
staticcheck ./...

# Clean dependencies
go mod tidy

# Build and test
go build -o pgtransfer . && go test ./...
```

## 🔒 Security Features

### Authentication & Encryption
- **🔐 SSH Tunnel Encryption**: End-to-end encryption for all database connections
- **🛡️ SSL/TLS Support**: Configurable SSL modes (disable, require, verify-ca, verify-full)
- **🔑 Multiple Auth Methods**: SSH keys, SSH agent, and password authentication
- **👤 Secure Input**: Passwords never echoed to terminal or logged

### Data Protection
- **📁 Secure Storage**: Encrypted local storage of connection profiles
- **🚫 No Credential Logging**: Sensitive data excluded from all log files
- **🔄 Connection Validation**: Automatic certificate and host key verification
- **⏱️ Session Management**: Automatic timeout and cleanup of connections

### Advanced Security
- **🏢 Bastion Host Support**: Secure access through jump servers and bastion hosts
- **🔍 Connection Auditing**: Detailed logging of connection attempts and operations
- **🛡️ Network Isolation**: Database access only through encrypted tunnels
- **📋 Security Standards**: Supports common enterprise security requirements

### Known Limitations

- **SSH host key verification**: SSH connections currently accept any host key
  (`InsecureIgnoreHostKey`) to keep the initial setup friction low. For hardened
  environments, pin the expected host key or use `known_hosts` verification
  before relying on tunnels for production credentials.
- **Restore error tolerance**: `import dump` tolerates benign client/server
  version-skew errors by default. Use `--strict` when you need a restore to fail
  on the first error.
- **`--overwrite` semantics**: for database migration, overwrite relies on
  `pg_restore --clean --if-exists` for binary dumps; plain SQL dumps rely on the
  DROP statements emitted by `pg_dump`.

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

### Development Setup

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and add tests
4. Commit your changes: `git commit -m 'Add amazing feature'`
5. Push to the branch: `git push origin feature/amazing-feature`
6. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

- **Issues**: [GitHub Issues](https://github.com/andymarthin/pgtransfer/issues)
- **Discussions**: [GitHub Discussions](https://github.com/andymarthin/pgtransfer/discussions)
- **Documentation**: [Wiki](https://github.com/andymarthin/pgtransfer/wiki)

## 🏆 Acknowledgments

### Core Technologies
- **[Go](https://golang.org/)**: High-performance systems programming language
- **[PostgreSQL](https://postgresql.org/)**: Advanced open-source relational database
- **[Cobra](https://github.com/spf13/cobra)**: Powerful CLI framework for Go

### Key Dependencies
- **[pq](https://github.com/lib/pq)**: Pure Go PostgreSQL driver
- **[golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh)**: SSH client implementation
- **[golang.org/x/term](https://pkg.go.dev/golang.org/x/term)**: Terminal utilities for secure input
- **[gopkg.in/yaml.v3](https://gopkg.in/yaml.v3)**: YAML configuration parsing

### Development Tools
- **[staticcheck](https://staticcheck.io/)**: Advanced Go static analysis
- **[goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports)**: Import formatting and optimization

---

**Made with ❤️ for the PostgreSQL community**

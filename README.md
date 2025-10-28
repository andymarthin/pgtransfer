
# PGTransfer

[![Go Version](https://img.shields.io/badge/Go-1.24.4+-blue.svg)](https://golang.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-13+-blue.svg)](https://postgresql.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**PGTransfer** is a powerful, secure, and user-friendly command-line tool for PostgreSQL data operations. Built with Go, it provides seamless data import/export capabilities with advanced features like SSH tunneling, connection profiles, and interactive configuration.

## ✨ Features

- **🔄 Data Transfer**: Import and export PostgreSQL tables to/from CSV format
- **⚡ Batch Processing**: High-performance batch operations with configurable batch sizes
- **🔐 Secure Connections**: Support for SSL/TLS and SSH tunnel connections
- **👤 Profile Management**: Reusable connection profiles with secure credential storage
- **🖥️ Interactive Setup**: User-friendly interactive profile configuration
- **📊 Progress Tracking**: Real-time progress indicators for large data operations
- **📝 Comprehensive Logging**: Detailed JSON-formatted operation logs
- **🔑 Multiple Authentication**: Support for password and SSH key authentication
- **💾 Memory Efficient**: Optimized for large datasets with efficient memory usage
- **🎯 Smart Routing**: Automatic optimization for different data sizes

## 🚀 Quick Start

### Prerequisites

- **Go**: Version 1.24.4 or newer
- **PostgreSQL**: Version 13 or newer
- **SSH Client**: For SSH tunnel connections (optional)

### Installation

#### From Source

```bash
git clone https://github.com/andymarthin/pgtransfer.git
cd pgtransfer
go build -o pgtransfer
sudo mv pgtransfer /usr/local/bin/
```

#### Verify Installation

```bash
pgtransfer --help
```

## 📋 Usage

### Profile Management

PGTransfer uses connection profiles to manage database credentials and settings securely.

#### Create a New Profile (Interactive)

```bash
pgtransfer profile add myprofile
```

The interactive setup will guide you through:
- Database connection details
- SSL configuration
- SSH tunnel settings (optional)
- Authentication method selection

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

Import from SQL dump file (automatically detects format):

```bash
pgtransfer import dump myprofile backup.sql
```

## ⚡ Batch Processing & Performance

PGTransfer features advanced batch processing capabilities optimized for large datasets. The system automatically handles memory management and provides real-time progress tracking.

### Batch Size Optimization

The default batch size is **500 rows**, which provides optimal performance for most use cases. However, you can customize this based on your specific needs:

#### Performance Benchmarks (1M Records)

| Batch Size | Export Time | Import Time | Memory Usage | Recommended For |
|------------|-------------|-------------|--------------|-----------------|
| 100        | 2m 38s      | ~4m         | Low          | Memory-constrained environments |
| 500 (default) | 3.20s   | 3m 19s      | Moderate     | ✅ **Optimal for most cases** |
| 1,000      | 3.84s       | ~3m         | Moderate     | Large datasets with good network |
| 5,000      | 6.84s       | ~2m 30s     | Higher       | High-performance environments |
| 10,000     | 5.25s       | ~2m         | High         | Maximum throughput scenarios |

### Large Dataset Examples

#### Export 1 Million Records

```bash
# Optimal performance with default batch size
pgtransfer export csv production public.large_table export_1m.csv --headers

# High-throughput export for fast networks
pgtransfer export csv production public.large_table export_1m.csv --headers --batch-size 5000

# Memory-efficient export for constrained environments
pgtransfer export csv production public.large_table export_1m.csv --headers --batch-size 100
```

#### Import Large CSV Files

```bash
# Standard import with progress tracking
pgtransfer import csv production public.target_table large_data.csv --headers

# High-speed import with larger batches
pgtransfer import csv production public.target_table large_data.csv --headers --batch-size 2000

# Safe import with table replacement
pgtransfer import csv production public.target_table large_data.csv --headers --overwrite
```

### Progress Tracking

All operations display real-time progress with:
- **Progress Bar**: Visual completion indicator
- **Speed Metrics**: Rows processed per second
- **Time Estimates**: Elapsed and estimated remaining time
- **Memory Usage**: Current system resource utilization

Example output:
```
Exporting public.users (elapsed 3s) 100% |████████████████████| (1000000/1000000, 322199 it/s)
✅ Exported 1000000 rows to users_export.csv (batch size: 500)
🕒 Duration: 3.15s
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

## 🐳 Development & Testing

### Local Development with Docker

Create a `docker-compose.yml` for testing:

```yaml
version: "3.8"
services:
  postgres:
    image: postgres:15
    container_name: pgtransfer_test
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: testdb
    ports:
      - "5432:5432"
    volumes:
      - ./test_data:/docker-entrypoint-initdb.d
```

Start the test environment:

```bash
docker-compose up -d
```

### Sample Test Data

Create `test_data/init.sql`:

```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    age INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO users (username, email, age) VALUES
    ('alice', 'alice@example.com', 30),
    ('bob', 'bob@example.com', 25),
    ('charlie', 'charlie@example.com', 35),
    ('diana', 'diana@example.com', 28);
```

### Running Tests

```bash
go test ./...
```

## 🔒 Security Features

- **Secure Password Input**: Passwords are never echoed to the terminal
- **SSH Key Support**: Supports both password and key-based SSH authentication
- **SSL/TLS Encryption**: Configurable SSL modes for database connections
- **Credential Storage**: Secure local storage of connection profiles
- **SSH Tunnel Encryption**: All data transfers through SSH tunnels are encrypted

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

- Built with [Go](https://golang.org/)
- PostgreSQL driver: [pq](https://github.com/lib/pq)
- SSH client: [golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh)
- CLI framework: [Cobra](https://github.com/spf13/cobra)

---

**Made with ❤️ by the PGTransfer team**

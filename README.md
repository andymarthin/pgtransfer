
# PGTransfer

[![Go Version](https://img.shields.io/badge/Go-1.24.4+-blue.svg)](https://golang.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-13+-blue.svg)](https://postgresql.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**PGTransfer** is a powerful, secure, and user-friendly command-line tool for PostgreSQL data operations. Built with Go, it provides seamless data import/export capabilities with advanced features like SSH tunneling, connection profiles, and interactive configuration.

## ✨ Features

- **🔄 Data Transfer**: Import and export PostgreSQL tables to/from CSV format
- **🔐 Secure Connections**: Support for SSL/TLS and SSH tunnel connections
- **👤 Profile Management**: Reusable connection profiles with secure credential storage
- **🖥️ Interactive Setup**: User-friendly interactive profile configuration
- **📊 Progress Tracking**: Real-time progress indicators for large data operations
- **📝 Comprehensive Logging**: Detailed JSON-formatted operation logs
- **🔑 Multiple Authentication**: Support for password and SSH key authentication
- **⚡ High Performance**: Optimized for large datasets with efficient memory usage

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
pgtransfer export --profile myprofile --table users --file users_export.csv
```

With custom query:

```bash
pgtransfer export --profile myprofile \
  --query "SELECT id, name, email FROM users WHERE active = true" \
  --file active_users.csv
```

#### Import Data

Import CSV data to a table:

```bash
pgtransfer import --profile myprofile --table users --file users_import.csv
```

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

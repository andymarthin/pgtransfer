
# pgtransfer

`pgtransfer` is a command-line tool written in Go for importing and exporting data from PostgreSQL databases.
It supports direct connections or connections through SSH tunnels and allows reusable connection profiles for convenience.

---

## Features

`pgtransfer` simplifies PostgreSQL data transfer operations.
It supports importing and exporting tables in standard formats, handles secure connections over SSH, and manages connection profiles for multiple environments.

---

## Installation

### Requirements
- Go 1.24.4 or newer
- PostgreSQL 13 or newer
- (Optional) Docker & Docker Compose

### Build
```bash
git clone https://github.com/andymarthin/pgtransfer.git
cd pgtransfer
go build -o pgtransfer
sudo mv pgtransfer /usr/local/bin/


---

## Configuration

Connection profiles are stored in:

```
~/.pgtransfer_config.yaml
```

Each profile can define connection parameters or a full database URL.
SSH settings are optional.

Example configuration:

```yaml
profiles:
  local:
    user: postgres
    password: postgres
    host: localhost
    port: 5432
    database: pgtransfer_test
  staging:
    db_url: postgres://staging_user:secret@staging.example.com:5432/appdb
    ssh_host: bastion.example.com
    ssh_user: ubuntu
    ssh_key: ~/.ssh/id_rsa
```

---

## Profile Management

### Create or update a profile

```bash
pgtransfer profile:add local \
  --user postgres --password postgres \
  --host localhost --port 5432 --database pgtransfer_test
```

### Interactive setup

```bash
pgtransfer profile:add
```

Example:

```
Profile name: local
Host: localhost
Port: 5432
User: postgres
Password: ******
Database: pgtransfer_test
🔍 Testing connection...
✅ Connection test succeeded. Profile 'local' saved.
```

### Overwrite an existing profile

```bash
pgtransfer profile:add local --db postgres://... --force
```

---

## Export Data

Export a PostgreSQL table to a CSV file.

```bash
pgtransfer export --profile local --table users --file users_export.csv
```

Output:

```
Connecting to database: localhost:5432/pgtransfer_test (direct)
Progress: ███████████████████ 100%
✅ Export complete. File saved at ./users_export.csv
```

To override the database defined in a profile:

```bash
pgtransfer export --profile local --database test_db --table users --file users_export.csv
```

---

## Import Data

Import a CSV file into a PostgreSQL table.

```bash
pgtransfer import --profile local --table users --file users_import.csv
```

Output:

```
Connecting to database: localhost:5432/pgtransfer_test (direct)
Progress: ███████████████████ 100%
✅ Import completed successfully.
```

---

## SSH Tunnel Example

If SSH settings are configured in a profile, the tool automatically uses an SSH tunnel.

```bash
pgtransfer export --profile staging --table users --file users_staging.csv
```

Output:

```
Connecting via SSH tunnel (bastion.example.com → localhost:5432)
Progress: ███████████████████ 100%
✅ Export complete.
```

---

## Local Testing with Docker

Example Docker Compose setup:

```yaml
version: "3.8"
services:
  db:
    image: postgres:15
    container_name: pgtransfer_db
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: pgtransfer_test
    ports:
      - "5432:5432"
```

Start the container:

```bash
docker compose up -d
```

---

## Example SQL for Testing

`init_pgtransfer_test.sql`

```sql
CREATE DATABASE pgtransfer_test;
\c pgtransfer_test;

CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  username VARCHAR(50),
  email VARCHAR(100),
  age INT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO users (username, email, age)
VALUES
  ('alice', 'alice@example.com', 30),
  ('bob', 'bob@example.com', 25),
  ('charlie', 'charlie@example.com', 35);
```

Run inside the container:

```bash
docker exec -i pgtransfer_db psql -U postgres -f /tmp/init_pgtransfer_test.sql
```

---

## Example CSV for Import

`users_import.csv`

```csv
username,email,age
david,david@example.com,28
emma,emma@example.com,24
frank,frank@example.com,31
```

---

## Typical Workflow

```bash
# Start local PostgreSQL
docker compose up -d

# Add a profile
pgtransfer profile:add local --user postgres --password postgres --host localhost --database pgtransfer_test

# Import data
pgtransfer import --profile local --table users --file users_import.csv

# Export data
pgtransfer export --profile local --table users --file exported_users.csv
```

---

## Logs

Logs are written as JSON files in:

```
~/.pgtransfer_logs/YYYY-MM-DD.log
```

Example:

```json
{
  "timestamp": "2025-10-27T09:21:12Z",
  "operation": "export",
  "profile": "local",
  "table": "users",
  "file": "users_export.csv",
  "status": "success",
  "duration_ms": 874
}
```

# PGTransfer Testing Environment

This directory contains all the necessary files to set up a complete testing environment for PGTransfer, including Docker Compose configuration, database initialization scripts, and sample CSV data for testing import/export functionality.

## 📁 Directory Structure

```
scripts/
├── README.md                 # This file
├── docker-compose.yml        # Docker Compose configuration
├── init.sql                  # Database schema initialization
├── sample-data.sql           # Sample data population
├── test-data/               # CSV test files directory
│   ├── users.csv            # Sample users data
│   ├── products.csv         # Sample products data
│   ├── customers.csv        # Sample customers data
│   ├── employees.csv        # Sample employees data
│   └── complex-data.csv     # Complex data with edge cases
└── ssh/                     # SSH configuration for testing
    └── authorized_keys      # SSH public keys for testing
```

## 🚀 Quick Start

### 1. Start the Testing Environment

```bash
# Navigate to the scripts directory
cd scripts

# Start all services
docker-compose up -d

# Check service status
docker-compose ps
```

### 2. Verify Database Connection

```bash
# Test connection using PGTransfer
cd ..
./pgtransfer profile add test-docker \
  --host localhost \
  --port 5432 \
  --user testuser \
  --password testpass \
  --database testdb \
  --ssl-mode disable

# Test the connection
./pgtransfer test-connection test-docker
```

### 3. Access Services

- **PostgreSQL Database**: `localhost:5432`
  - Database: `testdb`
  - Username: `testuser`
  - Password: `testpass`

- **pgAdmin Web Interface**: `http://localhost:8080`
  - Email: `admin@pgtransfer.local`
  - Password: `admin123`

- **SSH Server** (for tunnel testing): `localhost:2222`
  - Username: `sshuser`
  - Password: `sshpass`

## 🗄️ Database Schema

The testing database includes multiple schemas with realistic data:

### Public Schema
- **users** - User accounts and profiles
- **posts** - Blog posts and articles
- **comments** - User comments on posts

### Sales Schema
- **customers** - Customer information and contacts
- **orders** - Sales orders and transactions
- **order_items** - Individual items within orders

### Inventory Schema
- **products** - Product catalog and pricing
- **stock_movements** - Inventory tracking and movements

### HR Schema
- **employees** - Employee records and information
- **attendance** - Employee attendance tracking

## 📊 Sample Data

The database is pre-populated with realistic sample data:

- **10 users** with posts and comments
- **8 customers** with order history
- **10 orders** with multiple line items
- **15 products** across different categories
- **10 employees** with attendance records
- **Stock movements** and inventory tracking

## 📁 CSV Test Files

The `test-data/` directory contains CSV files for testing import functionality:

### Standard Test Files
- **users.csv** - 15 new users for import testing
- **products.csv** - 15 new products with various categories
- **customers.csv** - 12 new customers with complete information
- **employees.csv** - 10 new employees for HR testing

### Edge Case Testing
- **complex-data.csv** - Contains edge cases including:
  - Fields with quotes and commas
  - Multi-line content
  - Special characters and Unicode
  - Empty and null-like values
  - Very long field content
  - Various data type challenges

## 🔧 Testing Scenarios

### Basic Import Testing
```bash
# Test basic CSV import (when implemented)
./pgtransfer import csv test-docker public.users scripts/test-data/users.csv
./pgtransfer import csv test-docker inventory.products scripts/test-data/products.csv
```

### Export Testing
```bash
# Test CSV export (when implemented)
./pgtransfer export csv test-docker public.users users_export.csv
./pgtransfer export csv test-docker sales.customers customers_export.csv
```

### Dump Testing
```bash
# Test database dump export (when implemented)
./pgtransfer export dump test-docker testdb_backup.sql
./pgtransfer export dump test-docker --schema-only testdb_schema.sql
```

### SSH Tunnel Testing
```bash
# Create profile with SSH tunnel
./pgtransfer profile add test-ssh \
  --host localhost \
  --port 5432 \
  --user testuser \
  --password testpass \
  --database testdb \
  --ssh-host localhost \
  --ssh-port 2222 \
  --ssh-user sshuser \
  --ssh-password sshpass
```

## 🔐 SSH Testing Setup

### Password Authentication
The SSH server is configured with:
- Username: `sshuser`
- Password: `sshpass`
- Port: `2222`

### Key-based Authentication
1. Generate a test key pair:
```bash
ssh-keygen -t rsa -b 2048 -f ~/.ssh/pgtransfer_test_key -N ""
```

2. Add the public key to `scripts/ssh/authorized_keys`:
```bash
cat ~/.ssh/pgtransfer_test_key.pub >> scripts/ssh/authorized_keys
```

3. Restart the SSH container:
```bash
docker-compose restart ssh-server
```

4. Test with key authentication:
```bash
./pgtransfer profile add test-ssh-key \
  --host localhost \
  --port 5432 \
  --user testuser \
  --password testpass \
  --database testdb \
  --ssh-host localhost \
  --ssh-port 2222 \
  --ssh-user sshuser \
  --ssh-key ~/.ssh/pgtransfer_test_key
```

## 🧪 Advanced Testing

### Performance Testing
The database includes enough data to test performance with:
- Complex joins across schemas
- Large result sets
- Index performance
- Transaction handling

### Error Handling Testing
Use the complex-data.csv file to test:
- Invalid data type conversions
- Constraint violations
- Character encoding issues
- Field length limits

### Connection Testing
Test various connection scenarios:
- Direct connections
- SSH tunnel connections
- SSL/TLS connections
- Connection timeouts
- Authentication failures

## 🛠️ Maintenance Commands

### Reset Database
```bash
# Stop and remove containers
docker-compose down -v

# Start fresh
docker-compose up -d
```

### View Logs
```bash
# View all logs
docker-compose logs

# View specific service logs
docker-compose logs postgres
docker-compose logs pgadmin
docker-compose logs ssh-server
```

### Database Access
```bash
# Connect directly to PostgreSQL
docker exec -it pgtransfer-test-db psql -U testuser -d testdb

# Run SQL commands
docker exec -it pgtransfer-test-db psql -U testuser -d testdb -c "SELECT COUNT(*) FROM public.users;"
```

## 🔍 Troubleshooting

### Common Issues

1. **Port conflicts**: If ports 5432, 8080, or 2222 are in use, modify the ports in `docker-compose.yml`

2. **Permission issues**: Ensure Docker has proper permissions and the SSH authorized_keys file is readable

3. **Connection refused**: Wait for services to fully start (check with `docker-compose ps`)

4. **SSH key issues**: Verify key permissions and format in the authorized_keys file

### Health Checks
```bash
# Check PostgreSQL health
docker exec pgtransfer-test-db pg_isready -U testuser -d testdb

# Check SSH server
ssh -p 2222 sshuser@localhost -o ConnectTimeout=5 echo "SSH OK"

# Check pgAdmin
curl -f http://localhost:8080 || echo "pgAdmin not ready"
```

## 📝 Notes

- This environment is designed for **testing only** - do not use in production
- Default passwords are intentionally simple for testing convenience
- The SSH server configuration prioritizes functionality over security
- All data is ephemeral unless volumes are explicitly preserved
- Services are configured to restart automatically unless stopped

## 🤝 Contributing

When adding new test scenarios:

1. Add relevant CSV files to `test-data/`
2. Update database schema in `init.sql` if needed
3. Add sample data in `sample-data.sql`
4. Document the new scenarios in this README
5. Test with both successful and error cases

---

**Happy Testing!** 🎉
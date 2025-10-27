package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/utils"
	"github.com/lib/pq"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// DBConnection represents a PostgreSQL connection, possibly via SSH.
type DBConnection struct {
	DB        *sql.DB
	SSHClient *ssh.Client
	Profile   config.Profile
	Mode      string // "direct" or "tunnel"
}

// Connect returns a PostgreSQL connection (direct or via SSH tunnel).
func Connect(p config.Profile) (*DBConnection, error) {
	if p.SSH.Enabled {
		return connectViaSSH(p)
	}
	return connectDirect(p)
}

// -----------------------------
// Direct Connection
// -----------------------------

func connectDirect(p config.Profile) (*DBConnection, error) {
	utils.PrintInfo(nil, "Connecting directly to %s:%d...", p.Host, p.Port)

	dsn := config.BuildDSN(p)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open DB: %w", err)
	}

	configureDBPool(db)

	if err := pingDatabase(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	utils.PrintSuccess(nil, "✅ Connected to PostgreSQL (direct)")
	return &DBConnection{DB: db, Profile: p, Mode: "direct"}, nil
}

// -----------------------------
// SSH Tunnel Connection
// -----------------------------

func connectViaSSH(p config.Profile) (*DBConnection, error) {
	if p.SSH.Host == "" || p.SSH.User == "" {
		return nil, errors.New("SSH host and user are required")
	}

	utils.PrintInfo(nil, "🔐 Connecting via SSH tunnel to %s@%s...", p.SSH.User, p.SSH.Host)

	authMethods, err := sshAuth(p.SSH)
	if err != nil {
		return nil, fmt.Errorf("SSH auth setup failed: %w", err)
	}

	clientConfig := &ssh.ClientConfig{
		User:            p.SSH.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(p.SSH.Timeout) * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", p.SSH.Host, p.SSH.Port)
	sshClient, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed SSH connection: %w", err)
	}

	dsn := config.BuildDSN(p)
	connector, err := newConnectorWithDialer(dsn, &sshDialer{client: sshClient})
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("failed to create connector: %w", err)
	}

	db := sql.OpenDB(connector)
	configureDBPool(db)

	if err := pingDatabase(db); err != nil {
		db.Close()
		sshClient.Close()
		return nil, fmt.Errorf("DB ping through tunnel failed: %w", err)
	}

	utils.PrintSuccess(nil, "✅ Connected to PostgreSQL via SSH tunnel")
	return &DBConnection{DB: db, SSHClient: sshClient, Profile: p, Mode: "tunnel"}, nil
}

// -----------------------------
// Helpers
// -----------------------------

func configureDBPool(db *sql.DB) {
	db.SetConnMaxIdleTime(2 * time.Minute)
	db.SetConnMaxLifetime(10 * time.Minute)
	db.SetMaxIdleConns(2)
	db.SetMaxOpenConns(5)
}

func pingDatabase(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}

func sshAuth(sshCfg config.SSHConfig) ([]ssh.AuthMethod, error) {
	var auths []ssh.AuthMethod

	// Try ssh-agent first if SSH_AUTH_SOCK is available
	if agentConn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		agentClient := agent.NewClient(agentConn)
		auths = append(auths, ssh.PublicKeysCallback(agentClient.Signers))
	}

	// If a specific key path is provided, try to use it
	if sshCfg.KeyPath != "" {
		key, err := os.ReadFile(os.ExpandEnv(sshCfg.KeyPath))
		if err != nil {
			return nil, fmt.Errorf("failed to read private key: %w", err)
		}
		
		var signer ssh.Signer
		if sshCfg.Passphrase != "" {
			// Try parsing with passphrase first
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(sshCfg.Passphrase))
		} else {
			// Try parsing without passphrase
			signer, err = ssh.ParsePrivateKey(key)
		}
		
		if err != nil {
			return nil, fmt.Errorf("invalid private key: %w", err)
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}

	// Add password authentication if provided
	if sshCfg.Password != "" {
		auths = append(auths, ssh.Password(sshCfg.Password))
	}

	if len(auths) == 0 {
		return nil, errors.New("no valid SSH authentication method found")
	}

	return auths, nil
}

// sshDialer implements pq.Dialer using an active SSH client.
type sshDialer struct {
	client *ssh.Client
}

func (d *sshDialer) Dial(network, address string) (net.Conn, error) {
	return d.client.Dial(network, address)
}

func (d *sshDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	type result struct {
		conn net.Conn
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		c, err := d.client.Dial(network, address)
		ch <- result{c, err}
	}()

	select {
	case res := <-ch:
		return res.conn, res.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("dial timeout after %s", timeout)
	}
}

// -----------------------------
// Custom Connector (safe)
// -----------------------------

// newConnectorWithDialer creates a pq-compatible connector using a custom SSH dialer.
func newConnectorWithDialer(dsn string, dialer pq.Dialer) (driver.Connector, error) {
	return &sshConnector{dsn: dsn, dialer: dialer}, nil
}

// sshConnector implements driver.Connector to integrate SSH with pq.
type sshConnector struct {
	dsn    string
	dialer pq.Dialer
}

// Connect implements driver.Connector.
func (c *sshConnector) Connect(ctx context.Context) (driver.Conn, error) {
	return pq.DialOpen(c.dialer, c.dsn)
}

// Driver implements driver.Connector.
func (c *sshConnector) Driver() driver.Driver {
	return &pq.Driver{}
}

// -----------------------------
// Test Connection Wrapper
// -----------------------------

func TestConnection(p config.Profile) error {
	start := time.Now()
	utils.PrintInfo(nil, "Testing connection for profile '%s'...", p.Name)

	conn, err := Connect(p)
	if err != nil {
		utils.PrintError(nil, "❌ Connection failed: %v", err)
		return err
	}
	defer conn.Close()

	utils.PrintSuccess(nil, "✅ Connection verified (%s mode)", conn.Mode)
	utils.PrintInfo(nil, "⏱ Duration: %s", utils.FormatDuration(time.Since(start)))
	return nil
}

// -----------------------------
// Cleanup
// -----------------------------

func (c *DBConnection) Close() {
	if c.DB != nil {
		_ = c.DB.Close()
	}
	if c.SSHClient != nil {
		_ = c.SSHClient.Close()
	}
}

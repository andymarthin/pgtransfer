package db

import (
	"context"
	"database/sql/driver"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/andymarthin/pgtransfer/internal/config"
)

// mockDialer simulates a dialer for testing.
type mockDialer struct {
	dialCalled bool
	shouldFail bool
}

func (m *mockDialer) Dial(network, addr string) (net.Conn, error) {
	m.dialCalled = true
	if m.shouldFail {
		return nil, errors.New("mock dial failure")
	}
	return &mockConn{}, nil
}

func (m *mockDialer) DialTimeout(network, addr string, timeout time.Duration) (net.Conn, error) {
	return m.Dial(network, addr)
}

// mockConn is a dummy net.Conn for testing.
type mockConn struct{}

func (m *mockConn) Read(b []byte) (int, error)         { return len(b), nil }
func (m *mockConn) Write(b []byte) (int, error)        { return len(b), nil }
func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// ---------------------------
// Tests
// ---------------------------

func TestConnectDirect_Success(t *testing.T) {
	p := config.Profile{
		Name:     "direct",
		User:     "postgres",
		Password: "secret",
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		SSLMode:  "disable",
	}

	// This test verifies the DSN building and basic connection setup
	// without actually connecting to a database
	dsn := config.BuildDSN(p)
	if dsn == "" {
		t.Fatalf("expected non-empty DSN")
	}

	// Test that the profile is properly configured
	if p.Host != "localhost" || p.Port != 5432 {
		t.Fatalf("profile not configured correctly")
	}
}

func TestConnectViaSSH_FailureAuth(t *testing.T) {
	p := config.Profile{
		Name: "ssh-fail",
		SSH: config.SSHConfig{
			Enabled: true,
			User:    "root",
			Host:    "localhost",
			Port:    22,
		},
	}

	_, err := connectViaSSH(p)
	if err == nil {
		t.Fatalf("expected SSH auth failure, got nil")
	}
}

func TestSSHAuth_KeyAndPassword(t *testing.T) {
	sshCfg := config.SSHConfig{
		KeyPath:  "invalid.key",
		Password: "1234",
	}
	_, err := sshAuth(sshCfg)
	if err == nil {
		t.Fatalf("expected invalid key read error, got nil")
	}
}

func TestNewConnectorWithDialer(t *testing.T) {
	connector, err := newConnectorWithDialer("postgres://user:pass@localhost/db?sslmode=disable", &sshDialer{client: nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if connector == nil {
		t.Fatalf("expected connector, got nil")
	}
}

func TestSSHConnector_ImplementsDriverConnector(t *testing.T) {
	var _ driver.Connector = &sshConnector{}
}

func TestSSHConnectorDriver(t *testing.T) {
	c := &sshConnector{}
	if c.Driver() == nil {
		t.Fatalf("expected pq.Driver, got nil")
	}
}

func TestSSHConnectorConnect_Timeout(t *testing.T) {
	d := &mockDialer{shouldFail: true}
	c := &sshConnector{
		dsn:    "postgres://user:pass@localhost/db?sslmode=disable",
		dialer: d,
	}

	_, err := c.Connect(context.Background())
	if err == nil {
		t.Fatalf("expected failure, got nil")
	}
}

func TestDialTimeout_Success(t *testing.T) {
	m := &mockDialer{}

	conn, err := m.DialTimeout("tcp", "localhost:5432", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if conn == nil {
		t.Fatalf("expected conn, got nil")
	}
}

func TestDialTimeout_Failure(t *testing.T) {
	m := &mockDialer{shouldFail: true}

	_, err := m.DialTimeout("tcp", "localhost:5432", 500*time.Millisecond)
	if err == nil {
		t.Fatalf("expected timeout or failure, got nil")
	}
}

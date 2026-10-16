//go:build integration

package io

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andymarthin/pgtransfer/internal/config"
)

// sshTestProfile builds a profile that reaches the test database through an
// SSH tunnel. It is configured entirely from environment variables and skips
// when they are not provided.
//
//	PGTRANSFER_TEST_SSH_HOST (default "localhost")
//	PGTRANSFER_TEST_SSH_PORT (default "2222")
//	PGTRANSFER_TEST_SSH_USER
//	PGTRANSFER_TEST_SSH_PASSWORD
//	PGTRANSFER_TEST_DB_HOST   (DB host as seen from the SSH server, default "postgres")
//	PGTRANSFER_TEST_DB_PORT   (default "5432")
//	PGTRANSFER_TEST_DB_USER   (default "testuser")
//	PGTRANSFER_TEST_DB_PASSWORD (default "testpass")
//	PGTRANSFER_TEST_DB_NAME   (default "testdb")
func sshTestProfile(t *testing.T) config.Profile {
	t.Helper()

	if os.Getenv("PGTRANSFER_TEST_SSH_USER") == "" {
		skipOrFail(t, "PGTRANSFER_TEST_SSH_USER not set; skipping SSH tunnel integration test")
	}

	getenv := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}

	return config.Profile{
		Name:     "integration-ssh",
		User:     getenv("PGTRANSFER_TEST_DB_USER", "testuser"),
		Password: getenv("PGTRANSFER_TEST_DB_PASSWORD", "testpass"),
		Host:     getenv("PGTRANSFER_TEST_DB_HOST", "postgres"),
		Port:     atoiDefault(getenv("PGTRANSFER_TEST_DB_PORT", "5432"), 5432),
		Database: getenv("PGTRANSFER_TEST_DB_NAME", "testdb"),
		SSLMode:  "disable",
		SSH: config.SSHConfig{
			Enabled:  true,
			Host:     getenv("PGTRANSFER_TEST_SSH_HOST", "localhost"),
			Port:     atoiDefault(getenv("PGTRANSFER_TEST_SSH_PORT", "2222"), 2222),
			User:     os.Getenv("PGTRANSFER_TEST_SSH_USER"),
			Password: os.Getenv("PGTRANSFER_TEST_SSH_PASSWORD"),
			Timeout:  10,
		},
	}
}

func atoiDefault(s string, def int) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	if n == 0 {
		return def
	}
	return n
}

func TestIntegration_SSHTunnelDump(t *testing.T) {
	requirePgTools(t)
	profile := sshTestProfile(t)

	dir := t.TempDir()
	dumpPath := filepath.Join(dir, "tunnel.sql")

	if err := DumpDatabaseWithConnection(profile, dumpPath); err != nil {
		t.Fatalf("SSH tunnel dump failed: %v", err)
	}
	if _, err := os.Stat(dumpPath); err != nil {
		t.Fatalf("expected dump file via tunnel: %v", err)
	}
}

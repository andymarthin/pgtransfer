//go:build integration

package io

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andymarthin/pgtransfer/internal/config"
)

// newScratchDB creates a brand-new database on the same server as the
// integration DSN and returns a DSN pointing at it. The database is dropped on
// cleanup.
func newScratchDB(t *testing.T, baseDSN, name string) string {
	t.Helper()

	admin, err := sql.Open("postgres", baseDSN)
	if err != nil {
		t.Fatalf("failed to open admin connection: %v", err)
	}
	defer admin.Close()

	if _, err := admin.Exec("DROP DATABASE IF EXISTS " + name); err != nil {
		t.Fatalf("drop database failed: %v", err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("create database failed: %v", err)
	}
	t.Cleanup(func() {
		a, err := sql.Open("postgres", baseDSN)
		if err != nil {
			return
		}
		defer a.Close()
		_, _ = a.Exec("DROP DATABASE IF EXISTS " + name)
	})

	// Swap the database name in the DSN's path component.
	return replaceDSNDatabase(baseDSN, name)
}

// replaceDSNDatabase rewrites the database name in a postgres:// URL.
func replaceDSNDatabase(dsn, dbName string) string {
	// DSN form: postgres://user:pass@host:port/dbname?params
	at := strings.Index(dsn, "://")
	rest := dsn[at+3:]
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return dsn + "/" + dbName
	}
	prefix := dsn[:at+3+slash+1]
	tail := rest[slash+1:]
	if q := strings.Index(tail, "?"); q >= 0 {
		return prefix + dbName + tail[q:]
	}
	return prefix + dbName
}

func TestIntegration_DumpPlainAndRestore(t *testing.T) {
	requirePgTools(t)
	baseDSN := testDSN(t)
	db := testDB(t)

	// Seed a source table.
	if _, err := db.Exec("DROP TABLE IF EXISTS it_dump_src"); err != nil {
		t.Fatalf("drop failed: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE it_dump_src (id int PRIMARY KEY, v text)"); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := db.Exec("INSERT INTO it_dump_src VALUES (1,'a'),(2,'b')"); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec("DROP TABLE IF EXISTS it_dump_src") })

	dir := t.TempDir()
	dumpPath := filepath.Join(dir, "plain.sql")

	// Dump (plain).
	if err := DumpDatabase(baseDSN, dumpPath); err != nil {
		t.Fatalf("plain dump failed: %v", err)
	}
	if _, err := os.Stat(dumpPath); err != nil {
		t.Fatalf("expected dump file: %v", err)
	}

	// The dump must be detected as plain text.
	isText, err := isPlainTextDump(dumpPath)
	if err != nil {
		t.Fatalf("detection failed: %v", err)
	}
	if !isText {
		t.Fatalf("expected plain-text dump detection")
	}

	// Restore into a fresh database.
	targetDSN := newScratchDB(t, baseDSN, "it_restore_plain")
	if err := RestoreDatabaseSmartWithOptions(targetDSN, dumpPath, &RestoreOptions{}); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	target, err := sql.Open("postgres", targetDSN)
	if err != nil {
		t.Fatalf("open target failed: %v", err)
	}
	defer target.Close()
	var n int
	if err := target.QueryRow("SELECT COUNT(*) FROM it_dump_src").Scan(&n); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 restored rows, got %d", n)
	}
}

func TestIntegration_DumpCustomFormatDetectedAndRestored(t *testing.T) {
	requirePgTools(t)
	baseDSN := testDSN(t)
	db := testDB(t)

	if _, err := db.Exec("DROP TABLE IF EXISTS it_dump_custom"); err != nil {
		t.Fatalf("drop failed: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE it_dump_custom (id int PRIMARY KEY, v text)"); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if _, err := db.Exec("INSERT INTO it_dump_custom VALUES (1,'x')"); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec("DROP TABLE IF EXISTS it_dump_custom") })

	dir := t.TempDir()
	dumpPath := filepath.Join(dir, "custom.dump")

	if err := DumpDatabaseWithOptions(baseDSN, dumpPath, &DumpOptions{Format: "custom", Compress: true}); err != nil {
		t.Fatalf("custom dump failed: %v", err)
	}

	// Must be detected as binary (not plain text).
	isText, err := isPlainTextDump(dumpPath)
	if err != nil {
		t.Fatalf("detection failed: %v", err)
	}
	if isText {
		t.Fatalf("custom-format dump was misdetected as plain text")
	}

	targetDSN := newScratchDB(t, baseDSN, "it_restore_custom")
	if err := RestoreDatabaseSmartWithOptions(targetDSN, dumpPath, &RestoreOptions{}); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	target, err := sql.Open("postgres", targetDSN)
	if err != nil {
		t.Fatalf("open target failed: %v", err)
	}
	defer target.Close()
	var n int
	if err := target.QueryRow("SELECT COUNT(*) FROM it_dump_custom").Scan(&n); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 restored row, got %d", n)
	}
}

func TestIntegration_DumpWithConnection(t *testing.T) {
	requirePgTools(t)
	baseDSN := testDSN(t)

	profile := config.Profile{Name: "it", DBURL: baseDSN}
	dir := t.TempDir()
	dumpPath := filepath.Join(dir, "conn.sql")

	if err := DumpDatabaseWithConnection(profile, dumpPath); err != nil {
		t.Fatalf("connection-aware dump failed: %v", err)
	}
	if _, err := os.Stat(dumpPath); err != nil {
		t.Fatalf("expected dump file: %v", err)
	}
}

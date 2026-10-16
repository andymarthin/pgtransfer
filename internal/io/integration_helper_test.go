//go:build integration

// Package io integration tests exercise the real pg_dump/pg_restore/psql and
// PostgreSQL round-trips against a live database.
//
// They are gated behind the `integration` build tag and only run when a test
// database is reachable. Provide a DSN via the PGTRANSFER_TEST_DSN environment
// variable, e.g.:
//
//	PGTRANSFER_TEST_DSN='postgres://testuser:testpass@localhost:5433/testdb?sslmode=disable' \
//	  go test -tags=integration ./internal/io/...
//
// The simplest way to get such a database is the repository test stack:
//
//	cd scripts && docker compose up -d
//
// Set PGTRANSFER_TEST_REQUIRE=1 (CI does) to turn the "missing prerequisite"
// skips below into hard failures, so a misconfigured pipeline cannot pass
// green while running no integration tests at all.
package io

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// required reports whether missing prerequisites should fail rather than skip.
func required() bool {
	v := os.Getenv("PGTRANSFER_TEST_REQUIRE")
	return v != "" && v != "0" && v != "false"
}

// skipOrFail skips the test when prerequisites are optional, or fails when
// PGTRANSFER_TEST_REQUIRE is set.
func skipOrFail(t *testing.T, format string, args ...interface{}) {
	t.Helper()
	if required() {
		t.Fatalf(format, args...)
	}
	t.Skipf(format, args...)
}

// testDSN returns the configured integration-test DSN or skips the test.
func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("PGTRANSFER_TEST_DSN")
	if dsn == "" {
		skipOrFail(t, "PGTRANSFER_TEST_DSN not set; skipping integration test")
	}
	return dsn
}

// testDB opens a *sql.DB for the integration-test database or skips.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := testDSN(t)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		skipOrFail(t, "cannot open test database: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		skipOrFail(t, "test database not reachable: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// requirePgTools skips the test unless the PostgreSQL client tools are present.
func requirePgTools(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"pg_dump", "pg_restore", "psql"} {
		if !commandExists(tool) {
			skipOrFail(t, "%s not found in PATH; skipping integration test", tool)
		}
	}
}

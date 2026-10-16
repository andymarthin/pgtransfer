//go:build integration

package io

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// setupCSVTable creates a dedicated table and seeds it with deterministic rows.
func setupCSVTable(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	stmts := []string{
		"DROP TABLE IF EXISTS " + name,
		"CREATE TABLE " + name + " (id int PRIMARY KEY, name text, active bool, score numeric)",
		"INSERT INTO " + name + " (id, name, active, score) VALUES " +
			"(1, 'alpha', true, 1.5), (2, 'beta', false, 2.25), (3, 'gamma', true, 3.0)",
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("setup failed (%s): %v", s, err)
		}
	}
	t.Cleanup(func() { _, _ = db.Exec("DROP TABLE IF EXISTS " + name) })
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count failed: %v", err)
	}
	return n
}

func TestIntegration_CSVExportImportRoundTrip(t *testing.T) {
	db := testDB(t)
	table := "it_csv_roundtrip"
	setupCSVTable(t, db, table)

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "out.csv")

	// Export (with headers).
	if err := ExportCSVWithOptions(db, table, csvPath, &CSVOptions{BatchSize: 500, HasHeader: true}); err != nil {
		t.Fatalf("export failed: %v", err)
	}
	if _, err := os.Stat(csvPath); err != nil {
		t.Fatalf("expected exported CSV: %v", err)
	}

	// Verify the header row is present.
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("exported CSV is empty")
	}

	// Truncate then re-import with headers.
	if _, err := db.Exec("TRUNCATE " + table); err != nil {
		t.Fatalf("truncate failed: %v", err)
	}
	if got := countRows(t, db, table); got != 0 {
		t.Fatalf("expected 0 rows after truncate, got %d", got)
	}

	if err := ImportCSVWithOptions(db, table, csvPath, &CSVOptions{BatchSize: 500, HasHeader: true}); err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if got := countRows(t, db, table); got != 3 {
		t.Fatalf("expected 3 rows after round-trip, got %d", got)
	}
}

func TestIntegration_CSVImportHeaderless(t *testing.T) {
	db := testDB(t)
	table := "it_csv_headerless"
	if _, err := db.Exec("DROP TABLE IF EXISTS " + table); err != nil {
		t.Fatalf("drop failed: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE " + table + " (id int, name text, active bool)"); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec("DROP TABLE IF EXISTS " + table) })

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "noheader.csv")
	// No header row — every line is data.
	if err := os.WriteFile(csvPath, []byte("10,delta,true\n11,epsilon,false\n"), 0o600); err != nil {
		t.Fatalf("write fixture failed: %v", err)
	}

	if err := ImportCSVWithOptions(db, table, csvPath, &CSVOptions{BatchSize: 500, HasHeader: false}); err != nil {
		t.Fatalf("headerless import failed: %v", err)
	}
	if got := countRows(t, db, table); got != 2 {
		t.Fatalf("expected 2 rows, got %d", got)
	}
}

//go:build integration

package io

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegration_ExportRefusesEarlyEmptyBatch pins that an empty table exports
// cleanly. The batch loop used to decide "short batch means end of table", which
// is fine, but it also could not distinguish a driver error from exhaustion;
// both now take different paths and this checks the plain case still works.
func TestIntegration_ExportRefusesEarlyEmptyBatch(t *testing.T) {
	db := testDB(t)

	table := "it_export_empty"
	mustExec(t, db, "DROP TABLE IF EXISTS "+table)
	mustExec(t, db, "CREATE TABLE "+table+" (id bigint PRIMARY KEY, name text)")

	path := filepath.Join(t.TempDir(), "empty.csv")
	if err := ExportCSVWithOptions(db, table, path, &CSVOptions{BatchSize: 100, HasHeader: true}); err != nil {
		t.Fatalf("ExportCSVWithOptions on an empty table: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	// Header only, no data rows.
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("empty table exported %d lines, want 1 header line: %q", len(lines), data)
	}
	if lines[0] != "id,name" {
		t.Errorf("header = %q, want %q", lines[0], "id,name")
	}

	mustExec(t, db, "DROP TABLE "+table)
}

// TestIntegration_ExportExactMultipleOfBatchSize pins the batch boundary. A
// table whose row count is an exact multiple of the batch size needs one extra
// query to discover it is done; the loop must terminate on the empty batch and
// not repeat or drop rows.
func TestIntegration_ExportExactMultipleOfBatchSize(t *testing.T) {
	db := testDB(t)

	table := "it_export_multiple"
	mustExec(t, db, "DROP TABLE IF EXISTS "+table)
	mustExec(t, db, "CREATE TABLE "+table+" (id bigint PRIMARY KEY, name text)")
	for i := 1; i <= 10; i++ {
		mustExec(t, db, "INSERT INTO "+table+" (id, name) VALUES ($1, $2)", i, "row")
	}

	for _, batchSize := range []int{1, 2, 5, 10, 20} {
		path := filepath.Join(t.TempDir(), "out.csv")
		if err := ExportCSVWithOptions(db, table, path, &CSVOptions{BatchSize: batchSize, HasHeader: false}); err != nil {
			t.Fatalf("batch size %d: %v", batchSize, err)
		}
		if got := CountCSVDataRows(path, false); got != 10 {
			t.Errorf("batch size %d exported %d rows, want 10", batchSize, got)
		}
	}

	mustExec(t, db, "DROP TABLE "+table)
}

// TestIntegration_ImportHeaderlessMatchesExportedHeaderless is the round trip
// that the default flags must support: export without headers, import without
// headers, and get the same data back.
func TestIntegration_ImportHeaderlessMatchesExportedHeaderless(t *testing.T) {
	db := testDB(t)

	src := "it_rt_src"
	dst := "it_rt_dst"
	mustExec(t, db, "DROP TABLE IF EXISTS "+src)
	mustExec(t, db, "DROP TABLE IF EXISTS "+dst)

	mustExec(t, db, "CREATE TABLE "+src+" (id bigint PRIMARY KEY, name text, note text)")
	mustExec(t, db, "CREATE TABLE "+dst+" (id bigint PRIMARY KEY, name text, note text)")

	mustExec(t, db, "INSERT INTO "+src+" VALUES (1, 'a', NULL), (2, 'b', 'text'), (3, NULL, 'x')")

	path := filepath.Join(t.TempDir(), "data.csv")
	if err := ExportCSVWithOptions(db, src, path, &CSVOptions{BatchSize: 2, HasHeader: false}); err != nil {
		t.Fatalf("export: %v", err)
	}

	if err := ImportCSVWithOptions(db, dst, path, &CSVOptions{BatchSize: 2, HasHeader: false}); err != nil {
		t.Fatalf("import: %v", err)
	}

	var srcCount, dstCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + src).Scan(&srcCount); err != nil {
		t.Fatalf("count src: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM " + dst).Scan(&dstCount); err != nil {
		t.Fatalf("count dst: %v", err)
	}
	if srcCount != dstCount {
		t.Fatalf("row counts differ: src %d, dst %d", srcCount, dstCount)
	}

	var differing int
	q := "SELECT COUNT(*) FROM (SELECT * FROM " + src + " EXCEPT SELECT * FROM " + dst + ") d"
	if err := db.QueryRow(q).Scan(&differing); err != nil {
		t.Fatalf("compare: %v", err)
	}
	if differing != 0 {
		t.Errorf("%d rows differ after round trip", differing)
	}

	mustExec(t, db, "DROP TABLE "+src)
	mustExec(t, db, "DROP TABLE "+dst)
}

// TestIntegration_ImportCSVEmptyFieldIsNull pins that import inverts what
// export writes: an empty field is SQL NULL, not an empty string.
func TestIntegration_ImportCSVEmptyFieldIsNull(t *testing.T) {
	db := testDB(t)

	table := "it_null_rt"
	mustExec(t, db, "DROP TABLE IF EXISTS "+table)
	mustExec(t, db, "CREATE TABLE "+table+" (id bigint PRIMARY KEY, val text)")

	path := filepath.Join(t.TempDir(), "nulls.csv")
	if err := os.WriteFile(path, []byte("id,val\n1,\n2,present\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := ImportCSVWithOptions(db, table, path, &CSVOptions{BatchSize: 10, HasHeader: true}); err != nil {
		t.Fatalf("import: %v", err)
	}

	var isNull bool
	if err := db.QueryRow("SELECT val IS NULL FROM " + table + " WHERE id = 1").Scan(&isNull); err != nil {
		t.Fatalf("query: %v", err)
	}
	if !isNull {
		t.Error("empty CSV field was imported as a value, not NULL")
	}

	mustExec(t, db, "DROP TABLE "+table)
}

// TestIntegration_DumpRemovesPartialFileOnFailure pins that a failed dump does
// not leave a truncated file behind. The dump points at a database that does
// not exist, so pg_dump fails after creating the output file.
func TestIntegration_DumpRemovesPartialFileOnFailure(t *testing.T) {
	dsn := testDSN(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.sql")

	// Rewrite the DSN to a database that does not exist, keeping credentials.
	badDSN := replaceDatabase(dsn, "pgtransfer_no_such_db")

	err := DumpDatabase(badDSN, path)
	if err == nil {
		t.Fatalf("DumpDatabase() succeeded against a missing database")
	}
	if !strings.Contains(err.Error(), "removed") {
		t.Logf("dump error: %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("partial dump left on disk at %s", path)
	}
}

// replaceDatabase swaps the database name in a connection URL.
func replaceDatabase(dsn, name string) string {
	at := strings.LastIndex(dsn, "@")
	if at < 0 {
		return dsn
	}
	rest := dsn[at+1:]
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return dsn
	}
	tail := rest[slash+1:]
	query := ""
	if q := strings.Index(tail, "?"); q >= 0 {
		query = tail[q:]
	}
	return dsn[:at+1] + rest[:slash+1] + name + query
}

// mustExec runs a statement and fails the test on error.
func mustExec(t *testing.T, db *sql.DB, query string, args ...interface{}) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

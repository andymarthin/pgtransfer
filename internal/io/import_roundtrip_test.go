package io

import (
	"database/sql"
	"encoding/csv"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

// openTestDB opens the integration database, or skips the test.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PGTRANSFER_TEST_DSN")
	if dsn == "" {
		t.Skip("PGTRANSFER_TEST_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return db
}

// createTable creates a table and drops it when the test ends.
func createTable(t *testing.T, db *sql.DB, name, ddl string) {
	t.Helper()
	dropTable(t, db, name)
	if _, err := db.Exec("CREATE TABLE " + name + " " + ddl); err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	t.Cleanup(func() { dropTable(t, db, name) })
}

func dropTable(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	if _, err := db.Exec("DROP TABLE IF EXISTS " + name); err != nil {
		t.Errorf("drop %s: %v", name, err)
	}
}

// TestImportCSVRoundTripsThroughCopy is the guard on the COPY import path.
//
// COPY is what makes an import take seconds instead of minutes, and it encodes
// each value in PostgreSQL's text syntax. That encoding has to survive the
// values a real export contains: a tab, a newline, a backslash, a quote, a
// comma, a leading space, a NULL, and text outside ASCII. Any of them being
// mangled silently changes the data, so the test compares what the server
// holds after the import against what it held before the export.
func TestImportCSVRoundTripsThroughCopy(t *testing.T) {
	db := openTestDB(t)

	const src = "pgtransfer_copy_roundtrip_src"
	const dst = "pgtransfer_copy_roundtrip_dst"
	ddl := `(id bigint primary key, txt text, num numeric(12,2), ts timestamptz, flag boolean, note text)`
	createTable(t, db, src, ddl)
	createTable(t, db, dst, ddl)

	rows := [][]string{
		{"1", "plain", "1.00", "2024-01-01 00:00:00+00", "true", ""},
		{"2", "tab\there", "2.50", "2024-02-02 03:04:05+00", "false", "line\nbreak"},
		{"3", `back\slash and "quote"`, "3.75", "2024-03-03 00:00:00+00", "true", "cr\r\nlf"},
		{"4", "comma, inside", "4.00", "2024-04-04 00:00:00+00", "false", " leading space"},
		{"5", "ünïcödé ✓ 🎉", "-5.25", "2024-05-05 00:00:00+00", "true", ""},
	}

	// Load the source through COPY so the source holds the values exactly.
	if _, err := db.Exec("ALTER TABLE " + src + " ALTER COLUMN note DROP NOT NULL"); err != nil {
		t.Fatalf("allow null note: %v", err)
	}
	for _, r := range rows {
		var note interface{}
		if r[5] != "" {
			note = r[5]
		}
		if _, err := db.Exec(
			"INSERT INTO "+src+" (id, txt, num, ts, flag, note) VALUES ($1,$2,$3,$4,$5,$6)",
			r[0], r[1], r[2], r[3], r[4], note,
		); err != nil {
			t.Fatalf("insert %v: %v", r, err)
		}
	}

	dir := t.TempDir()
	exportPath := dir + "/roundtrip.csv"
	if err := ExportCSVWithOptions(db, src, exportPath, &CSVOptions{BatchSize: 2, HasHeader: true}); err != nil {
		t.Fatalf("export: %v", err)
	}

	if !copySupported(db) {
		t.Fatal("copySupported() = false against PostgreSQL, so the COPY path was not exercised")
	}

	// ImportCSVWithOptions takes its own *sql.DB; reuse it.
	if err := ImportCSVWithOptions(db, dst, exportPath, &CSVOptions{BatchSize: 2, HasHeader: true}); err != nil {
		t.Fatalf("import: %v", err)
	}

	// The CSV that was imported must have used COPY, not the INSERT fallback:
	// a header that matches the table is what selects it.
	cols, err := copyColumnsForTable(db, dst)
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	header := exportHeader(t, exportPath)
	if !headerMatchesColumns(header, cols) {
		t.Fatalf("header %v does not match %v, so the import fell back to INSERT", header, cols)
	}

	want := digestOf(t, db, src)
	got := digestOf(t, db, dst)
	if want != got {
		t.Fatalf("digest mismatch after COPY import\nsource: %q\ntarget: %q", want, got)
	}
}

// exportHeader reads the first record of a CSV file.
func exportHeader(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	rec, err := csv.NewReader(f).Read()
	if err != nil {
		t.Fatalf("read header: %v", err)
	}
	return rec
}

// digestOf renders the rows of a table as text, ordered by id, for comparison.
func digestOf(t *testing.T, db *sql.DB, table string) string {
	t.Helper()
	var out string
	err := db.QueryRow(
		"SELECT coalesce(string_agg(t::text, E'\n' ORDER BY id), '') FROM " + table + " t",
	).Scan(&out)
	if err != nil {
		t.Fatalf("digest %s: %v", table, err)
	}
	return out
}

// TestImportCSVRejectsRaggedCopyRows pins that a short row is refused rather
// than written. COPY maps values onto columns by position, so a row that is
// missing values would otherwise leave the last columns NULL and report
// success.
func TestImportCSVRejectsRaggedCopyRows(t *testing.T) {
	db := openTestDB(t)

	const tbl = "pgtransfer_copy_ragged"
	createTable(t, db, tbl, `(id bigint primary key, a text, b text)`)

	dir := t.TempDir()
	path := dir + "/ragged.csv"
	// Two columns of data for a three-column table, header included.
	body := "id,a,b\n1,only-one\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := ImportCSVWithOptions(db, tbl, path, &CSVOptions{BatchSize: 100, HasHeader: true})
	if err == nil {
		t.Fatal("import of a ragged row returned no error")
	}
	// The message has to name the mismatch: the file is what the user fixes.
	// The ragged line reaches the value-count check rather than being padded
	// out, which is the behaviour being pinned.
	msg := err.Error()
	if !strings.Contains(msg, "2 values") || !strings.Contains(msg, "3 columns") {
		t.Errorf("error = %v, want it to name the value and column counts", err)
	}

	var n int
	if err := db.QueryRow("SELECT count(*) FROM " + tbl).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("%d rows were written by a failed import, want 0", n)
	}
}

// TestImportCSVPreservesNulls pins that an empty CSV field becomes SQL NULL,
// not an empty string. The two are different values, and an export/import
// round trip that turns one into the other changes the data.
func TestImportCSVPreservesNulls(t *testing.T) {
	db := openTestDB(t)

	const tbl = "pgtransfer_copy_nulls"
	createTable(t, db, tbl, `(id bigint primary key, note text)`)

	dir := t.TempDir()
	path := dir + "/nulls.csv"
	if err := os.WriteFile(path, []byte("id,note\n1,\n2,filled\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := ImportCSVWithOptions(db, tbl, path, &CSVOptions{BatchSize: 100, HasHeader: true}); err != nil {
		t.Fatalf("import: %v", err)
	}

	var isNull bool
	if err := db.QueryRow("SELECT note IS NULL FROM " + tbl + " WHERE id = 1").Scan(&isNull); err != nil {
		t.Fatalf("query: %v", err)
	}
	if !isNull {
		t.Error("an empty CSV field was imported as an empty string, want NULL")
	}
}

package io

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCSVOutputPath pins the path resolution that the overwrite guard depends
// on. Export appends the extension itself, so anything that wants to know which
// file will be written has to resolve it the same way.
func TestCSVOutputPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"users", "users.csv"},
		{"users.csv", "users.csv"},
		{"dir/users", "dir/users.csv"},
		{"dir/users.csv", "dir/users.csv"},
		{"users.CSV", "users.CSV.csv"}, // only the lowercase suffix is recognised
		{"", ".csv"},
	}
	for _, c := range cases {
		if got := CSVOutputPath(c.in); got != c.want {
			t.Errorf("CSVOutputPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestCountCSVRowsIgnoresEmbeddedNewlines pins that counting records matches
// how encoding/csv reads them. A quoted field containing a newline is one
// record, and counting newline bytes would overstate the row count.
func TestCountCSVRowsIgnoresEmbeddedNewlines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multiline.csv")

	// Three data records; the second contains a newline and a comma inside a
	// quoted field.
	content := "id,note\n1,plain\n2,\"two\nlines, with comma\"\n3,last\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	withHeader, err := countCSVRows(path, true)
	if err != nil {
		t.Fatalf("countCSVRows: %v", err)
	}
	if withHeader != 3 {
		t.Errorf("countCSVRows(hasHeader=true) = %d, want 3", withHeader)
	}

	noHeader, err := countCSVRows(path, false)
	if err != nil {
		t.Fatalf("countCSVRows: %v", err)
	}
	if noHeader != 4 {
		t.Errorf("countCSVRows(hasHeader=false) = %d, want 4", noHeader)
	}
}

// TestCountCSVDataRowsMatchesParser cross-checks the public helper against
// encoding/csv rather than against a newline count.
func TestCountCSVDataRowsMatchesParser(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rows.csv")

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"id", "text"})
	for i := 0; i < 5; i++ {
		_ = w.Write([]string{"1", "line\nwith newline"})
	}
	w.Flush()
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got := CountCSVDataRows(path, true); got != 5 {
		t.Errorf("CountCSVDataRows = %d, want 5", got)
	}
}

// TestCountCSVRowsMissingFile pins that a missing file is an error rather than
// a silent zero, which would have been read as "no data rows".
func TestCountCSVRowsMissingFile(t *testing.T) {
	if _, err := countCSVRows(filepath.Join(t.TempDir(), "absent.csv"), true); err == nil {
		t.Fatal("countCSVRows() accepted a missing file")
	}
}

// TestCountCSVRowsEmptyFile pins that an empty file has no data rows.
func TestCountCSVRowsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.csv")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	n, err := countCSVRows(path, true)
	if err != nil {
		t.Fatalf("countCSVRows: %v", err)
	}
	if n != 0 {
		t.Errorf("countCSVRows(empty) = %d, want 0", n)
	}
}

// TestRedactDSNURLRemovesPassword pins the verbose-log redaction. The DSN is
// printed as part of the pg_dump argument list, so the password must not
// survive.
func TestRedactDSNURLRemovesPassword(t *testing.T) {
	cases := []struct {
		in       string
		wantOut  string
		mustHide string
	}{
		{
			in:       "postgres://app:hunter2@localhost:5432/db?sslmode=disable",
			wantOut:  "postgres://app:***@localhost:5432/db?sslmode=disable",
			mustHide: "hunter2",
		},
		{
			in:       "postgres://app:p%40ss%3Aword@db.example.com:5432/myapp",
			mustHide: "p%40ss",
		},
		{
			// An unparseable URL still must not leak the credential.
			in:       "postgres://app:hunter2@bad host/db",
			mustHide: "hunter2",
		},
		{
			// No password: left alone.
			in:      "postgres://app@localhost:5432/db",
			wantOut: "postgres://app@localhost:5432/db",
		},
		{
			// Not a URL at all: left alone.
			in:      "plain-value",
			wantOut: "plain-value",
		},
	}
	for _, c := range cases {
		got := redactDSNURL(c.in)
		if c.mustHide != "" && strings.Contains(got, c.mustHide) {
			t.Errorf("redactDSNURL(%q) leaked %q: %s", c.in, c.mustHide, got)
		}
		if c.wantOut != "" && got != c.wantOut {
			t.Errorf("redactDSNURL(%q) = %q, want %q", c.in, got, c.wantOut)
		}
	}
}

// TestRedactArgsOnlyTouchesConnectionURLs pins that flags and file paths pass
// through unchanged.
func TestRedactArgsOnlyTouchesConnectionURLs(t *testing.T) {
	args := []string{
		"--format", "custom",
		"--file", "/tmp/out.dump",
		"postgres://app:secret@localhost:5432/db",
	}
	got := redactArgs(args)

	if strings.Join(got, " ") != "--format custom --file /tmp/out.dump postgres://app:***@localhost:5432/db" {
		t.Fatalf("redactArgs() = %v", got)
	}
	// The input must not be mutated: it is still the argument list that runs.
	if !strings.Contains(args[4], "secret") {
		t.Error("redactArgs() mutated the original slice")
	}
}

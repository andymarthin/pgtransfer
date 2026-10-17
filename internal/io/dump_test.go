package io

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsPlainTextDump(t *testing.T) {
	dir := t.TempDir()

	writeFile := func(name string, content []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("failed to write fixture %s: %v", name, err)
		}
		return path
	}

	tests := []struct {
		name     string
		content  []byte
		wantText bool
	}{
		{
			name:     "plain sql with pg_dump header",
			content:  []byte("--\n-- PostgreSQL database dump\n--\nSET statement_timeout = 0;\nCREATE TABLE foo (id int);\n"),
			wantText: true,
		},
		{
			name:     "plain sql with CREATE TABLE",
			content:  []byte("CREATE TABLE foo (id integer);\nINSERT INTO foo VALUES (1);\n"),
			wantText: true,
		},
		{
			name:     "plain sql with COPY",
			content:  []byte("COPY public.users (id) FROM stdin;\n1\n\\.\n"),
			wantText: true,
		},
		{
			name:     "custom format archive",
			content:  []byte("PGDMP\x01\x0e\x00binarygarbagehere"),
			wantText: false,
		},
		{
			name:     "tar archive magic",
			content:  append(make([]byte, 257), []byte("ustar\x00tarcontent")...),
			wantText: false,
		},
		{
			name:     "unknown binary content",
			content:  []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd},
			wantText: false,
		},
		{
			name:     "empty file",
			content:  []byte(""),
			wantText: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(tc.name, tc.content)
			got, err := isPlainTextDump(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantText {
				t.Fatalf("isPlainTextDump() = %v, want %v", got, tc.wantText)
			}
		})
	}
}

func TestIsPlainTextDump_MissingFile(t *testing.T) {
	if _, err := isPlainTextDump(filepath.Join(t.TempDir(), "nope.sql")); err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
}

func TestNormalizeDumpPath(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		format string
		want   string
	}{
		{name: "plain default appends .sql", in: "backup", format: "", want: "backup.sql"},
		{name: "plain explicit keeps .sql", in: "backup.sql", format: "plain", want: "backup.sql"},
		{name: "custom from no ext", in: "backup", format: "custom", want: "backup.dump"},
		{name: "custom from .sql", in: "backup.sql", format: "custom", want: "backup.dump"},
		{name: "custom keeps .dump", in: "backup.dump", format: "custom", want: "backup.dump"},
		{name: "custom keeps .backup", in: "backup.backup", format: "custom", want: "backup.backup"},
		{name: "tar from no ext", in: "backup", format: "tar", want: "backup.tar"},
		{name: "tar from .sql", in: "backup.sql", format: "tar", want: "backup.tar"},
		{name: "directory strips .sql", in: "backup.sql", format: "directory", want: "backup"},
		{name: "directory keeps plain name", in: "backup", format: "directory", want: "backup"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeDumpPath(tc.in, &DumpOptions{Format: tc.format})
			if got != tc.want {
				t.Fatalf("normalizeDumpPath(%q, %q) = %q, want %q", tc.in, tc.format, got, tc.want)
			}
		})
	}
}

func TestNormalizeDumpPath_DumpToSql(t *testing.T) {
	// A .dump file requested as plain format should become .sql.
	got := normalizeDumpPath("backup.dump", &DumpOptions{Format: "plain"})
	if got != "backup.sql" {
		t.Fatalf("expected backup.sql, got %q", got)
	}
}

func TestBuildPgRestoreArgs(t *testing.T) {
	base := buildPgRestoreArgs("postgres://x", "/tmp/d.sql", false, false)
	if contains(base, "--clean") {
		t.Fatalf("did not expect --clean in non-clean args: %v", base)
	}
	if contains(base, "--exit-on-error") {
		t.Fatalf("did not expect --exit-on-error in tolerant args: %v", base)
	}
	clean := buildPgRestoreArgs("postgres://x", "/tmp/d.sql", true, false)
	if !contains(clean, "--clean") || !contains(clean, "--if-exists") {
		t.Fatalf("expected --clean --if-exists, got %v", clean)
	}
	if !contains(clean, "--no-owner") || !contains(clean, "--no-privileges") {
		t.Fatalf("expected --no-owner --no-privileges, got %v", clean)
	}

	// Strict restores must ask pg_restore to stop at the first failure, because
	// without it a failed object is reported on stderr and the exit status is 1
	// either way.
	strict := buildPgRestoreArgs("postgres://x", "/tmp/d.sql", false, true)
	if !contains(strict, "--exit-on-error") {
		t.Fatalf("expected --exit-on-error for a strict restore, got %v", strict)
	}
	if !contains(strict, "--dbname") {
		t.Fatalf("expected --dbname, got %v", strict)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

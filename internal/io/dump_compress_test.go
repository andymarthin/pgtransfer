package io

import (
	"os"
	"strings"
	"testing"
)

// TestAppendCompressionArgs pins the --compress argument a dump gets.
//
// The level matters for a migration: the archive is written and read again
// within one command, so the default has to be the cheap level rather than
// pg_dump's heavy one. Plain format takes no flag at all, because a plain dump
// is uncompressed by definition and pg_dump rejects it.
func TestAppendCompressionArgs(t *testing.T) {
	tests := []struct {
		name string
		opts DumpOptions
		want string // the flag's value, or "" for no flag
	}{
		{"plain format ignores compress", DumpOptions{Format: "plain", Compress: true}, ""},
		{"no format ignores compress", DumpOptions{Compress: true}, ""},
		{"custom with compress uses the default level", DumpOptions{Format: "custom", Compress: true}, "1"},
		{"explicit level wins", DumpOptions{Format: "custom", Compress: true, CompressLevel: 6}, "6"},
		{"explicit level alone is enough", DumpOptions{Format: "directory", CompressLevel: 3}, "3"},
		{"no compression requested", DumpOptions{Format: "custom"}, ""},
		{"tar with compress", DumpOptions{Format: "tar", Compress: true}, "1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := appendCompressionArgs(nil, &tc.opts)
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("args = %v, want none", got)
				}
				return
			}
			if len(got) != 2 || got[0] != "--compress" || got[1] != tc.want {
				t.Fatalf("args = %v, want [--compress %s]", got, tc.want)
			}
		})
	}
}

// TestCompressionLevelIsCheapByDefault guards the measured choice: a level high
// enough to be slow would put the migration back where it started.
func TestCompressionLevelIsCheapByDefault(t *testing.T) {
	if defaultCompressLevel > 3 {
		t.Errorf("defaultCompressLevel = %d, want a cheap level (<= 3)", defaultCompressLevel)
	}
}

// TestMigrationTempFileIsAnArchive pins the suffix the migration stages.
//
// The restore path decides between pg_restore and psql by reading the file, but
// a temp file that leaks after a crash is looked at by a human, and a name
// ending in .sql for a custom archive is misleading.
func TestMigrationTempFileIsAnArchive(t *testing.T) {
	path, err := migrationTempFile()
	if err != nil {
		t.Fatalf("migrationTempFile() error = %v", err)
	}
	if !strings.HasSuffix(path, ".dump") {
		t.Errorf("migrationTempFile() = %q, want a .dump suffix", path)
	}
	if !strings.Contains(path, "migration_") {
		t.Errorf("migrationTempFile() = %q, want a migration_ prefix", path)
	}

	// Two migrations started at once must not share a staging file: the second
	// would overwrite the first's archive while it is being restored.
	other, err := migrationTempFile()
	if err != nil {
		t.Fatalf("migrationTempFile() error = %v", err)
	}
	if other == path {
		t.Errorf("two calls returned the same path %q", path)
	}
	for _, p := range []string{path, other} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%q was not created: %v", p, err)
		}
		os.Remove(p)
	}
}

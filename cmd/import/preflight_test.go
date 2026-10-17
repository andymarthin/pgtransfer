package importcmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/andymarthin/pgtransfer/internal/preflight"
)

// emptyPATH makes preflight.Lookup report nothing as installed.
func emptyPATH(t *testing.T) {
	t.Helper()
	origLookup := preflight.Lookup
	t.Cleanup(func() { preflight.Lookup = origLookup })
	preflight.Lookup = func(name string) (string, error) {
		return "", errors.New("executable file not found in $PATH")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PGTRANSFER_CONFIG", filepath.Join(home, "config.yaml"))
}

// writeFile writes a fixture dump and returns its path.
func writeFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// TestDumpImportChecksTheToolForTheFileFormat pins that the pre-flight check
// asks for the tool the restore will actually run.
//
// A plain SQL dump is restored with psql and a custom archive with pg_restore.
// Reporting the wrong one sends the user to install something that will not
// help, and hides the thing that will.
func TestDumpImportChecksTheToolForTheFileFormat(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content []byte
		want    string
	}{
		{
			name:    "plain sql needs psql",
			file:    "backup.sql",
			content: []byte("CREATE TABLE t (id int);\n"),
			want:    "psql",
		},
		{
			name:    "custom archive needs pg_restore",
			file:    "backup.dump",
			content: []byte("PGDMP\x01\x0e\x00binary"),
			want:    "pg_restore",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			emptyPATH(t)
			path := writeFile(t, tc.file, tc.content)

			dumpCmd.SetOut(&bytes.Buffer{})
			dumpCmd.SetErr(&bytes.Buffer{})
			err := runDumpImport(dumpCmd, []string{"prod", path})

			if err == nil {
				t.Fatal("runDumpImport() succeeded with no client tools on PATH")
			}
			var missing *preflight.MissingError
			if !errors.As(err, &missing) {
				t.Fatalf("error is %T, want *preflight.MissingError: %v", err, err)
			}
			if len(missing.Tools) != 1 || missing.Tools[0].Name != tc.want {
				t.Errorf("missing tools = %v, want just %s", missing.Tools, tc.want)
			}
		})
	}
}

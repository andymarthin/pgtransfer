package migrate

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// TestDatabaseMigrationNeedsAllThreeTools pins the strictest pre-flight: a
// migration dumps with pg_dump and restores with either pg_restore or psql
// depending on the archive, and it fails in the worst place — after the archive
// is written — if one is missing. So all three are required up front and all
// three are named.
func TestDatabaseMigrationNeedsAllThreeTools(t *testing.T) {
	emptyPATH(t)

	databaseCmd.SetOut(&bytes.Buffer{})
	databaseCmd.SetErr(&bytes.Buffer{})
	err := runDatabaseMigration(databaseCmd, []string{"src", "dst"})

	if err == nil {
		t.Fatal("runDatabaseMigration() succeeded with no client tools on PATH")
	}
	var missing *preflight.MissingError
	if !errors.As(err, &missing) {
		t.Fatalf("error is %T, want *preflight.MissingError: %v", err, err)
	}

	have := make(map[string]bool)
	for _, tool := range missing.Tools {
		have[tool.Name] = true
	}
	for _, want := range []string{"pg_dump", "pg_restore", "psql"} {
		if !have[want] {
			t.Errorf("missing tools %v do not include %s", missing.Tools, want)
		}
	}
}

// TestRollbackChecksTheToolForTheFileFormat pins that a rollback asks for the
// one tool its backup actually needs.
func TestRollbackChecksTheToolForTheFileFormat(t *testing.T) {
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
			path := filepath.Join(t.TempDir(), tc.file)
			if err := os.WriteFile(path, tc.content, 0o600); err != nil {
				t.Fatalf("write %s: %v", tc.file, err)
			}

			rollbackCmd.SetOut(&bytes.Buffer{})
			rollbackCmd.SetErr(&bytes.Buffer{})
			err := runRollback(rollbackCmd, []string{"prod", path})

			if err == nil {
				t.Fatal("runRollback() succeeded with no client tools on PATH")
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

// TestRollbackReportsAMissingBackupFirst pins the order of the checks. A path
// typo is the more likely mistake and the cheaper one to fix, so it is reported
// even on a machine that is also missing psql.
func TestRollbackReportsAMissingBackupFirst(t *testing.T) {
	emptyPATH(t)

	rollbackCmd.SetOut(&bytes.Buffer{})
	rollbackCmd.SetErr(&bytes.Buffer{})
	err := runRollback(rollbackCmd, []string{"prod", filepath.Join(t.TempDir(), "absent.sql")})
	if err == nil {
		t.Fatal("runRollback() succeeded with a missing backup file")
	}
	if !strings.Contains(err.Error(), "backup file does not exist") {
		t.Errorf("error %q does not report the missing backup file", err)
	}
}

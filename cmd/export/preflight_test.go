package export

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andymarthin/pgtransfer/internal/preflight"
)

// emptyPATH makes preflight.Lookup report nothing as installed, and a clean
// HOME so the command cannot fall back to real profiles.
func emptyPATH(t *testing.T) string {
	t.Helper()
	origLookup := preflight.Lookup
	t.Cleanup(func() { preflight.Lookup = origLookup })
	preflight.Lookup = func(name string) (string, error) {
		return "", errors.New("executable file not found in $PATH")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PGTRANSFER_CONFIG", filepath.Join(home, "config.yaml"))
	return home
}

// TestDumpExportFailsBeforeItWritesAnything pins the point of the pre-flight
// check: a machine without pg_dump is told so, and no output file is created.
//
// The check runs before the profile is loaded, so this also covers the case of
// a machine with neither pg_dump nor a config: the missing tool is the more
// useful message.
func TestDumpExportFailsBeforeItWritesAnything(t *testing.T) {
	emptyPATH(t)

	out := filepath.Join(t.TempDir(), "backup.dump")

	dumpCmd.SetOut(&bytes.Buffer{})
	dumpCmd.SetErr(&bytes.Buffer{})
	err := runDumpExport(dumpCmd, []string{"prod", out})

	if err == nil {
		t.Fatal("runDumpExport() succeeded with no pg_dump on PATH")
	}
	var missing *preflight.MissingError
	if !errors.As(err, &missing) {
		t.Fatalf("error is %T, want *preflight.MissingError: %v", err, err)
	}
	if len(missing.Tools) != 1 || missing.Tools[0].Name != "pg_dump" {
		t.Errorf("missing tools = %v, want just pg_dump", missing.Tools)
	}

	// The output path is .dump, so the command would have appended nothing;
	// neither it nor the .sql variant may exist.
	for _, p := range []string{out, out + ".sql"} {
		if _, statErr := os.Stat(p); statErr == nil {
			t.Errorf("%s was created despite the missing tool", p)
		}
	}
}

// TestDumpExportNamesTheTool pins the wording, since that is the user's fix.
func TestDumpExportNamesTheTool(t *testing.T) {
	emptyPATH(t)

	dumpCmd.SetOut(&bytes.Buffer{})
	dumpCmd.SetErr(&bytes.Buffer{})
	err := runDumpExport(dumpCmd, []string{"prod", filepath.Join(t.TempDir(), "b.dump")})
	if err == nil {
		t.Fatal("runDumpExport() succeeded with no pg_dump on PATH")
	}
	if !strings.Contains(err.Error(), "pg_dump") {
		t.Errorf("error %q does not name pg_dump", err)
	}
}

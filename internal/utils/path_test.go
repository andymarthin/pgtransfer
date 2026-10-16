package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetConfigDirAndPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got, want := GetConfigDir(), filepath.Join(home, ".pgtransfer"); got != want {
		t.Fatalf("GetConfigDir() = %q, want %q", got, want)
	}
	if got, want := GetConfigPath(), filepath.Join(home, ".pgtransfer", "config.yaml"); got != want {
		t.Fatalf("GetConfigPath() = %q, want %q", got, want)
	}
	if got, want := GetLogDir(), filepath.Join(home, ".pgtransfer", "logs"); got != want {
		t.Fatalf("GetLogDir() = %q, want %q", got, want)
	}
}

func TestGetDefaultExportPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := GetDefaultExportPath("users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantDir := filepath.Join(home, ".pgtransfer", "exports")
	if filepath.Dir(got) != wantDir {
		t.Fatalf("export path dir = %q, want %q", filepath.Dir(got), wantDir)
	}
	base := filepath.Base(got)
	if !strings.HasPrefix(base, "users_") || !strings.HasSuffix(base, ".csv") {
		t.Fatalf("export filename = %q, want users_<timestamp>.csv", base)
	}

	// The directory must actually be created.
	if _, err := os.Stat(wantDir); err != nil {
		t.Fatalf("expected exports directory to exist: %v", err)
	}
}

func TestGetDefaultDumpPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := GetDefaultDumpPath("mydb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantDir := filepath.Join(home, ".pgtransfer", "dumps")
	if filepath.Dir(got) != wantDir {
		t.Fatalf("dump path dir = %q, want %q", filepath.Dir(got), wantDir)
	}
	base := filepath.Base(got)
	if !strings.HasPrefix(base, "mydb_") || !strings.HasSuffix(base, ".sql") {
		t.Fatalf("dump filename = %q, want mydb_<timestamp>.sql", base)
	}
	if _, err := os.Stat(wantDir); err != nil {
		t.Fatalf("expected dumps directory to exist: %v", err)
	}
}

func TestGetDefaultExportPath_MkdirFailure(t *testing.T) {
	// Point HOME at a path whose parent is a file, so MkdirAll fails.
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	t.Setenv("HOME", filepath.Join(blocker, "sub"))

	if _, err := GetDefaultExportPath("t"); err == nil {
		t.Fatalf("expected error when the exports directory cannot be created")
	}
	if _, err := GetDefaultDumpPath("d"); err == nil {
		t.Fatalf("expected error when the dumps directory cannot be created")
	}
}

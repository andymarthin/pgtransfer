package log

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andymarthin/pgtransfer/internal/utils"
)

func todayLogPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(utils.GetLogDir(), time.Now().Format("2006-01-02")+".log")
}

func readEntries(t *testing.T, path string) []Entry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read log file %s: %v", path, err)
	}
	var entries []Entry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("invalid JSON line %q: %v", line, err)
		}
		entries = append(entries, e)
	}
	return entries
}

func TestSuccess_WritesEntry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	start := time.Now().Add(-2 * time.Second)
	Success("export csv", "prod", "exported 10 rows", start)

	entries := readEntries(t, todayLogPath(t))
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Status != "SUCCESS" || e.Command != "export csv" || e.Profile != "prod" {
		t.Fatalf("unexpected entry: %+v", e)
	}
	if e.Duration == "" {
		t.Fatalf("expected a duration to be recorded")
	}
	if _, err := time.Parse(time.RFC3339, e.Timestamp); err != nil {
		t.Fatalf("timestamp not RFC3339: %q", e.Timestamp)
	}
}

func TestFailureAndInfo_Append(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	Failure("import dump", "dev", "restore failed", time.Now())
	Info("version", "", "printed version")

	entries := readEntries(t, todayLogPath(t))
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Status != "FAILURE" {
		t.Fatalf("expected first entry FAILURE, got %q", entries[0].Status)
	}
	if entries[1].Status != "INFO" {
		t.Fatalf("expected second entry INFO, got %q", entries[1].Status)
	}
}

func TestWrite_CreatesLogDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	Success("cmd", "p", "m", time.Now())

	if _, err := os.Stat(utils.GetLogDir()); err != nil {
		t.Fatalf("expected log dir to be created: %v", err)
	}
}

func TestWrite_MkdirFailureIsSilentInHelpers(t *testing.T) {
	// Helpers swallow write errors; make the log dir impossible to create and
	// ensure they do not panic.
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	t.Setenv("HOME", filepath.Join(blocker, "sub"))

	Success("c", "p", "m", time.Now())
	Failure("c", "p", "m", time.Now())
	Info("c", "p", "m")
}

func TestReadLogs_MissingFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Should not error; it prints a warning and returns nil.
	if err := ReadLogs("1999-01-01"); err != nil {
		t.Fatalf("unexpected error for missing log: %v", err)
	}
}

func TestReadLogs_EmptyFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := utils.GetLogDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2020-01-01.log"), []byte(""), 0o600); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if err := ReadLogs("2020-01-01"); err != nil {
		t.Fatalf("unexpected error for empty log: %v", err)
	}
}

func TestReadLogs_AllStatusesAndSorting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := utils.GetLogDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Written out of order and with an unknown status, plus a malformed line
	// that must be skipped.
	content := "" +
		`{"timestamp":"2021-05-02T10:00:00Z","command":"c","profile":"p","status":"INFO","message":"third"}` + "\n" +
		`{"timestamp":"2021-05-02T08:00:00Z","command":"c","profile":"p","status":"SUCCESS","message":"first","duration":"1s"}` + "\n" +
		"not json\n" +
		`{"timestamp":"2021-05-02T09:00:00Z","command":"c","profile":"p","status":"FAILURE","message":"second","duration":"2s"}` + "\n" +
		`{"timestamp":"2021-05-02T11:00:00Z","command":"c","profile":"p","status":"OTHER","message":"fourth"}` + "\n"

	if err := os.WriteFile(filepath.Join(dir, "2021-05-02.log"), []byte(content), 0o600); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	out := captureStdout(t, func() {
		if err := ReadLogs("2021-05-02"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// The malformed line is dropped; the remaining four are printed sorted by
	// timestamp.
	idxFirst := strings.Index(out, "first")
	idxSecond := strings.Index(out, "second")
	idxThird := strings.Index(out, "third")
	idxFourth := strings.Index(out, "fourth")
	if idxFirst < 0 || idxSecond < 0 || idxThird < 0 || idxFourth < 0 {
		t.Fatalf("expected all entries in output, got: %q", out)
	}
	if !(idxFirst < idxSecond && idxSecond < idxThird && idxThird < idxFourth) {
		t.Fatalf("entries not sorted by timestamp: %q", out)
	}
}

func TestReadLogs_DefaultsToToday(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	Success("cmd", "p", "today entry", time.Now())

	out := captureStdout(t, func() {
		if err := ReadLogs(""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "today entry") {
		t.Fatalf("expected today's entry in output, got %q", out)
	}
}

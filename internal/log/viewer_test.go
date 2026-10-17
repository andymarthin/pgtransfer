package log

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/andymarthin/pgtransfer/internal/utils"
)

// writeLogFile writes raw content to the log file for a date.
func writeLogFile(t *testing.T, date, content string) {
	t.Helper()
	dir := utils.GetLogDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, date+".log"), []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// TestLogPath pins the file naming and that an empty date means today.
func TestLogPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	want := filepath.Join(utils.GetLogDir(), "2026-10-17.log")
	if got := LogPath("2026-10-17"); got != want {
		t.Errorf("LogPath() = %q, want %q", got, want)
	}

	today := filepath.Join(utils.GetLogDir(), time.Now().Format("2006-01-02")+".log")
	if got := LogPath(""); got != today {
		t.Errorf("LogPath(\"\") = %q, want %q", got, today)
	}
}

// TestEntriesMissingFileIsNotAnError pins that a day with no log is empty, not a
// failure. Every machine is in that state before its first command.
func TestEntriesMissingFileIsNotAnError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	entries, err := Entries("1999-01-01")
	if err != nil {
		t.Fatalf("Entries() = %v, want nil", err)
	}
	if len(entries) != 0 {
		t.Errorf("Entries() returned %d entries, want 0", len(entries))
	}
}

// TestEntriesEmptyFile pins that an empty log reads as no entries.
func TestEntriesEmptyFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeLogFile(t, "2020-01-01", "")

	entries, err := Entries("2020-01-01")
	if err != nil {
		t.Fatalf("Entries(): %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Entries() = %d entries, want 0", len(entries))
	}
}

// TestEntriesSortsAndSkipsMalformed pins both the ordering and that one bad
// line does not hide the rest of the day.
func TestEntriesSortsAndSkipsMalformed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	content := "" +
		`{"timestamp":"2021-05-02T10:00:00Z","command":"c","status":"INFO","message":"third"}` + "\n" +
		`{"timestamp":"2021-05-02T08:00:00Z","command":"c","status":"SUCCESS","message":"first","duration":"1s"}` + "\n" +
		"not json\n" +
		"\n" +
		`{"timestamp":"2021-05-02T09:00:00Z","command":"c","status":"FAILURE","message":"second"}` + "\n"
	writeLogFile(t, "2021-05-02", content)

	entries, err := Entries("2021-05-02")
	if err != nil {
		t.Fatalf("Entries(): %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("Entries() = %d entries, want 3 (one malformed and one blank line skipped)", len(entries))
	}

	want := []string{"first", "second", "third"}
	for i, e := range entries {
		if e.Message != want[i] {
			t.Errorf("entry %d = %q, want %q", i, e.Message, want[i])
		}
	}
}

// TestEntriesRejectsBadDate pins that a malformed --date is reported rather than
// silently looking up a file that cannot exist.
func TestEntriesRejectsBadDate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	for _, date := range []string{"17-10-2026", "yesterday", "2026-13-01", "2026-10-32"} {
		if _, err := Entries(date); err == nil {
			t.Errorf("Entries(%q) = nil error, want a rejection", date)
		}
	}
}

// TestAvailableDates pins that only well-formed day files are listed, newest
// first.
func TestAvailableDates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	for _, date := range []string{"2026-10-17", "2026-10-15", "2026-10-16"} {
		writeLogFile(t, date, "")
	}
	// Not a date: must be ignored.
	writeLogFile(t, "notes", "hello")

	got := AvailableDates()
	want := []string{"2026-10-17", "2026-10-16", "2026-10-15"}
	if len(got) != len(want) {
		t.Fatalf("AvailableDates() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AvailableDates()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestAvailableDatesNoDirectory pins that a machine with no log directory gets
// an empty list rather than an error.
func TestAvailableDatesNoDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := AvailableDates(); len(got) != 0 {
		t.Errorf("AvailableDates() = %v, want empty", got)
	}
}

// TestEntriesRoundTripsWithWriter pins that what Success/Failure/Info write is
// what Entries reads back, so the viewer cannot drift from the writer.
func TestEntriesRoundTripsWithWriter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	start := time.Now().Add(-2 * time.Second)
	Success("migrate database", "production", "migrated to staging", start)
	Failure("migrate database", "production", "restore failed", start)
	Info("migrate database", "production", "starting")

	entries, err := Entries("")
	if err != nil {
		t.Fatalf("Entries(): %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("Entries() = %d entries, want 3", len(entries))
	}

	// The file is JSONL: one object per line, which is what makes it tail-able.
	raw, err := os.ReadFile(LogPath(""))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for i, line := range splitNonEmpty(raw) {
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatalf("line %d is not a JSON object: %v", i+1, err)
		}
	}
}

func splitNonEmpty(data []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				out = append(out, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		out = append(out, data[start:])
	}
	return out
}

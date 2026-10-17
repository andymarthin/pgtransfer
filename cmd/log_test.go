package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andymarthin/pgtransfer/internal/log"
	"github.com/andymarthin/pgtransfer/internal/output"
	"github.com/andymarthin/pgtransfer/internal/utils"
	"github.com/spf13/cobra"
)

// resetLogFlags restores the package-level log flags and output mode after a
// test, since cobra and the output package keep both across invocations in a
// single process.
func resetLogFlags(t *testing.T) {
	t.Helper()
	output.Reset()
	t.Cleanup(func() {
		logDate = ""
		logLimit = 0
		output.Reset()
	})
}

// captureOutput runs fn with output redirected to a buffer and returns what was
// written. The command's own writers take priority over the process-level ones
// most helpers use, so both are pointed at the same buffer.
func captureOutput(t *testing.T, fn func(*cobra.Command) error) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	output.SetWriters(&buf, &buf)
	defer output.SetWriters(os.Stdout, os.Stderr)

	cmd := &cobra.Command{Use: "log"}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	runErr := fn(cmd)
	return buf.String(), runErr
}

// TestLogCmd_RejectsBadDate pins that a malformed --date is a usage error and
// that it is reported before anything reads the filesystem.
func TestLogCmd_RejectsBadDate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetLogFlags(t)

	logDate = "17/10/2026"
	if err := runLog(logCmd, nil); err == nil {
		t.Fatalf("runLog() with a bad date = nil, want an error")
	}
}

// TestLogCmd_NoEntries pins the empty-state message, including that it names the
// days that do have entries so the user is not left guessing.
func TestLogCmd_NoEntries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetLogFlags(t)

	dir := utils.GetLogDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-10-16.log"), nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	logDate = "2026-10-17"
	out, err := captureOutput(t, func(c *cobra.Command) error { return runLog(c, nil) })
	if err != nil {
		t.Fatalf("runLog() error: %v", err)
	}
	if !strings.Contains(out, "No operations recorded on 2026-10-17") {
		t.Errorf("output = %q, want the empty-state message for the requested day", out)
	}
	if !strings.Contains(out, "2026-10-16") {
		t.Errorf("output = %q, want it to list a day that has entries", out)
	}
}

// TestLogCmd_RendersAndLimits pins the table contents and that --limit keeps the
// newest entries, not the first ones read.
func TestLogCmd_RendersAndLimits(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetLogFlags(t)

	dir := utils.GetLogDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := strings.Join([]string{
		`{"timestamp":"2026-10-17T08:00:00Z","command":"migrate database","profile":"prod","status":"SUCCESS","message":"oldest","duration":"3s"}`,
		`{"timestamp":"2026-10-17T09:00:00Z","command":"migrate database","profile":"prod","status":"FAILURE","message":"middle"}`,
		`{"timestamp":"2026-10-17T10:00:00Z","command":"migrate database","profile":"","status":"INFO","message":"newest"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "2026-10-17.log"), []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	logDate = "2026-10-17"
	logLimit = 2
	out, err := captureOutput(t, func(c *cobra.Command) error { return runLog(c, nil) })
	if err != nil {
		t.Fatalf("runLog() error: %v", err)
	}

	// The newest two are kept; the oldest must be off the page.
	if strings.Contains(out, "oldest") {
		t.Errorf("output includes the oldest entry despite --limit 2:\n%s", out)
	}
	for _, want := range []string{"middle", "newest", "fail", "info", "Showing the last 2 of 3 entries"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// An entry with no profile renders as a placeholder rather than an empty
	// cell, which the tabwriter would collapse and shift the columns left.
	if !strings.Contains(out, "-        newest") {
		t.Errorf("output missing the empty-profile placeholder:\n%s", out)
	}
}

// TestLogCmd_TodayDefault pins that no --date shows today.
func TestLogCmd_TodayDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetLogFlags(t)

	today := time.Now().Format("2006-01-02")
	dir := utils.GetLogDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	line := `{"timestamp":"` + time.Now().UTC().Format(time.RFC3339) + `","command":"c","status":"SUCCESS","message":"today-marker"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, today+".log"), []byte(line), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	out, err := captureOutput(t, func(c *cobra.Command) error { return runLog(c, nil) })
	if err != nil {
		t.Fatalf("runLog() error: %v", err)
	}
	if !strings.Contains(out, "today-marker") || !strings.Contains(out, today) {
		t.Errorf("output = %q, want today's entry and date", out)
	}
}

// TestLogCmd_JSON pins the machine-readable shape: a stable key set, and a
// non-nil array for a day with no entries so consumers do not have to handle
// null.
func TestLogCmd_JSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PGTRANSFER_JSON", "1")
	resetLogFlags(t)
	output.Configure(false)

	logDate = "2026-10-17"
	out, err := captureOutput(t, func(c *cobra.Command) error { return runLog(c, nil) })
	if err != nil {
		t.Fatalf("runLog() error: %v", err)
	}

	var result struct {
		OK   bool `json:"ok"`
		Data struct {
			Date    string     `json:"date"`
			Total   int        `json:"total"`
			Entries []logEntry `json:"entries"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("output is not one JSON object: %v\n%s", err, out)
	}
	if !result.OK || result.Data.Date != "2026-10-17" || result.Data.Total != 0 {
		t.Errorf("result = %+v, want ok with the requested date and no entries", result)
	}
	if result.Data.Entries == nil {
		t.Errorf("entries = null, want an empty array")
	}
}

// logEntry mirrors the fields a consumer relies on, so a rename in the log
// package breaks this test rather than a user's script.
type logEntry struct {
	Timestamp string `json:"timestamp"`
	Command   string `json:"command"`
	Profile   string `json:"profile"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

var _ = log.Entry{}

package utils

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// captureStdout redirects os.Stdout for the duration of fn and returns what was
// written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	_ = w.Close()
	os.Stdout = old
	return <-done
}

func TestPrintHelpers_NilCmd(t *testing.T) {
	tests := []struct {
		name string
		fn   func()
		want string
	}{
		{"success", func() { PrintSuccess(nil, "ok %d", 1) }, "✅ ok 1"},
		{"warning", func() { PrintWarning(nil, "warn") }, "⚠️  warn"},
		{"info", func() { PrintInfo(nil, "info") }, "ℹ️  info"},
		{"note", func() { PrintNote(nil, "note") }, "🔷 note"},
		{"muted", func() { PrintMuted(nil, "muted") }, "muted"},
		{"title", func() { PrintTitle(nil, "Heading") }, "Heading"},
		{"divider", func() { PrintDivider(nil) }, "----------------------------------------"},
		{"error", func() { PrintError(nil, "boom") }, "❌ boom"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := captureStdout(t, tc.fn)
			if !strings.Contains(out, tc.want) {
				t.Fatalf("output %q does not contain %q", out, tc.want)
			}
		})
	}
}

func TestPrintHelpers_WithCmd(t *testing.T) {
	// When a cobra command is supplied, messages go through its writers.
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	PrintSuccess(cmd, "done")
	PrintTitle(cmd, "Title")

	out := buf.String()
	if !strings.Contains(out, "done") {
		t.Fatalf("expected cmd output to contain 'done', got %q", out)
	}
	if !strings.Contains(out, "Title") {
		t.Fatalf("expected cmd output to contain 'Title', got %q", out)
	}
}

func TestColorTextHelpers(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"green", ColorTextGreen("x"), ColorGreen + "x" + ColorReset},
		{"red", ColorTextRed("x"), ColorRed + "x" + ColorReset},
		{"cyan", ColorTextCyan("x"), ColorCyan + "x" + ColorReset},
		{"blue", ColorTextBlue("x"), ColorBlue + "x" + ColorReset},
		{"yellow", ColorTextYellow("x"), ColorYellow + "x" + ColorReset},
		{"gray", ColorTextGray("x"), ColorGray + "x" + ColorReset},
		{"bold", ColorTextBold("x"), ColorBold + "x" + ColorReset},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("got %q, want %q", tc.got, tc.want)
			}
		})
	}
}

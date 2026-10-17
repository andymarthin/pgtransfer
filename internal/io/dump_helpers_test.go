package io

import (
	"bytes"
	"strings"
	"testing"
)

func TestRestoreIsStrict(t *testing.T) {
	tests := []struct {
		name string
		opts *RestoreOptions
		want bool
	}{
		{"nil options", nil, false},
		{"default", &RestoreOptions{}, false},
		{"strict", &RestoreOptions{Strict: true}, true},
		{"clean only", &RestoreOptions{Clean: true}, false},
		{"timeout only", &RestoreOptions{Timeout: 30}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := restoreIsStrict(tc.opts); got != tc.want {
				t.Fatalf("restoreIsStrict(%+v) = %v, want %v", tc.opts, got, tc.want)
			}
		})
	}
}

func TestClassifyRestoreLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want restoreLineKind
	}{
		// Version skew, the case the tolerant default exists for.
		{"unrecognized parameter", `psql:/tmp/d.sql:12: ERROR:  unrecognized configuration parameter "transaction_timeout"`, restoreLineBenign},
		{"role already exists", `pg_restore: error: could not execute query: ERROR:  role "app" already exists`, restoreLineBenign},
		{"table already exists", `ERROR:  relation "users" already exists`, restoreLineBenign},
		{"explicit notice", `NOTICE:  extension "plpgsql" already exists, skipping`, restoreLineBenign},
		{"drop skipping", `pg_restore: error: could not execute query: ERROR:  table "old" does not exist, skipping`, restoreLineBenign},
		{"ownership warning", `WARNING:  no privileges could be revoked for "public"`, restoreLineBenign},
		{"no owner", `WARNING:  no ownership could be changed for "reporting"`, restoreLineBenign},

		// Real failures that used to be swallowed because pg_restore exits 1
		// for both these and the benign cases above.
		{"duplicate key", `pg_restore: error: COPY failed for table "users": ERROR:  duplicate key value violates unique constraint "users_pkey"`, restoreLineError},
		{"not null violation", `pg_restore: error: COPY failed for table "orders": ERROR:  null value in column "id" violates not-null constraint`, restoreLineError},
		{"permission denied", `ERROR:  permission denied for table audit_log`, restoreLineError},
		{"undefined table", `ERROR:  relation "missing_table" does not exist`, restoreLineError},
		{"out of memory", `FATAL:  out of memory`, restoreLineError},
		{"disk full", `ERROR:  could not extend file "base/16384/16400": No space left on device`, restoreLineError},

		// Neither: chatter, progress, empty lines.
		{"empty", "", restoreLineUnknown},
		{"blank", "   ", restoreLineUnknown},
		{"info line", `pg_restore: creating TABLE public.users`, restoreLineUnknown},
		{"psql set", `SET`, restoreLineUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyRestoreLine(tc.line); got != tc.want {
				t.Fatalf("classifyRestoreLine(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

// TestStderrCollectorSplitsLines checks that classification works when the
// tool writes partial lines, which is how a pipe actually delivers output.
func TestStderrCollectorSplitsLines(t *testing.T) {
	var sink bytes.Buffer
	c := &stderrCollector{w: &sink}

	// One benign error and one fatal error, split across five writes.
	for _, chunk := range []string{
		"psql:/tmp/d.sql:1: ERROR:  unrecog",
		"nized configuration parameter \"x\"\npg_restore: error: COPY fai",
		"led for table \"users\": ERROR:  duplicate key value\n",
		"pg_restore: creating INDEX",
	} {
		if _, err := c.Write([]byte(chunk)); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}

	if got := c.warningCount(); got != 1 {
		t.Errorf("warningCount() = %d, want 1", got)
	}
	problems := c.erroneousLines()
	if len(problems) != 1 {
		t.Fatalf("erroneousLines() = %v, want exactly the duplicate-key line", problems)
	}
	if !strings.Contains(problems[0], "duplicate key value") {
		t.Errorf("captured %q, want the duplicate-key line", problems[0])
	}

	// Everything must still have reached the user.
	if !strings.Contains(sink.String(), "duplicate key value") {
		t.Errorf("stderr was not passed through: %q", sink.String())
	}
}

// TestStderrCollectorFlushesPartialLine covers a trailing line with no newline,
// which is what a killed process leaves behind.
func TestStderrCollectorFlushesPartialLine(t *testing.T) {
	var sink bytes.Buffer
	c := &stderrCollector{w: &sink}
	_, _ = c.Write([]byte(`pg_restore: error: COPY failed for table "x": ERROR:  permission denied`))

	if len(c.erroneousLines()) != 0 {
		t.Fatalf("line should not be classified before the buffer is flushed")
	}
	c.flush()
	if len(c.erroneousLines()) != 1 {
		t.Fatalf("flush should classify the trailing line, got %v", c.erroneousLines())
	}
}

// TestStderrCollectorIsBounded guards against a restore that emits unbounded
// errors exhausting memory.
func TestStderrCollectorIsBounded(t *testing.T) {
	var sink bytes.Buffer
	c := &stderrCollector{w: &sink}
	line := []byte("ERROR:  permission denied for table x\n")
	for i := 0; i < stderrCollectorLimit*5; i++ {
		_, _ = c.Write(line)
	}
	if len(c.erroneousLines()) > stderrCollectorLimit {
		t.Fatalf("retained %d lines, want at most %d", len(c.erroneousLines()), stderrCollectorLimit)
	}
	if len(c.erroneousLines()) == 0 {
		t.Fatalf("should have retained some lines")
	}
}

func TestCommandExists(t *testing.T) {
	// `sh` exists on every Unix-like CI runner.
	if !commandExists("sh") {
		t.Fatalf("expected 'sh' to be found on PATH")
	}
	if commandExists("definitely-not-a-real-binary-xyz") {
		t.Fatalf("expected unknown binary to not be found")
	}
}

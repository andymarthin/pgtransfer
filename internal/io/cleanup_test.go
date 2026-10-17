package io

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestRemoveOnFailureKeepsFileOnSuccess pins that a successful dump is kept.
func TestRemoveOnFailureKeepsFileOnSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keep.sql")
	if err := os.WriteFile(path, []byte("-- dump"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := removeOnFailure(path, nil); err != nil {
		t.Fatalf("removeOnFailure(nil) = %v, want nil", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("successful dump was removed: %v", err)
	}
}

// TestRemoveOnFailureDeletesPartialFile pins that a failed dump does not leave
// a truncated file that looks like a backup.
func TestRemoveOnFailureDeletesPartialFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial.sql")
	if err := os.WriteFile(path, []byte("-- truncated"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cause := errors.New("pg_dump failed")
	err := removeOnFailure(path, cause)
	if err == nil {
		t.Fatal("removeOnFailure() returned nil for a failure")
	}
	if !errors.Is(err, cause) {
		t.Errorf("removeOnFailure() lost the cause: %v", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("removeOnFailure() error does not name the file: %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("partial dump still on disk: %v", statErr)
	}
}

// TestRemoveOnFailureReportsUnremovableFile pins that a file we cannot remove is
// reported instead of being silently left behind.
func TestRemoveOnFailureReportsUnremovableFile(t *testing.T) {
	dir := t.TempDir()
	// A non-empty directory cannot be removed with os.Remove.
	path := filepath.Join(dir, "d")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "f"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := removeOnFailure(path, errors.New("boom"))
	if err == nil || !strings.Contains(err.Error(), "failed to remove") {
		t.Fatalf("removeOnFailure() = %v, want a removal warning", err)
	}
}

// TestSyncFileFlushes pins that syncFile accepts a plain file and that syncing a
// closed handle is reported rather than ignored.
func TestSyncFileFlushes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.sql")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := f.WriteString("data"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := syncFile(f); err != nil {
		t.Fatalf("syncFile() = %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if err := syncFile(f); err == nil {
		t.Error("syncFile() on a closed handle returned nil")
	}
}

// TestSyncPathFlushesExistingFile covers the --file code path, where pg_dump
// wrote the file and we only have its path.
func TestSyncPathFlushesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom.dump")
	if err := os.WriteFile(path, []byte("dump"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := syncPath(path); err != nil {
		t.Fatalf("syncPath() = %v", err)
	}
	if err := syncPath(filepath.Join(t.TempDir(), "absent.dump")); err == nil {
		t.Error("syncPath() accepted a missing file")
	}
}

// TestProgressBarTimerDoesNotLeak pins that Finish releases the elapsed-time
// goroutine. The bar is created, never finished for a while, then finished; the
// goroutine count must return to where it started.
//
// Leaked goroutines were one per bar, which a single command never noticed but
// a long-lived process hosting the MCP tools did.
func TestProgressBarTimerDoesNotLeak(t *testing.T) {
	before := runtime.NumGoroutine()

	const bars = 50
	finished := make([]*Bar, 0, bars)
	for i := 0; i < bars; i++ {
		finished = append(finished, NewProgressBarWithTimer(10, "test"))
	}
	// A spinner has its own path through the constructor.
	finished = append(finished, NewProgressBarWithTimer(-1, "spinner"))
	finished = append(finished, NewProgressBarWithTimer(0, "zero"))

	for _, bar := range finished {
		bar.Finish()
		bar.Finish() // idempotent
	}

	if n := waitForGoroutines(before, 2*time.Second); n > before+2 {
		t.Errorf("goroutines after Finish: %d, want about %d", n, before)
	}
}

// TestProgressBarTimerStopsOnFinishWithoutPolling pins that a bar finished
// immediately does not keep describing itself a second later.
func TestProgressBarTimerStopsOnFinishWithoutPolling(t *testing.T) {
	bar := NewProgressBarWithTimer(3, "short-lived")
	bar.Add(3)
	if err := bar.Finish(); err != nil {
		t.Fatalf("Finish() = %v", err)
	}

	// Give a leaked goroutine time to tick; it would panic-free but would show
	// up as a goroutine above the baseline.
	before := runtime.NumGoroutine()
	time.Sleep(1200 * time.Millisecond)
	if n := runtime.NumGoroutine(); n > before+2 {
		t.Errorf("goroutines grew from %d to %d after Finish", before, n)
	}
}

// waitForGoroutines polls until the goroutine count is at most target or the
// timeout expires, then returns the final count.
func waitForGoroutines(target int, timeout time.Duration) int {
	deadline := time.Now().Add(timeout)
	for {
		n := runtime.NumGoroutine()
		if n <= target || time.Now().After(deadline) {
			return n
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestRestoreOptionsForCarriesStrict pins that a migration's --strict reaches
// the restore. Without it a migration asked to stop on the first error would
// tolerate every restore error.
func TestRestoreOptionsForCarriesStrict(t *testing.T) {
	opts := &MigrationOptions{Overwrite: true, Strict: true}
	got := restoreOptionsFor(opts)
	if got == nil {
		t.Fatal("restoreOptionsFor() returned nil")
	}
	if !got.Strict {
		t.Error("restoreOptionsFor() dropped Strict")
	}
	if !got.Clean {
		t.Error("restoreOptionsFor() dropped Clean")
	}

	loose := restoreOptionsFor(&MigrationOptions{})
	if loose.Strict || loose.Clean {
		t.Errorf("restoreOptionsFor(zero) = %+v, want both false", loose)
	}
}

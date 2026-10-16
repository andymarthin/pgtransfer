package io

import (
	"errors"
	"os/exec"
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

func TestPgRestoreErrorsIgnored(t *testing.T) {
	t.Run("non-exec error", func(t *testing.T) {
		if pgRestoreErrorsIgnored(errors.New("boom")) {
			t.Fatalf("expected false for a non-exec error")
		}
	})

	t.Run("nil error", func(t *testing.T) {
		if pgRestoreErrorsIgnored(nil) {
			t.Fatalf("expected false for nil error")
		}
	})

	t.Run("exit code 1 is ignored", func(t *testing.T) {
		// Run a command that exits with status 1.
		err := exec.Command("sh", "-c", "exit 1").Run()
		if err == nil {
			t.Fatalf("expected command to fail")
		}
		if !pgRestoreErrorsIgnored(err) {
			t.Fatalf("expected exit code 1 to be treated as ignorable")
		}
	})

	t.Run("exit code 2 is not ignored", func(t *testing.T) {
		err := exec.Command("sh", "-c", "exit 2").Run()
		if err == nil {
			t.Fatalf("expected command to fail")
		}
		if pgRestoreErrorsIgnored(err) {
			t.Fatalf("expected exit code 2 to be treated as fatal")
		}
	})
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

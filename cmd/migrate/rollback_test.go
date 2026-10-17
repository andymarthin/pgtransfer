package migrate

import (
	"strings"
	"testing"

	"github.com/andymarthin/pgtransfer/internal/io"
)

// TestRollbackStrictReachesRestore pins the bug this test exists for: the
// rollback built a RestoreOptions without Strict, so --strict was accepted,
// printed in the help, and silently ignored. A rollback asked to stop on the
// first error therefore tolerated every error and reported success.
func TestRollbackStrictReachesRestore(t *testing.T) {
	opts := restoreOptionsForRollback(&io.MigrationOptions{Strict: true, Timeout: 120, Verbose: true})

	if !opts.Strict {
		t.Fatalf("RestoreOptions.Strict = false, want true when the rollback was asked to be strict")
	}
	if opts.Timeout != 120 {
		t.Errorf("RestoreOptions.Timeout = %d, want the 120 from the migration options", opts.Timeout)
	}
	if !opts.Verbose {
		t.Errorf("RestoreOptions.Verbose = false, want true so --verbose prints the command")
	}
}

// TestRollbackNonStrictStaysNonStrict pins the other direction, so the fix
// cannot be "always strict" — a default rollback must still tolerate version
// skew, which is why the default exists.
func TestRollbackNonStrictStaysNonStrict(t *testing.T) {
	opts := restoreOptionsForRollback(&io.MigrationOptions{})
	if opts.Strict {
		t.Fatalf("RestoreOptions.Strict = true, want false by default")
	}
	if opts.Verbose {
		t.Fatalf("RestoreOptions.Verbose = true, want false by default")
	}
}

// TestRollbackCommandHasStrictFlag pins that the flag is registered. Wiring a
// flag variable into a struct is easy to lose in a refactor, and the failure is
// silent: the command still runs, just with the wrong semantics.
func TestRollbackCommandHasStrictFlag(t *testing.T) {
	f := rollbackCmd.Flags().Lookup("strict")
	if f == nil {
		t.Fatalf("migrate rollback has no --strict flag")
	}
	if f.DefValue != "false" {
		t.Errorf("--strict default = %q, want false", f.DefValue)
	}
	if !strings.Contains(strings.ToLower(f.Usage), "strict") && !strings.Contains(strings.ToLower(f.Usage), "first") {
		t.Errorf("--strict usage %q does not explain what it does", f.Usage)
	}
}

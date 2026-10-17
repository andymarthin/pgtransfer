package preflight

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubLookup makes Lookup report exactly the named tools as present, and
// restores the real one when the test ends.
func stubLookup(t *testing.T, present ...string) {
	t.Helper()
	have := make(map[string]bool, len(present))
	for _, p := range present {
		have[p] = true
	}
	orig := Lookup
	t.Cleanup(func() { Lookup = orig })
	Lookup = func(name string) (string, error) {
		if have[name] {
			return "/usr/local/bin/" + name, nil
		}
		return "", errors.New("executable file not found in $PATH")
	}
}

// TestCheckReportsEveryMissingTool pins the behaviour that matters for a user
// fixing their machine: one message listing everything absent, not one error
// per attempt.
func TestCheckReportsEveryMissingTool(t *testing.T) {
	stubLookup(t) // nothing installed

	err := Check(PgDump, PgRestore)
	if err == nil {
		t.Fatal("Check() returned nil with no tools installed")
	}

	var missing *MissingError
	if !errors.As(err, &missing) {
		t.Fatalf("error is %T, want *MissingError", err)
	}
	if len(missing.Tools) != 2 {
		t.Fatalf("Tools = %v, want both pg_dump and pg_restore", missing.Tools)
	}

	msg := err.Error()
	for _, want := range []string{"pg_dump", "pg_restore", "PATH"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not mention %q", msg, want)
		}
	}
}

// TestCheckPassesWhenInstalled pins the happy path.
func TestCheckPassesWhenInstalled(t *testing.T) {
	stubLookup(t, "pg_dump", "pg_restore")
	if err := Check(PgDump, PgRestore); err != nil {
		t.Fatalf("Check() = %v with both installed", err)
	}
}

// TestCheckIncludesTheReason pins that the message says which command needs the
// tool: "pg_restore not found" is less useful than "pg_restore, needed by
// restoring an archive".
func TestCheckIncludesTheReason(t *testing.T) {
	stubLookup(t)
	err := Check(PgDump)
	if err == nil {
		t.Fatal("Check() returned nil")
	}
	if !strings.Contains(err.Error(), PgDump.Why) {
		t.Errorf("message %q does not say what needs %s", err, PgDump.Name)
	}
}

// TestCheckPartialInstall pins that a tool which is present is not reported.
func TestCheckPartialInstall(t *testing.T) {
	stubLookup(t, "pg_dump")
	err := Check(PgDump, PgRestore)
	if err == nil {
		t.Fatal("Check() returned nil with pg_restore absent")
	}
	var missing *MissingError
	if !errors.As(err, &missing) {
		t.Fatalf("error is %T, want *MissingError", err)
	}
	if len(missing.Tools) != 1 || missing.Tools[0].Name != "pg_restore" {
		t.Errorf("Tools = %v, want only pg_restore", missing.Tools)
	}
}

// TestCheckNoTools pins that asking for nothing is not an error.
func TestCheckNoTools(t *testing.T) {
	stubLookup(t)
	if err := Check(); err != nil {
		t.Errorf("Check() with no tools = %v, want nil", err)
	}
}

// TestCheckSSHIdentity covers the reasons a tunnelled profile cannot
// authenticate. The tunnel runs in-process, so this is the SSH check that can
// happen before any connection is attempted.
func TestCheckSSHIdentity(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(key, []byte("not a real key"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	tests := []struct {
		name    string
		id      SSHIdentity
		wantErr bool
	}{
		{
			name: "key present",
			id:   SSHIdentity{KeyPath: key},
		},
		{
			name:    "key missing",
			id:      SSHIdentity{KeyPath: filepath.Join(dir, "absent")},
			wantErr: true,
		},
		{
			name:    "key is a directory",
			id:      SSHIdentity{KeyPath: dir},
			wantErr: true,
		},
		{
			// A profile that names a key wants that key; an unreadable one is
			// an error even with a password to fall back on.
			name:    "broken key with a password is still an error",
			id:      SSHIdentity{KeyPath: filepath.Join(dir, "absent"), Password: "hunter2"},
			wantErr: true,
		},
		{
			name: "password only",
			id:   SSHIdentity{Password: "hunter2"},
		},
		{
			name: "agent only",
			id:   SSHIdentity{AgentAvailable: true},
		},
		{
			name:    "nothing to authenticate with",
			id:      SSHIdentity{},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckSSHIdentity(tc.id)
			if tc.wantErr && err == nil {
				t.Fatal("CheckSSHIdentity() reported no error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("CheckSSHIdentity() error = %v", err)
			}
		})
	}
}

// TestCheckSSHIdentityNamesTheFields pins that the error says which profile
// fields to set, since that is the fix the user has to make.
func TestCheckSSHIdentityNamesTheFields(t *testing.T) {
	err := CheckSSHIdentity(SSHIdentity{})
	if err == nil {
		t.Fatal("CheckSSHIdentity() reported no error")
	}
	for _, want := range []string{"key_path", "password", "SSH_AUTH_SOCK"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// TestCheckSSHIdentityExpandsThePath pins that a path written with an
// environment variable resolves before it is checked.
func TestCheckSSHIdentityExpandsThePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "id_rsa"), []byte("key"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	t.Setenv("PGTRANSFER_TEST_KEYDIR", dir)

	if err := CheckSSHIdentity(SSHIdentity{KeyPath: "$PGTRANSFER_TEST_KEYDIR/id_rsa"}); err != nil {
		t.Fatalf("CheckSSHIdentity() with an expanded path = %v", err)
	}
}

// TestCheckProfiles pins that only tunnelled profiles are checked, and that the
// error names which end of the transfer is at fault.
func TestCheckProfiles(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(key, []byte("not a real key"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	t.Run("direct profiles are not checked", func(t *testing.T) {
		// No identity at all, but SSH is off, so there is nothing to check.
		if err := CheckProfiles(
			ProfileSSH{Name: "src"},
			ProfileSSH{Name: "dst"},
		); err != nil {
			t.Errorf("CheckProfiles() with direct profiles = %v", err)
		}
	})

	t.Run("the passing end is skipped", func(t *testing.T) {
		err := CheckProfiles(
			ProfileSSH{Name: "src", Enabled: true, KeyPath: key},
			ProfileSSH{Name: "dst"}, // direct
		)
		if err != nil {
			t.Errorf("CheckProfiles() = %v", err)
		}
	})

	t.Run("a tunnelled end with a bad key is an error naming it", func(t *testing.T) {
		err := CheckProfiles(
			ProfileSSH{Name: "src"}, // direct
			ProfileSSH{Name: "dst", Enabled: true, KeyPath: filepath.Join(dir, "absent")},
		)
		if err == nil {
			t.Fatal("CheckProfiles() reported no error")
		}
		if !strings.Contains(err.Error(), `"dst"`) {
			t.Errorf("error %q does not name the profile", err)
		}
	})

	t.Run("both ends are checked, source first", func(t *testing.T) {
		err := CheckProfiles(
			ProfileSSH{Name: "src", Enabled: true, KeyPath: filepath.Join(dir, "absent-src")},
			ProfileSSH{Name: "dst", Enabled: true, KeyPath: filepath.Join(dir, "absent-dst")},
		)
		if err == nil {
			t.Fatal("CheckProfiles() reported no error")
		}
		if !strings.Contains(err.Error(), "src") {
			t.Errorf("error %q does not report the source end first", err)
		}
	})
}

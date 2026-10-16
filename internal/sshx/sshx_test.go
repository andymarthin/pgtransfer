package sshx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildAuthMethods_NoMethods(t *testing.T) {
	// Ensure no ssh-agent is picked up by the test environment.
	t.Setenv("SSH_AUTH_SOCK", "")

	_, err := BuildAuthMethods(Config{})
	if err == nil {
		t.Fatalf("expected error when no auth method is configured")
	}
}

func TestBuildAuthMethods_Password(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	methods, err := BuildAuthMethods(Config{Password: "secret"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected 1 auth method, got %d", len(methods))
	}
}

func TestBuildAuthMethods_InvalidKeyPath(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	_, err := BuildAuthMethods(Config{
		KeyPath:  filepath.Join(t.TempDir(), "does-not-exist"),
		Password: "secret",
	})
	if err == nil {
		t.Fatalf("expected error for missing key file")
	}
}

func TestBuildAuthMethods_InvalidKeyContent(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	path := filepath.Join(t.TempDir(), "bad.key")
	if err := os.WriteFile(path, []byte("not a valid private key"), 0o600); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	_, err := BuildAuthMethods(Config{KeyPath: path})
	if err == nil {
		t.Fatalf("expected error for invalid key content")
	}
}

func TestBuildAuthMethods_KeyPathEnvExpansion(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.key")
	if err := os.WriteFile(path, []byte("garbage"), 0o600); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}
	t.Setenv("TEST_KEY_DIR", dir)

	// Expansion should resolve $TEST_KEY_DIR/bad.key and then fail on content,
	// proving the path was expanded (otherwise it would fail on file-not-found).
	if _, err := BuildAuthMethods(Config{KeyPath: "$TEST_KEY_DIR/bad.key"}); err == nil {
		t.Fatalf("expected error for invalid key content")
	}
}

func TestReadPrivateKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "k")
	want := []byte("keydata")
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	got, err := ReadPrivateKey(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("ReadPrivateKey = %q, want %q", got, want)
	}
}

func TestReadPrivateKey_Missing(t *testing.T) {
	if _, err := ReadPrivateKey(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatalf("expected error for missing file")
	}
}

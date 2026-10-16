package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestCommand builds a cobra command carrying the same connection flags as
// the real subcommands, so hasConnectionFlags/hasTestConnectionFlags can be
// exercised in isolation.
func newConnectionFlagCommand(t *testing.T) *cobra.Command {
	t.Helper()
	c := &cobra.Command{Use: "x"}
	f := c.Flags()
	f.String("user", "", "")
	f.String("password", "", "")
	f.String("host", "localhost", "")
	f.Int("port", 5432, "")
	f.String("database", "", "")
	f.String("db", "", "")
	f.String("sslmode", "disable", "")
	f.String("ssh-host", "", "")
	f.String("ssh-user", "", "")
	f.String("ssh-key", "", "")
	f.String("ssh-passphrase", "", "")
	f.String("ssh-password", "", "")
	f.Int("ssh-port", 22, "")
	f.Int("ssh-timeout", 10, "")
	return c
}

func TestHasTestConnectionFlags(t *testing.T) {
	t.Run("no flags changed", func(t *testing.T) {
		if hasTestConnectionFlags(newConnectionFlagCommand(t)) {
			t.Fatalf("expected false when no flags are set")
		}
	})

	t.Run("database flag counts as changed", func(t *testing.T) {
		c := newConnectionFlagCommand(t)
		if err := c.ParseFlags([]string{"--database", "app"}); err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if !hasTestConnectionFlags(c) {
			t.Fatalf("expected true after --database was provided")
		}
	})
}

func TestPrintVersion(t *testing.T) {
	out := captureStdout(t, func() { PrintVersion() })
	if !strings.Contains(out, "pgtransfer") {
		t.Fatalf("expected version output to mention pgtransfer, got %q", out)
	}
	if !strings.Contains(out, "Commit:") || !strings.Contains(out, "Built:") {
		t.Fatalf("expected commit and build info, got %q", out)
	}
}

// TestCommandTree_HelpRenders walks the whole command tree and renders help for
// every command. This exercises the init() wiring that registers subcommands
// and flags, catching nil commands or duplicate/panicking flag definitions.
func TestCommandTree_HelpRenders(t *testing.T) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		var buf bytes.Buffer
		c.SetOut(&buf)
		c.SetErr(&buf)

		if err := c.Help(); err != nil {
			t.Fatalf("help failed for %q: %v", c.CommandPath(), err)
		}
		if buf.Len() == 0 {
			t.Fatalf("expected non-empty help for %q", c.CommandPath())
		}

		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
}

func TestRootCmd_SubcommandsRegistered(t *testing.T) {
	want := map[string]bool{
		"export":          false,
		"import":          false,
		"migrate":         false,
		"profile":         false,
		"test-connection": false,
		"version":         false,
	}
	for _, c := range rootCmd.Commands() {
		name := c.Name()
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("expected subcommand %q to be registered on the root command", name)
		}
	}
}

func TestRootCmd_VersionTemplate(t *testing.T) {
	if rootCmd.Version == "" {
		t.Fatalf("expected root command version to be set")
	}
}

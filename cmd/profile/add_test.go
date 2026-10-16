package profile

import (
	"testing"

	"github.com/spf13/cobra"
)

// newConnectionFlagCommand builds a cobra command carrying the same connection
// flags as the real `profile add` command, so hasConnectionFlags can be
// exercised in isolation.
func newConnectionFlagCommand() *cobra.Command {
	c := &cobra.Command{Use: "add"}
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

func TestHasConnectionFlags(t *testing.T) {
	t.Run("no flags changed", func(t *testing.T) {
		if hasConnectionFlags(newConnectionFlagCommand()) {
			t.Fatalf("expected false when no flags are set")
		}
	})

	t.Run("database flag counts as changed", func(t *testing.T) {
		c := newConnectionFlagCommand()
		if err := c.ParseFlags([]string{"--user", "alice"}); err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if !hasConnectionFlags(c) {
			t.Fatalf("expected true after --user was provided")
		}
	})

	t.Run("ssh flag counts as changed", func(t *testing.T) {
		c := newConnectionFlagCommand()
		if err := c.ParseFlags([]string{"--ssh-host", "bastion"}); err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if !hasConnectionFlags(c) {
			t.Fatalf("expected true after --ssh-host was provided")
		}
	})

	t.Run("port flag counts as changed", func(t *testing.T) {
		c := newConnectionFlagCommand()
		if err := c.ParseFlags([]string{"--port", "6543"}); err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if !hasConnectionFlags(c) {
			t.Fatalf("expected true after --port was provided")
		}
	})
}

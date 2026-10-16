package migrate

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

// TestMigrateCmd_HelpRenders walks the migrate command tree, rendering help for
// every registered command. This exercises the init() wiring that attaches
// subcommands and flags.
func TestMigrateCmd_HelpRenders(t *testing.T) {
	walkHelp(t, MigrateCmd)
}

func TestMigrateCmd_Subcommands(t *testing.T) {
	want := map[string]bool{"database": false, "rollback": false}
	for _, c := range MigrateCmd.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("expected migrate subcommand %q to be registered", name)
		}
	}
}

func walkHelp(t *testing.T, root *cobra.Command) {
	t.Helper()
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
	walk(root)
}

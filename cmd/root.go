package cmd

import (
	"strings"

	"github.com/andymarthin/pgtransfer/cmd/export"
	importcmd "github.com/andymarthin/pgtransfer/cmd/import"
	"github.com/andymarthin/pgtransfer/cmd/migrate"
	"github.com/andymarthin/pgtransfer/cmd/profile"
	"github.com/andymarthin/pgtransfer/internal/output"
	"github.com/spf13/cobra"
)

// jsonOutput is bound to the global --json flag.
var jsonOutput bool

var rootCmd = &cobra.Command{
	Use:   "pgtransfer",
	Short: "Transfer PostgreSQL data between databases or CSV files",
	Long: `pgtransfer transfers PostgreSQL data between databases or CSV files,
with support for SSH tunnels and saved connection profiles.

For scripts and AI agents, pass --json (or set PGTRANSFER_JSON=1) to receive a
single structured JSON object on stdout instead of human-friendly text.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Resolve output mode from the flag / environment before any command
		// body runs, so every command emits a consistent format.
		output.Configure(jsonOutput)
	},
}

func Execute() error {
	err := rootCmd.Execute()
	if err != nil {
		if output.IsJSON() {
			// Command bodies emit their own structured errors, but cobra's
			// own failures (unknown command, bad flags, wrong arg count) reach
			// here without one. Emit a generic structured error so agents
			// always get parseable output.
			output.Fail(strings.TrimPrefix(rootCmd.CommandPath(), "pgtransfer "), "usage_error", "%v", err)
		} else {
			output.PrintErrorLine(err)
		}
	}
	return err
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false,
		"Emit machine-readable JSON instead of human-friendly text (also honours PGTRANSFER_JSON=1)")

	rootCmd.AddCommand(profile.ProfileCmd)
	rootCmd.AddCommand(testConnectionCmd)
	rootCmd.AddCommand(export.ExportCmd)
	rootCmd.AddCommand(importcmd.ImportCmd)
	rootCmd.AddCommand(migrate.MigrateCmd)
}

package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/andymarthin/pgtransfer/internal/mcpserver"
	"github.com/andymarthin/pgtransfer/internal/output"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run a Model Context Protocol (MCP) server over stdio",
	Long: `Run pgtransfer as a Model Context Protocol (MCP) server.

The server communicates with an MCP host (such as Claude Desktop, Cursor, or an
agent runtime) over stdin/stdout and exposes pgtransfer's capabilities as typed
tools: list_profiles, test_connection, export_csv, import_csv, export_dump,
import_dump and migrate_database.

Add it to an MCP host like this (Claude Desktop example):

  {
    "mcpServers": {
      "pgtransfer": { "command": "pgtransfer", "args": ["mcp"] }
    }
  }

Because stdout carries the MCP protocol, all diagnostics go to stderr.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// The MCP protocol owns stdout: never write JSON envelopes or banners
		// there, and keep logs on stderr.
		output.SetJSON(false)

		mcpserver.Version = Version

		fmt.Fprintln(os.Stderr, "pgtransfer MCP server listening on stdio")

		// Redirect stray library output away from stdout, and keep a handle on
		// the real stdout for the protocol stream.
		realStdout := output.EnableMCPMode()

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		if err := mcpserver.Run(ctx, realStdout); err != nil {
			return fmt.Errorf("mcp server stopped: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

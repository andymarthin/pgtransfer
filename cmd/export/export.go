package export

import (
	"github.com/spf13/cobra"
)

// ExportCmd represents the export command
var ExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export data from PostgreSQL database",
	Long: `Export data from PostgreSQL database to various formats including CSV files and SQL dump files.

Examples:
  # Export table to CSV
  pgtransfer export csv myprofile public.users users.csv

  # Export database to SQL dump
  pgtransfer export dump myprofile mydatabase backup.sql

  # Export with custom query to CSV
  pgtransfer export csv myprofile --query "SELECT * FROM users WHERE active = true" active_users.csv`,
}

func init() {
	// Add subcommands
	ExportCmd.AddCommand(csvCmd)
	ExportCmd.AddCommand(dumpCmd)
}

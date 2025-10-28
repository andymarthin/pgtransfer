package importcmd

import (
	"github.com/spf13/cobra"
)

// ImportCmd represents the import command
var ImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import data into PostgreSQL database",
	Long: `Import data into PostgreSQL database from various formats including CSV files and SQL dump files.

Examples:
  # Import CSV file into table
  pgtransfer import csv myprofile public.users users.csv

  # Import CSV with custom batch size
  pgtransfer import csv myprofile public.products products.csv --batch-size 1000

  # Import CSV with headers
  pgtransfer import csv myprofile public.customers customers.csv --headers`,
}

func init() {
	// Add subcommands
	ImportCmd.AddCommand(csvCmd)
	ImportCmd.AddCommand(dumpCmd)
}

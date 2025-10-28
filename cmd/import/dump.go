package importcmd

import (
	"fmt"
	"os"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/io"
	"github.com/spf13/cobra"
)

var dumpCmd = &cobra.Command{
	Use:   "dump [profile] [input-file]",
	Short: "Import PostgreSQL database from SQL dump file",
	Long: `Import PostgreSQL database from SQL dump file using pg_restore or psql.

This command restores a database from a dump file created by pg_dump. It supports various
dump formats including plain SQL, custom format, tar format, and directory format.`,
	Example: `  # Import from SQL dump file
  pgtransfer import dump myprofile backup.sql

  # Import from custom format dump
  pgtransfer import dump myprofile backup.dump

  # Import with clean first (drop existing objects)
  pgtransfer import dump myprofile backup.sql --clean

  # Import with verbose output and timeout
  pgtransfer import dump myprofile backup.sql --verbose --timeout 300`,
	Args: cobra.ExactArgs(2),
	RunE: runDumpImport,
}

func runDumpImport(cmd *cobra.Command, args []string) error {
	profileName := args[0]
	inputFile := args[1]

	// Check if input file exists
	if _, err := os.Stat(inputFile); err != nil {
		return fmt.Errorf("input file not found: %w", err)
	}

	// Load profile
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	profile, exists := cfg.Profiles[profileName]
	if !exists {
		return fmt.Errorf("profile '%s' not found", profileName)
	}

	// Build database URL
	dbURL := config.BuildDSN(profile)

	// Perform the restore
	return io.RestoreDatabaseSmart(dbURL, inputFile)
}
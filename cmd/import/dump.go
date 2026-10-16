package importcmd

import (
	"fmt"
	"os"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/io"
	"github.com/andymarthin/pgtransfer/internal/utils"
	"github.com/spf13/cobra"
)

var (
	dumpImportClean   bool
	dumpImportVerbose bool
	dumpImportTimeout int
	dumpImportStrict  bool
)

var dumpCmd = &cobra.Command{
	Use:   "dump [profile] [input-file]",
	Short: "Import PostgreSQL database from SQL dump file",
	Long: `Import PostgreSQL database from SQL dump file using pg_restore or psql.

This command restores a database from a dump file created by pg_dump. It supports various
dump formats including plain SQL, custom format, tar format, and directory format.

By default, benign errors caused by client/server version differences (for
example an "unrecognized configuration parameter" emitted by a newer pg_dump)
are tolerated so the restore can complete. Use --strict to make any restore
error fatal.`,
	Example: `  # Import from SQL dump file
  pgtransfer import dump myprofile backup.sql

  # Import from custom format dump
  pgtransfer import dump myprofile backup.dump

  # Import with clean first (drop existing objects)
  pgtransfer import dump myprofile backup.sql --clean

  # Import with verbose output and timeout
  pgtransfer import dump myprofile backup.sql --verbose --timeout 300

  # Import and stop on the first error
  pgtransfer import dump myprofile backup.sql --strict`,
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

	utils.PrintInfo(cmd, "Importing database dump using profile '%s'...", profileName)

	// Use connection-aware restore function that supports SSH tunnels.
	options := &io.RestoreOptions{
		Clean:   dumpImportClean,
		Timeout: dumpImportTimeout,
		Strict:  dumpImportStrict,
	}
	return io.RestoreDatabaseWithConnectionAndOptions(profile, inputFile, options)
}

func init() {
	dumpCmd.Flags().BoolVar(&dumpImportClean, "clean", false, "Drop existing objects before restoring (pg_restore only)")
	dumpCmd.Flags().BoolVar(&dumpImportVerbose, "verbose", false, "Enable verbose output")
	dumpCmd.Flags().IntVar(&dumpImportTimeout, "timeout", 0, "Restore timeout in seconds (0 = no timeout)")
	dumpCmd.Flags().BoolVar(&dumpImportStrict, "strict", false, "Stop and fail on the first restore error")
}

package migrate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/io"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback [target-profile] [backup-file]",
	Short: "Rollback a database migration using a backup file",
	Long: `Rollback a database migration by restoring from a backup file.

This command restores a target database from a previously created backup file,
effectively rolling back any changes made during a migration.`,
	Example: `  # Rollback using a specific backup file
  pgtransfer migrate rollback mydb /path/to/backup.sql

  # Rollback with verbose output
  pgtransfer migrate rollback mydb /path/to/backup.sql --verbose

  # Rollback with timeout
  pgtransfer migrate rollback mydb /path/to/backup.sql --timeout 300`,
	Args: cobra.ExactArgs(2),
	RunE: runRollback,
}

var (
	rollbackVerbose bool
	rollbackTimeout int
)

func init() {
	rollbackCmd.Flags().BoolVarP(&rollbackVerbose, "verbose", "v", false, "Enable verbose output")
	rollbackCmd.Flags().IntVar(&rollbackTimeout, "timeout", 0, "Timeout in seconds (0 = no timeout)")
}

func runRollback(cmd *cobra.Command, args []string) error {
	targetProfile := args[0]
	backupFile := args[1]

	// Validate backup file exists
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		return fmt.Errorf("backup file does not exist: %s", backupFile)
	}

	// Load configuration
	configFile, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Find target profile
	var targetConfig *config.Profile
	for _, profile := range configFile.Profiles {
		if profile.Name == targetProfile {
			targetConfig = &profile
			break
		}
	}

	if targetConfig == nil {
		return fmt.Errorf("target profile '%s' not found", targetProfile)
	}

	if rollbackVerbose {
		fmt.Printf("🔄 Starting rollback operation...\n")
		fmt.Printf("ℹ️  Target profile: %s\n", targetProfile)
		fmt.Printf("ℹ️  Backup file: %s\n", backupFile)
	}

	// Create rollback options
	opts := &io.MigrationOptions{
		TargetProfile: *targetConfig,
		Verbose:       rollbackVerbose,
		Timeout:       rollbackTimeout,
	}

	// Perform rollback
	if err := performRollbackOperation(opts, backupFile); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	fmt.Printf("✅ Rollback completed successfully\n")
	return nil
}

func performRollbackOperation(opts *io.MigrationOptions, backupFile string) error {
	if opts.Verbose {
		fmt.Printf("🔄 Restoring database from backup...\n")
	}

	// Create progress bar for rollback
	rollbackBar := io.NewProgressBarWithTimer(0, fmt.Sprintf("Rolling back from %s", filepath.Base(backupFile)))

	// Restore from backup
	targetURL := config.BuildDSN(opts.TargetProfile)
	if err := io.RestoreDatabaseSmart(targetURL, backupFile); err != nil {
		rollbackBar.Finish()
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	rollbackBar.Finish()

	if opts.Verbose {
		fmt.Printf("✅ Database restored successfully from backup\n")
	}

	return nil
}

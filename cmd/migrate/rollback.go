package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/io"
	"github.com/andymarthin/pgtransfer/internal/log"
	"github.com/andymarthin/pgtransfer/internal/output"
	"github.com/andymarthin/pgtransfer/internal/preflight"
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
  pgtransfer migrate rollback mydb /path/to/backup.sql --timeout 300

  # Fail on the first restore error instead of tolerating version skew
  pgtransfer migrate rollback mydb /path/to/backup.sql --strict`,
	Args: cobra.ExactArgs(2),
	RunE: runRollback,
}

var (
	rollbackVerbose bool
	rollbackTimeout int
	rollbackStrict  bool
)

func init() {
	rollbackCmd.Flags().BoolVarP(&rollbackVerbose, "verbose", "v", false, "Enable verbose output")
	rollbackCmd.Flags().IntVar(&rollbackTimeout, "timeout", 0, "Timeout in seconds (0 = no timeout)")
	rollbackCmd.Flags().BoolVar(&rollbackStrict, "strict", false, "Abort the rollback on the first restore error")
}

func runRollback(cmd *cobra.Command, args []string) error {
	start := time.Now()
	targetProfile := args[0]
	backupFile := args[1]

	// Validate backup file exists
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		return fmt.Errorf("backup file does not exist: %s", backupFile)
	}

	// A rollback restores an archive: pg_restore for a custom, tar or
	// directory dump, psql for a plain SQL file. Which one is read from the
	// file, so check for the one the restore will actually run.
	tools := []preflight.Tool{preflight.PgRestore}
	if plain, err := io.IsPlainTextDump(backupFile); err == nil && plain {
		tools = []preflight.Tool{preflight.Psql}
	}
	if err := preflight.Check(tools...); err != nil {
		if output.IsJSON() {
			return output.Fail("migrate rollback", "missing_tool", "%v", err)
		}
		return err
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
		Strict:        rollbackStrict,
	}

	// Perform rollback
	if err := performRollbackOperation(opts, backupFile); err != nil {
		log.Failure("migrate rollback", targetProfile, err.Error(), start)
		if output.IsJSON() {
			return output.Fail("migrate rollback", "rollback_failed", "rollback failed: %v", err)
		}
		return fmt.Errorf("rollback failed: %w", err)
	}

	if output.IsJSON() {
		output.Success("migrate rollback", map[string]interface{}{
			"profile":     targetProfile,
			"backup_file": backupFile,
		})
		return nil
	}

	log.Success("migrate rollback", targetProfile, fmt.Sprintf("Rolled back from %s", filepath.Base(backupFile)), start)
	fmt.Printf("✅ Rollback completed successfully\n")
	return nil
}

// restoreOptionsForRollback derives the restore options for a rollback.
//
// Strict has to reach the restore, otherwise a rollback asked to stop on the
// first error would tolerate every error exactly like a default restore, and
// leave the target half-rolled-back while reporting success.
func restoreOptionsForRollback(opts *io.MigrationOptions) *io.RestoreOptions {
	return &io.RestoreOptions{
		Timeout: opts.Timeout,
		Strict:  opts.Strict,
		Verbose: opts.Verbose,
	}
}

func performRollbackOperation(opts *io.MigrationOptions, backupFile string) error {
	if opts.Verbose {
		fmt.Printf("🔄 Restoring database from backup...\n")
	}

	// Create progress bar for rollback
	rollbackBar := io.NewProgressBarWithTimer(0, fmt.Sprintf("Rolling back from %s", filepath.Base(backupFile)))

	// Restore from backup.
	targetURL := config.BuildDSN(opts.TargetProfile)
	restoreOpts := restoreOptionsForRollback(opts)
	if err := io.RestoreDatabaseSmartWithOptions(targetURL, backupFile, restoreOpts); err != nil {
		rollbackBar.Finish()
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	rollbackBar.Finish()

	if opts.Verbose {
		fmt.Printf("✅ Database restored successfully from backup\n")
	}

	return nil
}

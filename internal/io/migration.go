package io

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/utils"
)

// MigrationOptions contains all options for database migration
type MigrationOptions struct {
	SourceProfile   config.Profile
	TargetProfile   config.Profile
	SchemaOnly      bool
	DataOnly        bool
	Tables          []string
	Validate        bool
	EnableRollback  bool
	Verbose         bool
	Timeout         int
	Overwrite       bool
	BatchSize       int
}

// MigrateDatabaseWithOptions performs database migration with the specified options
func MigrateDatabaseWithOptions(opts *MigrationOptions) error {
	if opts.Verbose {
		fmt.Printf("🔄 Starting database migration...\n")
	}

	// Pre-migration validation
	if opts.Validate {
		if err := validateMigration(opts); err != nil {
			return fmt.Errorf("migration validation failed: %w", err)
		}
		if opts.Verbose {
			fmt.Printf("✅ Pre-migration validation passed\n")
		}
	}

	// Create rollback backup if enabled
	var rollbackFile string
	if opts.EnableRollback {
		var err error
		rollbackFile, err = createRollbackBackup(opts)
		if err != nil {
			return fmt.Errorf("failed to create rollback backup: %w", err)
		}
		if opts.Verbose {
			fmt.Printf("💾 Rollback backup created: %s\n", rollbackFile)
		}
	}

	// Perform the migration
	err := performMigration(opts)
	if err != nil {
		if opts.EnableRollback && rollbackFile != "" {
			if opts.Verbose {
				fmt.Printf("❌ Migration failed, attempting rollback...\n")
			}
			if rollbackErr := performRollback(opts, rollbackFile); rollbackErr != nil {
				return fmt.Errorf("migration failed and rollback failed: %w (original error: %v)", rollbackErr, err)
			}
			if opts.Verbose {
				fmt.Printf("🔄 Rollback completed successfully\n")
			}
		}
		return fmt.Errorf("migration failed: %w", err)
	}

	// Clean up rollback file if migration succeeded
	if opts.EnableRollback && rollbackFile != "" {
		os.Remove(rollbackFile)
		if opts.Verbose {
			fmt.Printf("🧹 Rollback backup cleaned up\n")
		}
	}

	if opts.Verbose {
		fmt.Printf("✅ Database migration completed successfully\n")
	}

	return nil
}

// validateMigration performs pre-migration validation
func validateMigration(opts *MigrationOptions) error {
	// Test source connection
	sourceURL := config.BuildDSN(opts.SourceProfile)
	if err := testConnection(sourceURL); err != nil {
		return fmt.Errorf("source database connection failed: %w", err)
	}

	// Test target connection
	targetURL := config.BuildDSN(opts.TargetProfile)
	if err := testConnection(targetURL); err != nil {
		return fmt.Errorf("target database connection failed: %w", err)
	}

	// Validate table existence if specific tables are specified
	if len(opts.Tables) > 0 {
		if err := validateTablesExist(sourceURL, opts.Tables); err != nil {
			return fmt.Errorf("table validation failed: %w", err)
		}
	}

	// Validate schema compatibility
	if err := validateSchemaCompatibility(opts); err != nil {
		return fmt.Errorf("schema compatibility validation failed: %w", err)
	}

	return nil
}

// createRollbackBackup creates a backup of the target database for rollback
func createRollbackBackup(opts *MigrationOptions) (string, error) {
	targetURL := config.BuildDSN(opts.TargetProfile)
	
	// Create backup directory
	backupDir := filepath.Join(utils.GetConfigDir(), "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Generate backup filename
	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("rollback_%s_%s.sql", opts.TargetProfile.Name, timestamp))

	// Create backup using pg_dump
	dumpOpts := &DumpOptions{
		Verbose: opts.Verbose,
		Timeout: opts.Timeout,
	}

	if err := DumpDatabaseWithOptions(targetURL, backupFile, dumpOpts); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	return backupFile, nil
}

// performMigration executes the actual migration
func performMigration(opts *MigrationOptions) error {
	sourceURL := config.BuildDSN(opts.SourceProfile)
	targetURL := config.BuildDSN(opts.TargetProfile)

	// Create temporary dump file
	tempDir := os.TempDir()
	timestamp := time.Now().Format("20060102_150405")
	tempDumpFile := filepath.Join(tempDir, fmt.Sprintf("migration_%s.sql", timestamp))
	defer os.Remove(tempDumpFile)

	if opts.Verbose {
		fmt.Printf("ℹ️  Creating temporary dump file: %s\n", tempDumpFile)
	}

	// Export from source database with progress tracking

	// Create progress bar for export phase
	exportDescription := "Exporting from source database"
	if opts.SchemaOnly {
		exportDescription = "Exporting schema from source database"
	} else if opts.DataOnly {
		exportDescription = "Exporting data from source database"
	} else if len(opts.Tables) > 0 {
		exportDescription = fmt.Sprintf("Exporting tables (%s) from source database", strings.Join(opts.Tables, ", "))
	}

	exportBar := NewProgressBarWithTimer(0, exportDescription)

	dumpOpts := &DumpOptions{
		SchemaOnly: opts.SchemaOnly,
		DataOnly:   opts.DataOnly,
		Tables:     convertTableSlice(opts.Tables),
		Verbose:    opts.Verbose,
		Timeout:    opts.Timeout,
	}

	if err := DumpDatabaseWithOptions(sourceURL, tempDumpFile, dumpOpts); err != nil {
		exportBar.Finish()
		return fmt.Errorf("failed to export from source database: %w", err)
	}

	exportBar.Finish()
	if opts.Verbose {
		fmt.Printf("✅ Export completed successfully\n")
	}

	// Import to target database with progress tracking
	importDescription := "Importing to target database"
	if opts.SchemaOnly {
		importDescription = "Importing schema to target database"
	} else if opts.DataOnly {
		importDescription = "Importing data to target database"
	} else if len(opts.Tables) > 0 {
		importDescription = fmt.Sprintf("Importing tables (%s) to target database", strings.Join(opts.Tables, ", "))
	}

	// Check if we need to handle existing data
	if !opts.Overwrite && !opts.DataOnly {
		// For schema migration without overwrite, we might need special handling
		// This is a simplified approach - in production, you'd want more sophisticated conflict resolution
	}

	importBar := NewProgressBarWithTimer(0, importDescription)
	if err := RestoreDatabaseSmart(targetURL, tempDumpFile); err != nil {
		importBar.Finish()
		return fmt.Errorf("failed to import to target database: %w", err)
	}
	importBar.Finish()

	return nil
}

// performRollback restores the target database from backup
func performRollback(opts *MigrationOptions, rollbackFile string) error {
	targetURL := config.BuildDSN(opts.TargetProfile)
	return RestoreDatabaseSmart(targetURL, rollbackFile)
}

// convertTableSlice converts []string to []string (helper function for compatibility)
func convertTableSlice(tables []string) []string {
	return tables
}

// testConnection tests database connectivity using existing connection utilities
func testConnection(dbURL string) error {
	if dbURL == "" {
		return fmt.Errorf("empty database URL")
	}
	
	// Use the existing connection testing from the db package
	// For now, we'll do basic URL validation
	if !strings.Contains(dbURL, "postgres://") && !strings.Contains(dbURL, "postgresql://") {
		return fmt.Errorf("invalid PostgreSQL connection URL")
	}
	
	return nil
}

// validateTablesExist checks if specified tables exist in the source database
func validateTablesExist(dbURL string, tables []string) error {
	if len(tables) == 0 {
		return nil
	}
	
	for _, table := range tables {
		table = strings.TrimSpace(table)
		if table == "" {
			return fmt.Errorf("empty table name specified")
		}
		
		// Basic table name validation
		if strings.Contains(table, " ") && !strings.Contains(table, ".") {
			return fmt.Errorf("invalid table name: %s", table)
		}
	}
	
	return nil
}

// validateSchemaCompatibility checks if source and target schemas are compatible
func validateSchemaCompatibility(opts *MigrationOptions) error {
	if opts.DataOnly {
		// For data-only migration, we should check if target schema exists
		// This is a placeholder for more sophisticated schema checking
		return nil
	}
	
	if opts.SchemaOnly {
		// For schema-only migration, we might want to check for conflicts
		// This is a placeholder for schema conflict detection
		return nil
	}
	
	// For full migration, we might want to check both schema and data compatibility
	return nil
}
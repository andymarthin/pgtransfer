package migrate

import (
	"fmt"
	"strings"
	"time"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/io"
	"github.com/andymarthin/pgtransfer/internal/log"
	"github.com/spf13/cobra"
)

var (
	// Migration options
	migrateSchemaOnly   bool
	migrateDataOnly     bool
	migrateTables       string
	migrateValidate     bool
	migrateEnableRollback bool
	migrateVerbose      bool
	migrateTimeout      int
	migrateOverwrite    bool
	migrateBatchSize    int
	
	// Database override options
	migrateSourceDatabase string
	migrateTargetDatabase string
)

// databaseCmd represents the database migration command
var databaseCmd = &cobra.Command{
	Use:   "database [source_profile] [target_profile] OR database [profile] --source-database [db] --target-database [db]",
	Short: "Migrate database from source to target",
	Long: `Migrate database from source to target.

This command performs a complete database migration including schema and data transfer.
You can use either two different profiles or the same profile with database overrides.

Mode 1: Different profiles
  pgtransfer migrate database source_profile target_profile

Mode 2: Same profile with database overrides
  pgtransfer migrate database profile --source-database source_db --target-database target_db

You can customize the migration with various options for schema-only, data-only, 
specific tables, validation, and rollback support.

Examples:
  # Full database migration with different profiles
  pgtransfer migrate database source_profile target_profile

  # Same profile with different databases
  pgtransfer migrate database myprofile --source-database prod_db --target-database staging_db

  # Schema-only migration with database override
  pgtransfer migrate database myprofile --source-database prod_db --target-database dev_db --schema-only

  # Data-only migration (assumes schema exists)
  pgtransfer migrate database source_profile target_profile --data-only

  # Migrate specific tables with same profile
  pgtransfer migrate database myprofile --source-database db1 --target-database db2 --tables "users,orders"

  # Migration with validation
  pgtransfer migrate database source_profile target_profile --validate

  # Migration with rollback support
  pgtransfer migrate database source_profile target_profile --enable-rollback`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runDatabaseMigration,
}

func runDatabaseMigration(cmd *cobra.Command, args []string) error {
	start := time.Now()
	
	// Validate mutually exclusive options
	if migrateSchemaOnly && migrateDataOnly {
		return fmt.Errorf("--schema-only and --data-only are mutually exclusive")
	}

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	var sourceProfile, targetProfile config.Profile
	var sourceProfileName, targetProfileName string
	
	// Determine migration mode
	if len(args) == 2 {
		// Mode 1: Two different profiles
		sourceProfileName = args[0]
		targetProfileName = args[1]
		
		// Validate profiles are different
		if sourceProfileName == targetProfileName {
			return fmt.Errorf("source and target profiles cannot be the same when using two profiles")
		}
		
		// Validate database override flags are not used
		if migrateSourceDatabase != "" || migrateTargetDatabase != "" {
			return fmt.Errorf("--source-database and --target-database flags cannot be used with two profiles")
		}
		
		// Load source profile
		var exists bool
		sourceProfile, exists = cfg.Profiles[sourceProfileName]
		if !exists {
			return fmt.Errorf("source profile '%s' not found", sourceProfileName)
		}

		// Load target profile
		targetProfile, exists = cfg.Profiles[targetProfileName]
		if !exists {
			return fmt.Errorf("target profile '%s' not found", targetProfileName)
		}
		
	} else if len(args) == 1 {
		// Mode 2: Same profile with database overrides
		profileName := args[0]
		
		// Validate database override flags are provided
		if migrateSourceDatabase == "" || migrateTargetDatabase == "" {
			return fmt.Errorf("--source-database and --target-database flags are required when using single profile")
		}
		
		// Validate databases are different
		if migrateSourceDatabase == migrateTargetDatabase {
			return fmt.Errorf("source and target databases cannot be the same")
		}
		
		// Load base profile
		baseProfile, exists := cfg.Profiles[profileName]
		if !exists {
			return fmt.Errorf("profile '%s' not found", profileName)
		}
		
		// Create source and target profiles with database overrides
		sourceProfile = baseProfile
		sourceProfile.Database = migrateSourceDatabase
		sourceProfileName = fmt.Sprintf("%s(%s)", profileName, migrateSourceDatabase)
		
		targetProfile = baseProfile
		targetProfile.Database = migrateTargetDatabase
		targetProfileName = fmt.Sprintf("%s(%s)", profileName, migrateTargetDatabase)
		
	} else {
		return fmt.Errorf("invalid number of arguments")
	}

	if migrateVerbose {
		fmt.Printf("ℹ️  Starting database migration from '%s' to '%s'...\n", sourceProfileName, targetProfileName)
	}

	// Parse tables if specified
	var tableList []string
	if migrateTables != "" {
		tableList = strings.Split(migrateTables, ",")
		for i, table := range tableList {
			tableList[i] = strings.TrimSpace(table)
		}
	}

	// Create migration options
	migrationOpts := &io.MigrationOptions{
		SourceProfile:   sourceProfile,
		TargetProfile:   targetProfile,
		SchemaOnly:      migrateSchemaOnly,
		DataOnly:        migrateDataOnly,
		Tables:          tableList,
		Validate:        migrateValidate,
		EnableRollback:  migrateEnableRollback,
		Verbose:         migrateVerbose,
		Timeout:         migrateTimeout,
		Overwrite:       migrateOverwrite,
		BatchSize:       migrateBatchSize,
	}

	// Perform migration
	err = io.MigrateDatabaseWithOptions(migrationOpts)
	if err != nil {
		log.Failure("migrate database", sourceProfileName, err.Error(), start)
		return fmt.Errorf("database migration failed: %w", err)
	}

	log.Success("migrate database", sourceProfileName, fmt.Sprintf("Successfully migrated to %s", targetProfileName), start)
	fmt.Printf("✅ Database migration completed successfully from '%s' to '%s'\n", sourceProfileName, targetProfileName)
	return nil
}

func init() {
	// Schema/Data options
	databaseCmd.Flags().BoolVar(&migrateSchemaOnly, "schema-only", false, "Migrate schema only (no data)")
	databaseCmd.Flags().BoolVar(&migrateDataOnly, "data-only", false, "Migrate data only (assumes schema exists)")
	
	// Table selection
	databaseCmd.Flags().StringVar(&migrateTables, "tables", "", "Comma-separated list of tables to migrate")
	
	// Database override options
	databaseCmd.Flags().StringVar(&migrateSourceDatabase, "source-database", "", "Override source database name (use with single profile)")
	databaseCmd.Flags().StringVar(&migrateTargetDatabase, "target-database", "", "Override target database name (use with single profile)")
	
	// Validation and safety
	databaseCmd.Flags().BoolVar(&migrateValidate, "validate", false, "Validate migration before execution")
	databaseCmd.Flags().BoolVar(&migrateEnableRollback, "enable-rollback", false, "Enable rollback support (creates backup)")
	databaseCmd.Flags().BoolVar(&migrateOverwrite, "overwrite", false, "Overwrite existing data in target database")
	
	// Performance and output
	databaseCmd.Flags().BoolVar(&migrateVerbose, "verbose", false, "Enable verbose output")
	databaseCmd.Flags().IntVar(&migrateTimeout, "timeout", 3600, "Migration timeout in seconds")
	databaseCmd.Flags().IntVar(&migrateBatchSize, "batch-size", 1000, "Batch size for data migration")
}
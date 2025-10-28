package export

import (
	"fmt"
	"os"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/io"
	"github.com/spf13/cobra"
)

var (
	dumpOverwrite     bool
	dumpFormat        string
	dumpCompress      bool
	dumpSchemaOnly    bool
	dumpDataOnly      bool
	dumpTables        []string
	dumpExcludeTables []string
	dumpSchema        string
	dumpVerbose       bool
	dumpTimeout       int
)

var dumpCmd = &cobra.Command{
	Use:   "dump [profile] [output-file]",
	Short: "Export PostgreSQL database to SQL dump file with advanced options",
	Long: `Export PostgreSQL database to SQL dump file using pg_dump with advanced options.

This command creates a database dump using the native PostgreSQL pg_dump utility with support for
various formats, compression, filtering, and other advanced features.`,
	Example: `  # Basic SQL dump
  pgtransfer export dump myprofile backup.sql

  # Custom format with compression
  pgtransfer export dump myprofile backup.dump --format custom --compress

  # Export specific table only
  pgtransfer export dump myprofile users_backup.sql --table users

  # Schema-only export
  pgtransfer export dump myprofile schema.sql --schema-only

  # Export with verbose output and timeout
  pgtransfer export dump myprofile backup.sql --verbose --timeout 300`,
	Args: cobra.ExactArgs(2),
	RunE: runDumpExport,
}

func runDumpExport(cmd *cobra.Command, args []string) error {
	profileName := args[0]
	outputFile := args[1]

	// Check if output file exists and handle overwrite
	if _, err := os.Stat(outputFile); err == nil && !dumpOverwrite {
		return fmt.Errorf("output file '%s' already exists. Use --overwrite to replace it", outputFile)
	}

	// Validate mutually exclusive options
	if dumpSchemaOnly && dumpDataOnly {
		return fmt.Errorf("--schema-only and --data-only are mutually exclusive")
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

	fmt.Printf("ℹ️  Creating database dump using profile '%s'...\n", profileName)

	// Check if any advanced options are used
	hasAdvancedOptions := dumpFormat != "" || dumpCompress || dumpSchemaOnly || dumpDataOnly ||
		len(dumpTables) > 0 || len(dumpExcludeTables) > 0 || dumpSchema != "" ||
		dumpVerbose || dumpTimeout > 0

	if hasAdvancedOptions {
		// Use advanced dump function with connection support (SSH/direct)
		options := &io.DumpOptions{
			Format:        dumpFormat,
			Compress:      dumpCompress,
			SchemaOnly:    dumpSchemaOnly,
			DataOnly:      dumpDataOnly,
			Tables:        dumpTables,
			ExcludeTables: dumpExcludeTables,
			Schema:        dumpSchema,
			Verbose:       dumpVerbose,
			Timeout:       dumpTimeout,
		}
		return io.DumpDatabaseWithConnectionAndOptions(profile, outputFile, options)
	} else {
		// Use simple dump function with connection support (SSH/direct)
		return io.DumpDatabaseWithConnection(profile, outputFile)
	}
}

func init() {
	dumpCmd.Flags().BoolVar(&dumpOverwrite, "overwrite", false, "Overwrite output file if it exists")

	// Format and compression options
	dumpCmd.Flags().StringVar(&dumpFormat, "format", "", "Output format: plain, custom, directory, tar (default: plain)")
	dumpCmd.Flags().BoolVar(&dumpCompress, "compress", false, "Enable compression (not available for plain format)")

	// Content filtering options
	dumpCmd.Flags().BoolVar(&dumpSchemaOnly, "schema-only", false, "Export schema only (no data)")
	dumpCmd.Flags().BoolVar(&dumpDataOnly, "data-only", false, "Export data only (no schema)")

	// Table and schema filtering
	dumpCmd.Flags().StringSliceVar(&dumpTables, "table", []string{}, "Include specific table(s) (can be used multiple times)")
	dumpCmd.Flags().StringSliceVar(&dumpExcludeTables, "exclude-table", []string{}, "Exclude specific table(s) (can be used multiple times)")
	dumpCmd.Flags().StringVar(&dumpSchema, "schema", "", "Export specific schema only")

	// Advanced options
	dumpCmd.Flags().BoolVar(&dumpVerbose, "verbose", false, "Enable verbose output")
	dumpCmd.Flags().IntVar(&dumpTimeout, "timeout", 0, "Command timeout in seconds (0 = no timeout)")
}

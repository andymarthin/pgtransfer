package importcmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/db"
	"github.com/andymarthin/pgtransfer/internal/io"
	"github.com/andymarthin/pgtransfer/internal/output"
	"github.com/spf13/cobra"
)

var (
	csvOverwrite bool
	csvHeaders   bool
	csvBatchSize int
	csvSchema    string
)

var csvCmd = &cobra.Command{
	Use:   "csv [profile] [table] [input-file]",
	Short: "Import CSV file data into PostgreSQL table",
	Long: `Import data from CSV file into PostgreSQL table with progress tracking and batch processing for better performance with large datasets.

This command reads a CSV file and imports the data into the specified PostgreSQL table. The CSV file should have column headers that match the table structure, or you can specify that the first row contains headers.

The table can be specified as just the table name (uses default schema) or as schema.table format.
Default schema is 'public' unless specified with --schema flag.

Batch processing helps with memory efficiency and performance when dealing with large datasets by processing records in configurable batch sizes.`,
	Example: `  # Import CSV file into table (uses public schema by default)
  pgtransfer import csv myprofile users users.csv

  # Import a header-less file: columns are matched by position
  pgtransfer import csv myprofile users users.csv --headers=false

  # Import with explicit schema
  pgtransfer import csv myprofile users users.csv --schema myschema

  # Import using schema.table format
  pgtransfer import csv myprofile public.users users.csv

  # Import with custom batch size (default: 20000)
  pgtransfer import csv myprofile products products.csv --batch-size 50000

  # Import with overwrite (truncate table first)
  pgtransfer import csv myprofile orders orders.csv --overwrite`,
	Args: cobra.ExactArgs(3),
	RunE: runCSVImport,
}

func runCSVImport(cmd *cobra.Command, args []string) error {
	profileName := args[0]
	rawTableName := args[1]
	inputFile := args[2]

	// Handle schema.table format or use schema flag
	var tableName string
	if strings.Contains(rawTableName, ".") {
		// Table name already includes schema (e.g., "public.users")
		tableName = rawTableName
	} else {
		// Use schema flag or default to "public"
		schema := csvSchema
		if schema == "" {
			schema = "public"
		}
		tableName = fmt.Sprintf("%s.%s", schema, rawTableName)
	}

	// Check if input file exists
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("input file '%s' does not exist", inputFile)
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

	if !output.IsJSON() {
		fmt.Printf("ℹ️  Connecting to database using profile '%s'...\n", profileName)
	}

	// Connect to database
	dbConn, err := db.Connect(profile)
	if err != nil {
		if output.IsJSON() {
			return output.Fail("import csv", "connection_failed", "failed to connect to database: %v", err)
		}
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer dbConn.Close()

	// Handle overwrite option
	if csvOverwrite {
		if !output.IsJSON() {
			fmt.Printf("⚠️  Truncating table '%s'...\n", tableName)
		}
		truncateSQL := fmt.Sprintf("TRUNCATE TABLE %s", io.QuoteTable(tableName))
		if _, err := dbConn.DB.Exec(truncateSQL); err != nil {
			if output.IsJSON() {
				return output.Fail("import csv", "truncate_failed", "failed to truncate table %q: %v", tableName, err)
			}
			return fmt.Errorf("failed to truncate table: %w", err)
		}
	}

	// Create CSV options with batch size and header handling
	options := &io.CSVOptions{
		BatchSize: csvBatchSize,
		HasHeader: csvHeaders,
	}

	// Import using batch processing (options carry batch size + header flag)
	if err := io.ImportCSVWithOptions(dbConn.DB, tableName, inputFile, options); err != nil {
		if output.IsJSON() {
			return output.Fail("import csv", "import_failed", "%v", err)
		}
		return err
	}

	if output.IsJSON() {
		output.Success("import csv", map[string]interface{}{
			"profile":    profileName,
			"table":      tableName,
			"input_file": inputFile,
			"truncated":  csvOverwrite,
			"headers":    csvHeaders,
		})
	}
	return nil
}

func init() {
	csvCmd.Flags().BoolVar(&csvOverwrite, "overwrite", false, "Truncate table before importing (removes all existing data)")
	csvCmd.Flags().BoolVar(&csvHeaders, "headers", true, "First row contains column headers (use --headers=false for a header-less file)")
	csvCmd.Flags().IntVar(&csvBatchSize, "batch-size", 20000, "Number of rows to process in each batch")
	csvCmd.Flags().StringVar(&csvSchema, "schema", "", "Database schema name (default: 'public')")
}

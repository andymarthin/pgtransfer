package export

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/db"
	"github.com/andymarthin/pgtransfer/internal/io"
	"github.com/spf13/cobra"
)

var (
	csvOverwrite bool
	csvQuery     string
	csvHeaders   bool
	csvBatchSize int
	csvSchema    string
)

var csvCmd = &cobra.Command{
	Use:   "csv [profile] [table-or-output-file] [output-file]",
	Short: "Export PostgreSQL data to CSV file",
	Long: `Export data from PostgreSQL database to CSV file.

This command can export a complete table or execute a custom query and export the results to CSV format with progress tracking and batch processing for better performance with large datasets.

Usage modes:
1. Table export: pgtransfer export csv [profile] [table] [output-file]
2. Query export: pgtransfer export csv [profile] [output-file] --query "SELECT ..."

The table can be specified as just the table name (uses default schema) or as schema.table format.
Default schema is 'public' unless specified with --schema flag.

Batch processing helps with memory efficiency and performance when dealing with large datasets.`,
	Example: `  # Export entire table (uses public schema by default)
  pgtransfer export csv myprofile users users.csv

  # Export table with explicit schema
  pgtransfer export csv myprofile users users.csv --schema myschema

  # Export table using schema.table format
  pgtransfer export csv myprofile public.users users.csv

  # Export with custom query
  pgtransfer export csv myprofile active_users.csv --query "SELECT * FROM users WHERE is_active = true"

  # Export with headers
  pgtransfer export csv myprofile users users.csv --headers

  # Export with custom batch size (default: 500)
  pgtransfer export csv myprofile products products.csv --batch-size 1000

  # Export with overwrite
  pgtransfer export csv myprofile products products.csv --overwrite`,
	Args: cobra.RangeArgs(2, 3),
	RunE: runCSVExport,
}

func runCSVExport(cmd *cobra.Command, args []string) error {
	profileName := args[0]
	var tableName, outputFile string

	// Determine mode based on arguments and flags
	if csvQuery != "" {
		// Query mode: profile + output-file + --query
		if len(args) != 2 {
			return fmt.Errorf("when using --query, provide: [profile] [output-file]")
		}
		outputFile = args[1]
	} else {
		// Table mode: profile + table + output-file
		if len(args) != 3 {
			return fmt.Errorf("when exporting table, provide: [profile] [table] [output-file]")
		}
		rawTableName := args[1]
		outputFile = args[2]
		
		// Handle schema.table format or use schema flag
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
	}

	// Check if output file exists and handle overwrite
	if _, err := os.Stat(outputFile); err == nil && !csvOverwrite {
		return fmt.Errorf("output file '%s' already exists. Use --overwrite to replace it", outputFile)
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

	fmt.Printf("ℹ️  Connecting to database using profile '%s'...\n", profileName)

	// Connect to database
	dbConn, err := db.Connect(profile)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer dbConn.Close()

	// Create CSV options with batch size
	options := &io.CSVOptions{
		BatchSize: csvBatchSize,
	}

	if csvQuery != "" {
		// Export using custom query
		fmt.Printf("ℹ️  Executing custom query...\n")
		return exportCSVWithQuery(dbConn.DB, csvQuery, outputFile, csvHeaders)
	} else {
		// Export using table name with batch processing
		if csvBatchSize == 500 {
			// Use default function for backward compatibility when using default batch size
			return io.ExportCSV(dbConn.DB, tableName, outputFile)
		} else {
			// Use batch-enabled function when custom batch size is specified
			return io.ExportCSVWithOptions(dbConn.DB, tableName, outputFile, options)
		}
	}
}

func exportCSVWithQuery(db *sql.DB, query, outputFile string, includeHeaders bool) error {
	// Create output file
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Execute query
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get column names: %w", err)
	}

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write headers if requested
	if includeHeaders {
		if err := writer.Write(columns); err != nil {
			return fmt.Errorf("failed to write headers: %w", err)
		}
	}

	// Prepare value holders
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	rowCount := 0
	startTime := time.Now()

	// Process rows
	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert values to strings
		record := make([]string, len(columns))
		for i, val := range values {
			record[i] = io.FormatCSVValue(val)
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}

		rowCount++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error during row iteration: %w", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("✅ ✅ Export completed successfully!\n")
	fmt.Printf("ℹ️  📊 Exported %d rows in %v\n", rowCount, duration.Round(time.Millisecond))
	fmt.Printf("ℹ️  📁 Output file: %s\n", outputFile)

	return nil
}

func init() {
	csvCmd.Flags().BoolVar(&csvOverwrite, "overwrite", false, "Overwrite output file if it exists")
	csvCmd.Flags().StringVar(&csvQuery, "query", "", "Custom SQL query to execute")
	csvCmd.Flags().BoolVar(&csvHeaders, "headers", false, "Include column headers in CSV output")
	csvCmd.Flags().IntVar(&csvBatchSize, "batch-size", 500, "Number of rows to process in each batch (default: 500)")
	csvCmd.Flags().StringVar(&csvSchema, "schema", "", "Database schema name (default: 'public')")
}
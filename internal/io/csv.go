package io

import (
	"database/sql"
	"database/sql/driver"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andymarthin/pgtransfer/internal/utils"
	"github.com/schollz/progressbar/v3"
)

// CSVOptions contains configuration for CSV operations
type CSVOptions struct {
	BatchSize int // Number of rows to process in each batch (default: 500)
}

// DefaultCSVOptions returns default CSV configuration
func DefaultCSVOptions() *CSVOptions {
	return &CSVOptions{
		BatchSize: 500,
	}
}

// formatCSVValue properly formats a value for CSV export
func FormatCSVValue(v interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case time.Time:
		// Format timestamps in ISO 8601 format without timezone
		if val.Hour() == 0 && val.Minute() == 0 && val.Second() == 0 && val.Nanosecond() == 0 {
			// Date only
			return val.Format("2006-01-02")
		}
		// Timestamp
		return val.Format("2006-01-02 15:04:05")
	case []byte:
		// Handle PostgreSQL numeric types that come as byte arrays
		str := string(val)
		// Check if it's a numeric value
		if strings.Contains(str, ".") || (len(str) > 0 && (str[0] >= '0' && str[0] <= '9' || str[0] == '-')) {
			return str
		}
		// Otherwise treat as string
		return str
	case driver.Valuer:
		// Handle custom types that implement driver.Valuer
		if driverVal, err := val.Value(); err == nil {
			return FormatCSVValue(driverVal)
		}
		return fmt.Sprintf("%v", val)
	case string:
		return val
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", val)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		// For any other type, convert to string and clean up
		str := fmt.Sprintf("%v", val)
		// Remove timezone information if present
		str = strings.Replace(str, " +0000 +0000", "", -1)
		// Clean up byte array representations like [54 51 57 52 50 46 48 48]
		if strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]") && strings.Contains(str, " ") {
			// This looks like a byte array representation, try to convert it
			parts := strings.Split(strings.Trim(str, "[]"), " ")
			var bytes []byte
			for _, part := range parts {
				if len(part) > 0 {
					var b int
					if _, err := fmt.Sscanf(part, "%d", &b); err == nil && b >= 0 && b <= 255 {
						bytes = append(bytes, byte(b))
					} else {
						// Not a valid byte array, return as is
						return str
					}
				}
			}
			return string(bytes)
		}
		return str
	}
}

// ExportCSV streams a PostgreSQL table to a CSV file with a live progress bar.
func ExportCSV(db *sql.DB, table, exportPath string) error {
	start := time.Now()

	if !strings.HasSuffix(exportPath, ".csv") {
		exportPath += ".csv"
	}

	if err := os.MkdirAll(filepath.Dir(exportPath), 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	utils.PrintInfo(nil, "Starting export of table '%s'...", table)

	// Count total rows for progress tracking
	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
	if err := db.QueryRow(countQuery).Scan(&total); err != nil {
		total = -1 // fallback if counting fails
	}

	query := fmt.Sprintf("SELECT * FROM %s", table)
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query table: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	file, err := os.Create(exportPath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write(cols); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	bar := NewProgressBarWithTimer(total, fmt.Sprintf("Exporting %s", table))

	values := make([]interface{}, len(cols))
	valuePtrs := make([]interface{}, len(cols))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	var written int64
	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("row scan failed: %w", err)
		}

		record := make([]string, len(cols))
		for i, v := range values {
			record[i] = FormatCSVValue(v)
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}

		written++
		bar.Add(1)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("failed writing CSV: %w", err)
	}

	duration := time.Since(start)
	utils.PrintSuccess(nil, "✅ Exported %d rows to %s", written, exportPath)
	utils.PrintInfo(nil, "🕒 Duration: %s", utils.FormatDuration(duration))
	return nil
}

// ExportCSVWithOptions exports a PostgreSQL table to CSV with batch processing for better memory efficiency
func ExportCSVWithOptions(db *sql.DB, table, exportPath string, options *CSVOptions) error {
	if options == nil {
		options = DefaultCSVOptions()
	}

	start := time.Now()

	if !strings.HasSuffix(exportPath, ".csv") {
		exportPath += ".csv"
	}

	if err := os.MkdirAll(filepath.Dir(exportPath), 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	utils.PrintInfo(nil, "Starting batch export of table '%s' (batch size: %d)...", table, options.BatchSize)

	// Count total rows for progress tracking
	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
	if err := db.QueryRow(countQuery).Scan(&total); err != nil {
		total = -1 // fallback if counting fails
	}

	file, err := os.Create(exportPath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Get column information
	query := fmt.Sprintf("SELECT * FROM %s LIMIT 1", table)
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query table for columns: %w", err)
	}

	cols, err := rows.Columns()
	if err != nil {
		rows.Close()
		return fmt.Errorf("failed to get columns: %w", err)
	}
	rows.Close()

	// Write CSV header
	if err := writer.Write(cols); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	bar := NewProgressBarWithTimer(total, fmt.Sprintf("Exporting %s", table))

	var written int64
	offset := int64(0)

	// Process data in batches
	for {
		batchQuery := fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d", table, options.BatchSize, offset)
		batchRows, err := db.Query(batchQuery)
		if err != nil {
			return fmt.Errorf("failed to query batch: %w", err)
		}

		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		batchCount := 0
		for batchRows.Next() {
			if err := batchRows.Scan(valuePtrs...); err != nil {
				batchRows.Close()
				return fmt.Errorf("row scan failed: %w", err)
			}

			record := make([]string, len(cols))
			for i, v := range values {
				record[i] = FormatCSVValue(v)
			}

			if err := writer.Write(record); err != nil {
				batchRows.Close()
				return fmt.Errorf("failed to write row: %w", err)
			}

			written++
			batchCount++
			bar.Add(1)
		}

		batchRows.Close()

		// If we got fewer rows than batch size, we're done
		if batchCount < options.BatchSize {
			break
		}

		offset += int64(options.BatchSize)

		// Flush periodically to avoid memory buildup
		writer.Flush()
		if err := writer.Error(); err != nil {
			return fmt.Errorf("failed writing CSV batch: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("failed writing CSV: %w", err)
	}

	duration := time.Since(start)
	utils.PrintSuccess(nil, "✅ Exported %d rows to %s (batch size: %d)", written, exportPath, options.BatchSize)
	utils.PrintInfo(nil, "🕒 Duration: %s", utils.FormatDuration(duration))
	return nil
}

// ImportCSV imports data from a CSV file into a PostgreSQL table.
func ImportCSV(db *sql.DB, table, importPath string) error {
	start := time.Now()

	utils.PrintInfo(nil, "Starting import from %s into table '%s'...", importPath, table)

	file, err := os.Open(importPath)
	if err != nil {
		return fmt.Errorf("failed to open CSV: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) < 2 {
		return fmt.Errorf("no data rows found in %s", importPath)
	}

	headers := records[0]
	bar := NewProgressBarWithTimer(int64(len(records)-1), fmt.Sprintf("Importing %s", table))

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	placeholders := make([]string, len(headers))
	for i := range headers {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(headers, ","),
		strings.Join(placeholders, ","),
	)

	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for i, row := range records[1:] {
		args := make([]interface{}, len(row))
		for j, v := range row {
			args[j] = v
		}
		if _, err := stmt.Exec(args...); err != nil {
			tx.Rollback()
			return fmt.Errorf("insert failed on row %d: %w", i+1, err)
		}
		bar.Add(1)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	duration := time.Since(start)
	utils.PrintSuccess(nil, "✅ Imported %d rows from %s", len(records)-1, importPath)
	utils.PrintInfo(nil, "🕒 Duration: %s", utils.FormatDuration(duration))
	return nil
}

// ImportCSVWithOptions imports data from a CSV file with batch processing for better performance
func ImportCSVWithOptions(db *sql.DB, table, importPath string, options *CSVOptions) error {
	if options == nil {
		options = DefaultCSVOptions()
	}

	start := time.Now()

	utils.PrintInfo(nil, "Starting batch import from %s into table '%s' (batch size: %d)...", importPath, table, options.BatchSize)

	file, err := os.Open(importPath)
	if err != nil {
		return fmt.Errorf("failed to open CSV: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read header
	headers, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Count total rows for progress tracking
	tempFile, err := os.Open(importPath)
	if err != nil {
		return fmt.Errorf("failed to open CSV for counting: %w", err)
	}
	tempReader := csv.NewReader(tempFile)
	records, err := tempReader.ReadAll()
	tempFile.Close()
	if err != nil {
		return fmt.Errorf("failed to count CSV rows: %w", err)
	}

	totalRows := int64(len(records) - 1) // Exclude header
	if totalRows <= 0 {
		return fmt.Errorf("no data rows found in %s", importPath)
	}

	bar := NewProgressBarWithTimer(totalRows, fmt.Sprintf("Importing %s", table))

	placeholders := make([]string, len(headers))
	for i := range headers {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(headers, ","),
		strings.Join(placeholders, ","),
	)

	var imported int64
	batch := make([][]string, 0, options.BatchSize)

	// Process CSV in batches
	for {
		record, err := reader.Read()
		if err != nil {
			if err.Error() == "EOF" {
				// Process final batch if any
				if len(batch) > 0 {
					if err := processBatch(db, insertSQL, batch, &imported, bar); err != nil {
						return err
					}
				}
				break
			}
			return fmt.Errorf("failed to read CSV row: %w", err)
		}

		batch = append(batch, record)

		// Process batch when it reaches the batch size
		if len(batch) >= options.BatchSize {
			if err := processBatch(db, insertSQL, batch, &imported, bar); err != nil {
				return err
			}
			batch = batch[:0] // Reset batch slice
		}
	}

	duration := time.Since(start)
	utils.PrintSuccess(nil, "✅ Imported %d rows from %s (batch size: %d)", imported, importPath, options.BatchSize)
	utils.PrintInfo(nil, "🕒 Duration: %s", utils.FormatDuration(duration))
	return nil
}

// processBatch handles the insertion of a batch of records
func processBatch(db *sql.DB, insertSQL string, batch [][]string, imported *int64, bar *progressbar.ProgressBar) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for i, row := range batch {
		args := make([]interface{}, len(row))
		for j, v := range row {
			args[j] = v
		}
		if _, err := stmt.Exec(args...); err != nil {
			tx.Rollback()
			return fmt.Errorf("insert failed on batch row %d: %w", i+1, err)
		}
		*imported++
		bar.Add(1)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

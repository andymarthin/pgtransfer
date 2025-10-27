package io

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andymarthin/pgtransfer/internal/utils"
)

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
			if v == nil {
				record[i] = ""
			} else {
				record[i] = fmt.Sprintf("%v", v)
			}
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

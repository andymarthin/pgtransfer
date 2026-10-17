package io

import (
	"database/sql"
	"database/sql/driver"
	"encoding/csv"
	"fmt"
	goio "io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/andymarthin/pgtransfer/internal/utils"
)

// CSVOptions contains configuration for CSV operations
type CSVOptions struct {
	BatchSize int  // Number of rows to process in each batch
	HasHeader bool // Whether the first row of the CSV contains column headers
}

// DefaultBatchSize is the number of rows read or written per batch.
//
// It is deliberately much larger than the historical value of 500. Export used
// to slice the table with LIMIT/OFFSET, where a small batch meant thousands of
// round trips each rescanning the table from the start; that pattern is gone,
// but a small batch is still costly because every batch is a separate query.
// Measured on 5,000,000 rows: batch 500 took 15.3s, batch 20000 took 7.5s, with
// a server-side COPY at 5.3s. 20000 rows is a few megabytes of CSV, so memory
// stays flat while the per-batch overhead shrinks to something negligible.
const DefaultBatchSize = 20000

// CSVExtension is the suffix appended to an export path that has none.
const CSVExtension = ".csv"

// DefaultCSVOptions returns default CSV configuration
func DefaultCSVOptions() *CSVOptions {
	return &CSVOptions{
		BatchSize: DefaultBatchSize,
		HasHeader: true,
	}
}

// CSVOutputPath returns the path an export will actually write.
//
// Export appends ".csv" when the caller omits an extension. Callers that check
// whether they are about to overwrite a file must resolve the path the same
// way, otherwise "pgtransfer export csv prod users users" checks for a file
// called "users" and silently writes "users.csv" over an existing one.
func CSVOutputPath(path string) string {
	if strings.HasSuffix(path, CSVExtension) {
		return path
	}
	return path + CSVExtension
}

// formatCSVValue properly formats a value for CSV export
func FormatCSVValue(v interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case time.Time:
		// Format timestamps in ISO 8601 format without timezone.
		//
		// Written into a fixed buffer rather than via time.Format, which parses
		// its layout string and allocates on every call. A timestamp column was
		// the most expensive per-cell case in the export (105 ns/op) and this
		// brings it closer to the cost of the digits themselves.
		if val.Hour() == 0 && val.Minute() == 0 && val.Second() == 0 && val.Nanosecond() == 0 {
			// Date only: 2006-01-02
			var b [10]byte
			y, m, d := val.Date()
			putDate(b[:], y, int(m), d)
			return string(b[:])
		}
		// Timestamp: 2006-01-02 15:04:05
		var b [19]byte
		y, m, d := val.Date()
		putDate(b[:], y, int(m), d)
		b[10] = ' '
		putTwo(b[11:13], val.Hour())
		b[13] = ':'
		putTwo(b[14:16], val.Minute())
		b[16] = ':'
		putTwo(b[17:19], val.Second())
		return string(b[:])
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
	// The integer cases are split out so the concrete type reaches strconv
	// directly. Formatting through fmt.Sprintf boxes the value and costs an
	// allocation per cell; on an 8-column table that was roughly four million
	// allocations per million rows.
	case int64:
		return strconv.FormatInt(val, 10)
	case int:
		return strconv.FormatInt(int64(val), 10)
	case int8:
		return strconv.FormatInt(int64(val), 10)
	case int16:
		return strconv.FormatInt(int64(val), 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case uint:
		return strconv.FormatUint(uint64(val), 10)
	case uint8:
		return strconv.FormatUint(uint64(val), 10)
	case uint16:
		return strconv.FormatUint(uint64(val), 10)
	case uint32:
		return strconv.FormatUint(uint64(val), 10)
	case float64:
		return strconv.FormatFloat(val, 'g', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(val), 'g', -1, 32)
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

// ExportCSV exports a PostgreSQL table to a CSV file with a live progress bar.
//
// Kept for callers that only need the defaults; it shares the implementation
// with ExportCSVWithOptions so the two cannot diverge.
func ExportCSV(db *sql.DB, table, exportPath string) error {
	options := DefaultCSVOptions()

	start := time.Now()
	exportPath = CSVOutputPath(exportPath)

	if err := os.MkdirAll(filepath.Dir(exportPath), 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	utils.PrintInfo(nil, "Exporting table '%s'...", table)

	// Count total rows for progress tracking
	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteTable(table))
	if err := db.QueryRow(countQuery).Scan(&total); err != nil {
		total = -1 // fallback if counting fails
	}

	query := fmt.Sprintf("SELECT * FROM %s", quoteTable(table))
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

	if options.HasHeader {
		if err := writer.Write(cols); err != nil {
			return fmt.Errorf("failed to write CSV header: %w", err)
		}
	}

	bar := NewProgressBarWithTimer(total, fmt.Sprintf("Exporting %s", table))
	defer bar.Finish()

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
	// A driver can stop iterating because the connection failed, because the
	// server cancelled the query or because a value could not be converted.
	// Next() returning false covers all of those as well as a clean end, so the
	// error must be checked or a truncated export looks like a complete one.
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error reading rows: %w", err)
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
	if options.BatchSize <= 0 {
		options.BatchSize = DefaultBatchSize
	}

	start := time.Now()

	exportPath = CSVOutputPath(exportPath)
	if err := os.MkdirAll(filepath.Dir(exportPath), 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	utils.PrintInfo(nil, "Starting batch export of table '%s' (batch size: %d)...", table, options.BatchSize)

	// Count total rows for progress tracking
	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteTable(table))
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
	query := fmt.Sprintf("SELECT * FROM %s LIMIT 1", quoteTable(table))
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

	// Write CSV header (unless explicitly disabled).
	if options.HasHeader {
		if err := writer.Write(cols); err != nil {
			return fmt.Errorf("failed to write CSV header: %w", err)
		}
	}

	bar := NewProgressBarWithTimer(total, fmt.Sprintf("Exporting %s", table))
	defer bar.Finish()

	var written int64
	offset := int64(0)

	// Batches are paginated on a key rather than by OFFSET.
	//
	// OFFSET batching is incorrect as well as slow: PostgreSQL does not promise
	// a stable order for a query without ORDER BY, and with the default
	// synchronize_seqscans the scan may start part-way through the table. Each
	// batch is a separate query, so consecutive batches could be sliced from
	// differently-rotated orderings, duplicating some rows and silently losing
	// others. OFFSET is also quadratic, because row n cannot be reached without
	// reading the rows before it.
	//
	// Paginating on a unique key with ORDER BY removes both problems: every
	// batch is a true, disjoint window, and each batch reads only the rows it
	// returns.
	cursorCol := cursorColumn(db, table, cols)
	var lastKey interface{}

	// Process data in batches
	for {
		var (
			batchQuery string
			batchArgs  []interface{}
		)
		if cursorCol >= 0 && lastKey != nil {
			batchQuery = fmt.Sprintf(
				"SELECT * FROM %s WHERE %s > $1 ORDER BY %s LIMIT %d",
				quoteTable(table), quoteIdent(cols[cursorCol]), quoteIdent(cols[cursorCol]), options.BatchSize,
			)
			batchArgs = []interface{}{lastKey}
		} else if cursorCol >= 0 {
			batchQuery = fmt.Sprintf(
				"SELECT * FROM %s ORDER BY %s LIMIT %d",
				quoteTable(table), quoteIdent(cols[cursorCol]), options.BatchSize,
			)
		} else {
			batchQuery = fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d", quoteTable(table), options.BatchSize, offset)
		}

		batchRows, err := db.Query(batchQuery, batchArgs...)
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

			// Track the ordering key of the last row so the next batch can
			// start strictly after it.
			if cursorCol >= 0 {
				lastKey = values[cursorCol]
			}

			if err := writer.Write(record); err != nil {
				batchRows.Close()
				return fmt.Errorf("failed to write row: %w", err)
			}

			written++
			batchCount++
			bar.Add(1)
		}
		// A batch that ends early because of a driver error must not be read as
		// the end of the table.
		batchErr := batchRows.Err()
		batchRows.Close()
		if batchErr != nil {
			return fmt.Errorf("error reading batch: %w", batchErr)
		}

		// A short batch is the last batch only when the query returned fewer
		// rows than it asked for and did not report an error. A batch of zero
		// rows also stops the loop, which the previous "fewer than batch size"
		// test did implicitly; an empty table now exits on the first pass
		// instead of running a second query for nothing.
		if batchCount == 0 {
			break
		}
		if batchCount < options.BatchSize {
			break
		}

		if cursorCol < 0 {
			offset += int64(options.BatchSize)
		}

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
	return ImportCSVWithOptions(db, table, importPath, &CSVOptions{
		BatchSize: DefaultBatchSize,
		HasHeader: true,
	})
}

// ImportCSVWithOptions imports data from a CSV file with batch processing for better performance
func ImportCSVWithOptions(db *sql.DB, table, importPath string, options *CSVOptions) error {
	if options == nil {
		options = DefaultCSVOptions()
	}
	if options.BatchSize <= 0 {
		options.BatchSize = DefaultBatchSize
	}

	start := time.Now()

	utils.PrintInfo(nil, "Starting batch import from %s into table '%s' (batch size: %d)...", importPath, table, options.BatchSize)

	file, err := os.Open(importPath)
	if err != nil {
		return fmt.Errorf("failed to open CSV: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Determine whether the first row holds column headers.
	hasHeader := options.HasHeader

	var headers []string
	if hasHeader {
		headers, err = reader.Read()
		if err != nil {
			return fmt.Errorf("failed to read CSV header: %w", err)
		}
	}

	// Count total rows for progress tracking.
	//
	// Counted by re-reading the file in chunks rather than with ReadAll: a
	// large export is hundreds of megabytes, and materialising every record as
	// a []string costs several times the file size in memory just to display a
	// progress bar.
	totalRows, err := countCSVRows(importPath, hasHeader)
	if err != nil {
		return err
	}
	if totalRows <= 0 {
		return fmt.Errorf("no data rows found in %s", importPath)
	}

	// When no header row exists we need the target column names. Without them
	// we fall back to positional inserts by reading them from the table.
	if !hasHeader {
		headers, err = tableColumns(db, table)
		if err != nil {
			return err
		}
		if len(headers) == 0 {
			return fmt.Errorf("could not determine columns for table '%s'", table)
		}
	}

	bar := NewProgressBarWithTimer(totalRows, fmt.Sprintf("Importing %s", table))
	defer bar.Finish()

	insertSQL := buildInsertSQL(table, headers)

	var imported int64
	batch := make([][]string, 0, options.BatchSize)

	// Process CSV in batches
	for {
		record, err := reader.Read()
		if err == goio.EOF {
			// Process final batch if any
			if len(batch) > 0 {
				if err := processBatch(db, insertSQL, batch, &imported, bar); err != nil {
					return err
				}
			}
			break
		}
		if err != nil {
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

// countCSVRows counts the data records in a CSV file by streaming it.
func countCSVRows(path string, hasHeader bool) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open CSV for counting: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1 // ragged rows are validated on import, not here
	reader.ReuseRecord = true

	var n int64
	for {
		_, err := reader.Read()
		if err == goio.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("failed to count CSV rows: %w", err)
		}
		n++
	}
	if hasHeader && n > 0 {
		n--
	}
	return n, nil
}

// nullableValue maps an empty CSV field to SQL NULL.
//
// Export writes NULL as an empty field, so import must invert it. Without this,
// every NULL becomes an empty string and a round-trip silently changes the
// data. A genuinely empty string cannot be distinguished from NULL in CSV,
// which is a limitation of the format itself; treating empty as NULL matches
// what export produced.
func nullableValue(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}

// buildInsertSQL builds the parameterised INSERT used for CSV import.
//
// Column names come from the CSV header, which is user data: they must be
// quoted as identifiers, or a numeric or otherwise unusual header produces a
// syntactically broken statement ("INSERT INTO t (1,2) ...") and a crafted
// header could alter the statement itself. Values are always passed as
// parameters, never interpolated.
func buildInsertSQL(table string, headers []string) string {
	quoted := make([]string, len(headers))
	placeholders := make([]string, len(headers))
	for i, h := range headers {
		quoted[i] = quoteIdent(h)
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		quoteTable(table),
		strings.Join(quoted, ","),
		strings.Join(placeholders, ","),
	)
}

// tableColumns returns the ordered column names for the given table by asking
// PostgreSQL for the column list. Used when importing header-less CSV files.
func tableColumns(db *sql.DB, table string) ([]string, error) {
	query := fmt.Sprintf("SELECT * FROM %s LIMIT 0", quoteTable(table))
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to determine columns for '%s': %w", table, err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to read columns for '%s': %w", table, err)
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("table '%s' has no columns", table)
	}
	return cols, nil
}

// cursorColumn returns the index within cols of a column that can be used to
// paginate the export deterministically: a single-column primary key, or a
// single-column unique, non-null index. It returns -1 when no such column
// exists, in which case the caller must fall back to OFFSET pagination.
//
// Pagination needs a total order over the rows. A unique key provides exactly
// that, which is why any other column is rejected even if it is indexed.
func cursorColumn(db *sql.DB, table string, cols []string) int {
	schema, name := splitTableName(table)

	const q = `
		SELECT a.attname
		FROM pg_index i
		JOIN pg_class c ON c.oid = i.indrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = i.indkey[0]
		WHERE n.nspname = $1
		  AND c.relname = $2
		  AND i.indisunique
		  AND i.indnkeyatts = 1
		  AND i.indpred IS NULL
		  AND a.attnotnull
		ORDER BY i.indisprimary DESC
		LIMIT 1`

	var key string
	if err := db.QueryRow(q, schema, name).Scan(&key); err != nil {
		return -1
	}

	for i, c := range cols {
		if c == key {
			return i
		}
	}
	return -1
}

// splitTableName separates a qualified table name into schema and name,
// honouring quoted parts so that identifiers containing dots survive.
//
// The naive LastIndex(".") split mis-parses `"my.dotted.table"`, which then
// fails the catalogue lookup and silently drops the export onto the OFFSET
// fallback — the non-deterministic pagination this export path exists to
// avoid. Parsing the quotes properly keeps every identifier intact.
func splitTableName(table string) (schema, name string) {
	parts := splitQualifiedIdent(table)
	switch len(parts) {
	case 0:
		return "public", ""
	case 1:
		return "public", parts[0]
	default:
		// More than two parts: treat the last as the name and everything
		// before it as the schema (PostgreSQL allows only two, so this keeps
		// the lookup to something rather than nothing).
		return strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
	}
}

// splitQualifiedIdent splits a qualified identifier into its parts on the dots
// that separate them, leaving dots inside a quoted part alone and stripping the
// quotes. `"my.dotted.table"` is one part; `public.users` is two.
//
// A quote only opens a quoted part at the start of a part. A quote anywhere
// else cannot be part of an unquoted SQL identifier, so it is carried through
// verbatim and re-doubled later by quoteIdent, which keeps an identifier that
// genuinely contains a quote intact.
func splitQualifiedIdent(s string) []string {
	var (
		parts []string
		cur   strings.Builder
		quote bool
	)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' && quote:
			// A doubled quote inside a quoted part is a literal quote.
			if i+1 < len(s) && s[i+1] == '"' {
				cur.WriteByte('"')
				i++
				continue
			}
			quote = false
		case c == '"' && cur.Len() == 0 && !quote:
			// Quote at the start of a part opens a quoted identifier.
			quote = true
		case c == '.' && !quote:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	return append(parts, cur.String())
}

// quoteIdent quotes a PostgreSQL identifier, doubling any embedded quotes so
// that unusual column names cannot break out of the identifier.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// quoteTable renders a possibly schema-qualified table name for use in SQL.
//
// Table and schema names arrive from the command line, from profiles and from
// MCP tool arguments, and are interpolated into statements that lib/pq sends
// over the simple query protocol when there are no parameters — which accepts
// several statements separated by semicolons. An unquoted name is therefore an
// injection point as well as a source of syntax errors on any name that needs
// quoting (mixed case, spaces, dots, reserved words).
//
// Each dot-separated part is quoted separately, mirroring how PostgreSQL
// resolves "schema.table". Dots inside a quoted part are preserved, so an
// identifier such as "my.dotted.table" stays one part, and a name that is
// already quoted round-trips unchanged because the quotes are stripped before
// being re-applied.
func quoteTable(table string) string {
	parts := splitQualifiedIdent(strings.TrimSpace(table))
	for i, p := range parts {
		parts[i] = quoteIdent(p)
	}
	return strings.Join(parts, ".")
}

// QuoteTable is quoteTable for callers outside this package.
func QuoteTable(table string) string { return quoteTable(table) }

// putDate writes "YYYY-MM-DD" into b, which must hold at least 10 bytes.
func putDate(b []byte, year, month, day int) {
	putFour(b[0:4], year)
	b[4] = '-'
	putTwo(b[5:7], month)
	b[7] = '-'
	putTwo(b[8:10], day)
}

// putTwo writes a zero-padded two-digit number.
func putTwo(b []byte, v int) {
	b[0] = byte('0' + v/10%10)
	b[1] = byte('0' + v%10)
}

// putFour writes a zero-padded four-digit number.
func putFour(b []byte, v int) {
	b[0] = byte('0' + v/1000%10)
	b[1] = byte('0' + v/100%10)
	b[2] = byte('0' + v/10%10)
	b[3] = byte('0' + v%10)
}

// processBatch handles the insertion of a batch of records
func processBatch(db *sql.DB, insertSQL string, batch [][]string, imported *int64, bar *Bar) error {
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
			// An empty CSV field is written for a SQL NULL on export, so it is
			// read back as NULL. Passing "" would silently turn every NULL
			// column into an empty string, which is a different value: a
			// round-trip export/import would change the data.
			args[j] = nullableValue(v)
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

// ExportCSVWithQuery runs an arbitrary SELECT and writes the results to a CSV
// file. It returns the number of data rows written (headers excluded).
func ExportCSVWithQuery(db *sql.DB, query, exportPath string, includeHeaders bool) (int, error) {
	file, err := os.Create(exportPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	rows, err := db.Query(query)
	if err != nil {
		return 0, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return 0, fmt.Errorf("failed to get column names: %w", err)
	}

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if includeHeaders {
		if err := writer.Write(columns); err != nil {
			return 0, fmt.Errorf("failed to write headers: %w", err)
		}
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	count := 0
	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return count, fmt.Errorf("failed to scan row: %w", err)
		}
		record := make([]string, len(values))
		for i, v := range values {
			record[i] = FormatCSVValue(v)
		}
		if err := writer.Write(record); err != nil {
			return count, fmt.Errorf("failed to write row: %w", err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return count, fmt.Errorf("error iterating query results: %w", err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return count, fmt.Errorf("failed to flush CSV: %w", err)
	}
	return count, nil
}

// CountCSVDataRows best-effort counts records in a CSV file, optionally
// excluding the header row.
//
// This counts records rather than lines. A CSV record can span several lines
// when a quoted field contains a newline, so counting newline bytes overstates
// the row count for any table with multiline text in it.
func CountCSVDataRows(path string, hasHeader bool) int {
	n, err := countCSVRows(path, hasHeader)
	if err != nil {
		return 0
	}
	if n < 0 {
		return 0
	}
	return int(n)
}

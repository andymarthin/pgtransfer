package io

import (
	"database/sql"
	"fmt"
	"strings"
)

// Import strategy
//
// The rows of a CSV file are loaded with COPY, PostgreSQL's bulk loader: one
// command per batch, the rows streamed as text, the server parsing them with
// the same input functions it uses for its own COPY. A statement per row would
// give the server a parse, a plan and an execute for every value, and each of
// those is a round trip as well.
//
// COPY addresses columns by name, so it needs the target's column list to line
// up with the file's header. When it does not — a file missing a column, or
// naming one the table does not have — the batch is written with INSERT
// statements instead, which is slower but can load a subset of the columns and
// reports the mismatch in terms the user can act on.

// copySupported reports whether the target can accept a COPY FROM STDIN.
//
// lib/pq implements COPY on a dedicated connection against a real PostgreSQL
// backend. The server version is checked rather than assumed: COPY gained CSV
// format and the STDIN form this code relies on long ago, but a proxy that only
// speaks extended-query SQL cannot stream at all, and for that the version
// query fails and the INSERT path takes over.
func copySupported(db *sql.DB) bool {
	var version int
	if err := db.QueryRow("SHOW server_version_num").Scan(&version); err != nil {
		return false
	}
	return version >= 80200
}

// copyStatement renders the COPY command for a table and column list.
//
// The format is text, not csv, and that is deliberate: lib/pq's streaming Exec
// encodes each value in COPY's text syntax — tab delimited, with backslash
// escapes for tab, newline, carriage return and backslash, and \N for NULL.
// Asking the server for FORMAT csv makes it read those bytes as CSV, where the
// tabs are part of the value, so every row fails to type-check.
//
// lib/pq recognises a statement that starts with COPY and, when it is prepared
// inside a transaction, turns it into a streaming COPY: each Exec on the
// statement appends one row, and the copy is finalised by an Exec carrying no
// values.
func copyStatement(table string, columns []string) string {
	quoted := make([]string, len(columns))
	for i, c := range columns {
		quoted[i] = quoteIdent(c)
	}
	return fmt.Sprintf(
		"COPY %s (%s) FROM STDIN",
		quoteTable(table),
		strings.Join(quoted, ","),
	)
}

// copyColumnsForTable returns the target's columns in ordinal order.
//
// COPY needs an explicit column list to map values onto, and the list also
// decides whether the file's header can be used at all.
func copyColumnsForTable(db *sql.DB, table string) ([]string, error) {
	return tableColumns(db, table)
}

// headerMatchesColumns reports whether a CSV header maps one-to-one onto the
// table's columns, in any order. COPY can then address the columns by name.
//
// A header that names a column the table does not have, or omits one, cannot be
// streamed as-is: the missing column would take its default and the extra value
// would have nowhere to go. That case falls back to INSERT so the error the
// user sees names the offending column.
func headerMatchesColumns(header, columns []string) bool {
	if len(header) != len(columns) {
		return false
	}
	have := make(map[string]bool, len(columns))
	for _, c := range columns {
		have[c] = true
	}
	for _, h := range header {
		if !have[h] {
			return false
		}
	}
	return true
}

// copyImport streams a batch into a table with one COPY command.
func copyImport(db *sql.DB, table string, columns []string, batch [][]string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	// Rollback after a successful commit is a no-op.
	defer tx.Rollback()

	stmt, err := tx.Prepare(copyStatement(table, columns))
	if err != nil {
		return fmt.Errorf("failed to prepare COPY into %s: %w", quoteTable(table), err)
	}
	defer stmt.Close()

	args := make([]interface{}, len(columns))
	for _, row := range batch {
		if len(row) != len(columns) {
			return fmt.Errorf("row has %d values but the table has %d columns", len(row), len(columns))
		}
		for i, v := range row {
			// An empty CSV field is written for a SQL NULL on export, so it is
			// read back as NULL: pq encodes a nil as \N, which COPY reads as
			// NULL. Passing "" would make it an empty string, a different value.
			args[i] = nullableValue(v)
		}
		if _, err := stmt.Exec(args...); err != nil {
			return fmt.Errorf("COPY into %s failed: %w", quoteTable(table), err)
		}
	}

	// Exec with no values finalises the copy and returns the row count.
	if _, err := stmt.Exec(); err != nil {
		return fmt.Errorf("COPY into %s failed: %w", quoteTable(table), err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}
	return nil
}

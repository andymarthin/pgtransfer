package io

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/andymarthin/pgtransfer/internal/db"
)

// DryRunReport is what a migration would do, without doing it.
type DryRunReport struct {
	Source        string         `json:"source"`
	Target        string         `json:"target"`
	SchemaOnly    bool           `json:"schema_only"`
	DataOnly      bool           `json:"data_only"`
	Overwrite     bool           `json:"overwrite"`
	Strict        bool           `json:"strict"`
	Tables        []TableReport  `json:"tables"`
	OnlyInSource  []string       `json:"only_in_source"`
	OnlyInTarget  []string       `json:"only_in_target"`
	TypeConflicts []TypeConflict `json:"type_conflicts"`
	Warnings      []string       `json:"warnings"`
}

// TableReport is one table that would be migrated.
type TableReport struct {
	Table   string `json:"table"`
	Rows    int64  `json:"rows"`
	Exists  bool   `json:"exists_in_target"` // false means the restore creates it
	Dropped bool   `json:"would_drop"`       // true when --overwrite drops a target table
}

// TypeConflict is a column whose type differs between source and target. The
// restore does not fix this: an existing target column keeps its type and the
// copy fails, or silently coerces.
type TypeConflict struct {
	Table  string `json:"table"`
	Column string `json:"column"`
	Source string `json:"source_type"`
	Target string `json:"target_type"`
}

// columnKey identifies a column across two databases.
type columnKey struct {
	schema string
	table  string
	column string
}

// DryRunMigration reports what MigrateDatabaseWithConnection would do: which
// tables it would copy, which existing target tables --overwrite would drop, and
// which columns have incompatible types.
//
// It is read-only. Both databases are queried for table lists, approximate row
// counts and column types; nothing is dumped and nothing is written. The counts
// come from the planner's statistics, so they are estimates on a busy database.
func DryRunMigration(opts *MigrationOptions) (*DryRunReport, error) {
	report := &DryRunReport{
		Source:        opts.SourceProfile.Name,
		Target:        opts.TargetProfile.Name,
		SchemaOnly:    opts.SchemaOnly,
		DataOnly:      opts.DataOnly,
		Overwrite:     opts.Overwrite,
		Strict:        opts.Strict,
		Tables:        []TableReport{},
		OnlyInSource:  []string{},
		OnlyInTarget:  []string{},
		TypeConflicts: []TypeConflict{},
		Warnings:      []string{},
	}

	source, err := db.Connect(opts.SourceProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to source database: %w", err)
	}
	defer source.Close()

	target, err := db.Connect(opts.TargetProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to target database: %w", err)
	}
	defer target.Close()

	sourceTables, err := listTables(source.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to list source tables: %w", err)
	}
	if len(sourceTables) == 0 {
		report.Warnings = append(report.Warnings, "the source database has no tables in the public schema")
	}

	targetTables, err := listTables(target.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to list target tables: %w", err)
	}

	// Honour --tables the same way the migration does, but report a name that
	// does not exist rather than silently migrating nothing.
	selected := sourceTables
	if len(opts.Tables) > 0 {
		selected, report.Warnings = selectTables(opts.Tables, sourceTables, report.Warnings)
	}

	inTarget := make(map[string]bool, len(targetTables))
	for _, t := range targetTables {
		inTarget[t] = true
	}
	inSource := make(map[string]bool, len(sourceTables))
	for _, t := range sourceTables {
		inSource[t] = true
	}

	for _, t := range selected {
		rows, err := estimateRows(source.DB, t)
		if err != nil {
			return nil, fmt.Errorf("failed to count rows in %s: %w", t, err)
		}
		exists := inTarget[t]
		report.Tables = append(report.Tables, TableReport{
			Table: t,
			Rows:  rows,
			// A data-only migration assumes the schema is already there, so an
			// absent table is a failure, not a creation.
			Exists:  exists,
			Dropped: exists && opts.Overwrite && !opts.DataOnly,
		})
		if !exists && opts.DataOnly {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("--data-only, but table %q does not exist in the target; the restore will fail", t))
		}
	}

	for _, t := range selected {
		if !inTarget[t] {
			report.OnlyInSource = append(report.OnlyInSource, t)
		}
	}
	for _, t := range targetTables {
		if !inSource[t] {
			report.OnlyInTarget = append(report.OnlyInTarget, t)
		}
	}

	// Column types only matter when the target table already exists: a table
	// created by the restore takes the source's types.
	if !opts.SchemaOnly {
		conflicts, err := typeConflicts(source.DB, target.DB, selected, inTarget)
		if err != nil {
			return nil, err
		}
		report.TypeConflicts = conflicts
	}

	if opts.DataOnly && len(report.OnlyInSource) > 0 {
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("--data-only will fail: %d table(s) exist only in the source", len(report.OnlyInSource)))
	}
	if opts.Overwrite && !opts.DataOnly && len(report.OnlyInTarget) > 0 {
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("--overwrite only drops tables that the dump recreates; %d target-only table(s) are left untouched",
				len(report.OnlyInTarget)))
	}

	return report, nil
}

// selectTables resolves the --tables list against what the source contains.
func selectTables(wanted, available []string, warnings []string) ([]string, []string) {
	known := make(map[string]bool, len(available))
	for _, t := range available {
		known[t] = true
	}

	var out []string
	for _, w := range wanted {
		if w == "" {
			continue
		}
		if known[w] {
			out = append(out, w)
			continue
		}
		warnings = append(warnings, fmt.Sprintf("table %q is not in the source database", w))
	}
	return out, warnings
}

// listTables returns the qualified names of the ordinary tables in the public
// schema, sorted. Sequences and views are excluded: pg_dump recreates them, but
// they carry no rows to count and would make the report noise.
func listTables(conn *sql.DB) ([]string, error) {
	const q = `
		SELECT schemaname || '.' || tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename`

	rows, err := conn.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// estimateRows reads the planner's row estimate. Counting exactly would scan
// every table, which is the cost this command exists to avoid.
func estimateRows(conn *sql.DB, table string) (int64, error) {
	const q = `
		SELECT GREATEST(c.reltuples, 0)::bigint
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2`

	schema, name := schemaAndTable(table)

	var rows int64
	err := conn.QueryRow(q, schema, name).Scan(&rows)
	if err == sql.ErrNoRows {
		// Not in pg_class yet (a table created since the last statistics
		// refresh). Reporting zero is closer to the truth than failing.
		return 0, nil
	}
	return rows, err
}

// schemaAndTable splits a table name into its schema and table parts, treating a
// bare name as public. It keeps an embedded quote intact, so a table called
// `we"ird` is not mangled into a different name.
func schemaAndTable(qualified string) (schema, table string) {
	parts := splitQualifiedIdent(qualified)
	switch len(parts) {
	case 1:
		return "public", parts[0]
	default:
		return parts[len(parts)-2], parts[len(parts)-1]
	}
}

// typeConflicts compares column types for tables that already exist in the
// target. data_type and udt_name together distinguish types that share a name
// across schemas and capture array element types.
func typeConflicts(source, target *sql.DB, tables []string, inTarget map[string]bool) ([]TypeConflict, error) {
	sourceCols, err := columnTypes(source)
	if err != nil {
		return nil, fmt.Errorf("failed to read source column types: %w", err)
	}
	targetCols, err := columnTypes(target)
	if err != nil {
		return nil, fmt.Errorf("failed to read target column types: %w", err)
	}

	var conflicts []TypeConflict
	for _, table := range tables {
		if !inTarget[table] {
			continue
		}
		schema, name := schemaAndTable(table)

		for key, srcType := range sourceCols {
			if key.schema != schema || key.table != name {
				continue
			}
			tgtType, ok := targetCols[key]
			if !ok || tgtType == srcType {
				continue
			}
			conflicts = append(conflicts, TypeConflict{
				Table:  table,
				Column: key.column,
				Source: srcType,
				Target: tgtType,
			})
		}
	}

	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].Table != conflicts[j].Table {
			return conflicts[i].Table < conflicts[j].Table
		}
		return conflicts[i].Column < conflicts[j].Column
	})
	return conflicts, nil
}

// columnTypes maps every column in the public schema to its declared type.
func columnTypes(conn *sql.DB) (map[columnKey]string, error) {
	const q = `
		SELECT n.nspname, c.relname, a.attname,
		       CASE WHEN a.atttypid = 'pg_catalog.varchar'::regtype
		              OR a.atttypid = 'pg_catalog.bpchar'::regtype
		            THEN format_type(a.atttypid, a.atttypmod)
		            ELSE a.atttypid::regtype::text
		       END
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public'
		  AND c.relkind = 'r'
		  AND a.attnum > 0
		  AND NOT a.attisdropped`

	rows, err := conn.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[columnKey]string)
	for rows.Next() {
		var schema, table, column, typ string
		if err := rows.Scan(&schema, &table, &column, &typ); err != nil {
			return nil, err
		}
		out[columnKey{schema: schema, table: table, column: column}] = typ
	}
	return out, rows.Err()
}

// Lines renders the report as plain text lines, one per printed row. It returns
// text rather than writing it so the caller owns colour and destination, and so
// the plan can be asserted on.
func (r *DryRunReport) Lines() []string {
	var lines []string
	add := func(format string, args ...interface{}) {
		lines = append(lines, fmt.Sprintf(format, args...))
	}

	mode := "schema and data"
	switch {
	case r.SchemaOnly:
		mode = "schema only"
	case r.DataOnly:
		mode = "data only"
	}

	add("From:      %s", r.Source)
	add("To:        %s", r.Target)
	add("Mode:      %s", mode)
	if r.Overwrite {
		add("Overwrite: on - existing target tables are dropped and recreated")
	} else {
		add("Overwrite: off - rows are added to existing target tables")
	}
	add("Strict:    %v", r.Strict)

	if len(r.Tables) > 0 {
		lines = append(lines, "")
		lines = append(lines, renderTables(r.Tables)...)
		var totalRows int64
		for _, t := range r.Tables {
			totalRows += t.Rows
		}
		add("")
		add("%s rows (estimated) in %d table(s)", approximate(totalRows), len(r.Tables))
	} else {
		add("")
		add("No tables would be migrated.")
	}

	if len(r.TypeConflicts) > 0 {
		lines = append(lines, "", "Columns whose type differs (the restore does not change an existing column's type):")
		rows := make([][4]string, 0, len(r.TypeConflicts)+1)
		rows = append(rows, [4]string{"TABLE", "COLUMN", "SOURCE", "TARGET"})
		for _, c := range r.TypeConflicts {
			rows = append(rows, [4]string{c.Table, c.Column, c.Source, c.Target})
		}
		lines = append(lines, align(rows)...)
	}

	for _, w := range r.Warnings {
		lines = append(lines, "", "warning: "+w)
	}
	return lines
}

// renderTables lays out the per-table section.
func renderTables(tables []TableReport) []string {
	rows := make([][3]string, 0, len(tables)+1)
	rows = append(rows, [3]string{"TABLE", "ROWS", "TARGET"})
	for _, t := range tables {
		rows = append(rows, [3]string{t.Table, approximate(t.Rows), targetStatus(t)})
	}
	return align3(rows)
}

// targetStatus explains what happens to a table in the target.
func targetStatus(t TableReport) string {
	switch {
	case t.Dropped:
		return "dropped and recreated"
	case t.Exists:
		return "exists - rows are added"
	default:
		return "created"
	}
}

// approximate renders a row count with thousands separators, since the numbers
// involved are rarely small.
func approximate(n int64) string {
	s := fmt.Sprintf("%d", n)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

// align lays out a four-column table with two-space gaps, padding every column
// but the last so the widest cell wins.
func align(rows [][4]string) []string {
	widths := [4]int{}
	for _, row := range rows {
		for i := 0; i < 3; i++ {
			if len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		var b strings.Builder
		for i := 0; i < 3; i++ {
			b.WriteString(row[i])
			b.WriteString(strings.Repeat(" ", widths[i]-len(row[i])+2))
		}
		b.WriteString(row[3])
		out = append(out, b.String())
	}
	return out
}

// align3 is align for the three-column per-table section.
func align3(rows [][3]string) []string {
	widths := [3]int{}
	for _, row := range rows {
		for i := 0; i < 2; i++ {
			if len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		var b strings.Builder
		for i := 0; i < 2; i++ {
			b.WriteString(row[i])
			b.WriteString(strings.Repeat(" ", widths[i]-len(row[i])+2))
		}
		b.WriteString(row[2])
		out = append(out, b.String())
	}
	return out
}

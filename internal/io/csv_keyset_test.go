package io

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The export used to paginate with "LIMIT n OFFSET m" and no ORDER BY. That is
// both quadratic and, more seriously, incorrect: PostgreSQL makes no ordering
// promise for such a query, so each batch of a multi-batch export could be
// sliced from a differently-rotated ordering, writing some rows twice and never
// writing others. Exit status stayed 0, so the data loss was silent.
//
// These tests drive ExportCSVWithOptions through a fake driver that reproduces
// the rotation. They assert on the property that actually matters: the exported
// file must contain every row of the source exactly once.

// fakeDB builds a *sql.DB backed by the in-memory driver registered below.
// rows are given in logical order; the driver decides how to answer a query.
func fakeDB(t *testing.T, tables map[string][][]string, keyCol string) *sql.DB {
	t.Helper()
	db, err := sql.Open("csvfake", "")
	if err != nil {
		t.Fatalf("open fake db: %v", err)
	}
	db.SetMaxOpenConns(1)
	fakeState = &fakeStateT{tables: tables, keyCol: keyCol}
	t.Cleanup(func() { db.Close() })
	return db
}

type fakeStateT struct {
	tables map[string][][]string // table name -> data rows (no header)
	keyCol string                // single-column unique key honoured by ORDER BY
	// queries records every SQL statement the exporter issued, so a test can
	// assert that OFFSET was not used.
	queries []string
	// rotate makes unordered scans start part-way through the table, mimicking
	// synchronize_seqscans.
	rotate int
}

var fakeState *fakeStateT

func init() {
	sql.Register("csvfake", fakeDriver{})
}

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return fakeConn{}, nil }

type fakeConn struct{}

func (fakeConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (fakeConn) Close() error                        { return nil }
func (fakeConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (fakeConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return fakeQuery(query, args)
}

var _ driver.QueryerContext = fakeConn{}

// fakeQuery answers the handful of statements the exporter issues.
func fakeQuery(query string, args []driver.NamedValue) (driver.Rows, error) {
	st := fakeState
	if st == nil {
		return nil, fmt.Errorf("fake driver not initialised")
	}
	st.queries = append(st.queries, query)

	// Column/key discovery used by cursorColumn.
	if strings.Contains(query, "FROM pg_index") {
		if st.keyCol == "" {
			return &fakeRows{cols: []string{"attname"}}, nil
		}
		return &fakeRows{cols: []string{"attname"}, data: [][]driver.Value{{st.keyCol}}}, nil
	}

	table := fakeTableName(query)
	all, ok := st.tables[table]
	if !ok {
		return nil, fmt.Errorf("table %q not found", table)
	}
	if len(all) == 0 {
		return &fakeRows{}, nil
	}
	// Row 0 holds the column names; the rest are data rows.
	header, data := all[0], all[1:]

	// Header discovery: "SELECT * FROM t LIMIT 1" / "LIMIT 0".
	if strings.HasSuffix(strings.TrimSpace(query), "LIMIT 1") ||
		strings.HasSuffix(strings.TrimSpace(query), "LIMIT 0") {
		return &fakeRows{cols: header}, nil
	}
	// Row count for the progress bar.
	if strings.Contains(query, "COUNT(*)") {
		return &fakeRows{cols: []string{"count"}, data: [][]driver.Value{{int64(len(data))}}}, nil
	}

	rows := make([][]driver.Value, len(data))
	for i, r := range data {
		row := make([]driver.Value, len(r))
		for j, v := range r {
			row[j] = v
		}
		rows[i] = row
	}
	keyIdx := fakeKeyIndex(query, header)

	// ORDER BY <key>: deterministic, and with a cursor argument, resumable.
	if keyIdx >= 0 && strings.Contains(query, "ORDER BY") {
		sort.SliceStable(rows, func(i, j int) bool {
			return fakeLess(rows[i][keyIdx], rows[j][keyIdx])
		})
		if len(args) > 0 {
			cursor := args[0].Value
			filtered := rows[:0:0]
			for _, r := range rows {
				if fakeGreater(r[keyIdx], cursor) {
					filtered = append(filtered, r)
				}
			}
			rows = filtered
		}
	} else {
		// No ORDER BY: every call rotates the start point, exactly like a
		// synchronised sequential scan. This is what caused the data loss.
		if len(rows) > 0 && st.rotate > 0 {
			k := st.rotate % len(rows)
			rows = append(append([][]driver.Value(nil), rows[k:]...), rows[:k]...)
			st.rotate = (st.rotate * 3) % len(rows)
		}
	}

	_ = keyIdx
	limit, offset := fakeLimitOffset(query)
	if offset > len(rows) {
		offset = len(rows)
	}
	rows = rows[offset:]
	if limit >= 0 && limit < len(rows) {
		rows = rows[:limit]
	}
	return &fakeRows{cols: header, data: rows}, nil
}

func fakeTableName(query string) string {
	const marker = "SELECT * FROM "
	i := strings.Index(query, marker)
	if i < 0 {
		return ""
	}
	rest := query[i+len(marker):]
	end := strings.IndexAny(rest, " \t\n")
	name := rest
	if end >= 0 {
		name = rest[:end]
	}
	if j := strings.LastIndex(name, "."); j >= 0 {
		name = name[j+1:]
	}
	return strings.Trim(name, `"`)
}

func fakeCols(data [][]string) []string {
	if len(data) == 0 {
		return nil
	}
	// The fake stores the header as data row 0 so column names are available
	// even for an empty table.
	return data[0]
}

func fakeKeyIndex(query string, cols []string) int {
	i := strings.Index(query, "ORDER BY ")
	if i < 0 {
		return -1
	}
	rest := query[i+len("ORDER BY "):]
	name := strings.Trim(strings.Fields(rest)[0], `"`)
	for j, c := range cols {
		if c == name {
			return j
		}
	}
	return -1
}

func fakeLimitOffset(query string) (int, int) {
	limit, offset := -1, 0
	if _, err := fmt.Sscanf(lastToken(query, "LIMIT"), "%d", &limit); err != nil {
		limit = -1
	}
	if _, err := fmt.Sscanf(lastToken(query, "OFFSET"), "%d", &offset); err != nil {
		offset = 0
	}
	return limit, offset
}

func lastToken(query, keyword string) string {
	i := strings.LastIndex(query, keyword+" ")
	if i < 0 {
		return ""
	}
	return strings.Fields(query[i+len(keyword)+1:])[0]
}

func fakeLess(a, b driver.Value) bool {
	ai, aok := a.(int64)
	bi, bok := b.(int64)
	if aok && bok {
		return ai < bi
	}
	return fmt.Sprint(a) < fmt.Sprint(b)
}

func fakeGreater(a, b driver.Value) bool { return fakeLess(b, a) }

type fakeRows struct {
	cols []string
	data [][]driver.Value
	pos  int
}

func (r *fakeRows) Columns() []string { return r.cols }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.pos])
	r.pos++
	return nil
}

// exportFile runs an export and returns the parsed CSV rows.
func exportFile(t *testing.T, db *sql.DB, table string, opts *CSVOptions) [][]string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "out.csv")
	if err := ExportCSVWithOptions(db, table, out, opts); err != nil {
		t.Fatalf("export failed: %v", err)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer f.Close()
	rows, err := readAll(f)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	return rows
}

func readAll(r io.Reader) ([][]string, error) {
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(r); err != nil {
		return nil, err
	}
	if buf.Len() == 0 {
		return nil, nil
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	out := make([][]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, strings.Split(line, ","))
	}
	return out, nil
}

// buildTable produces rows: header ["id","payload"], then n data rows.
func buildTable(n int) [][]string {
	rows := [][]string{{"id", "payload"}}
	for i := 1; i <= n; i++ {
		rows = append(rows, []string{fmt.Sprint(i), fmt.Sprintf("payload_%d", i)})
	}
	return rows
}

// TestExportCSVWithOptions_NoRowsLostOrDuplicated is the regression test for the
// data-loss bug: a multi-batch export must contain every row exactly once, even
// when the underlying scan order is not stable.
func TestExportCSVWithOptions_NoRowsLostOrDuplicated(t *testing.T) {
	const rows = 1000
	data := buildTable(rows)

	for _, batchSize := range []int{25, 100, 500} {
		batchSize := batchSize
		t.Run(fmt.Sprintf("batch_%d", batchSize), func(t *testing.T) {
			db := fakeDB(t, map[string][][]string{"t": data}, "id")
			// Force an unstable scan: every unordered query starts at a
			// different point, like synchronize_seqscans.
			fakeState.rotate = 337

			out := exportFile(t, db, "t", &CSVOptions{BatchSize: batchSize, HasHeader: true})

			if len(out) != rows+1 {
				t.Fatalf("wrote %d lines, want %d (header + %d rows)", len(out), rows+1, rows)
			}
			seen := make(map[string]int, rows)
			for _, r := range out[1:] {
				seen[r[0]]++
			}
			if len(seen) != rows {
				t.Errorf("exported %d distinct rows, want %d (%d lost)",
					len(seen), rows, rows-len(seen))
			}
			// Report duplicates compactly: a broken pagination writes many rows
			// more than once, and listing each one buries the signal.
			duplicated, repeats := 0, 0
			first := ""
			for id, n := range seen {
				if n > 1 {
					duplicated++
					repeats += n - 1
					if first == "" {
						first = id
					}
				}
			}
			if duplicated > 0 {
				t.Errorf("%d rows written more than once (%d extra copies, e.g. row %s)",
					duplicated, repeats, first)
			}
		})
	}
}

// TestExportCSVWithOptions_UsesKeysetPagination asserts the exporter asks for
// rows by key rather than by OFFSET, which is what makes the export both
// correct and linear.
func TestExportCSVWithOptions_UsesKeysetPagination(t *testing.T) {
	db := fakeDB(t, map[string][][]string{"t": buildTable(200)}, "id")

	exportFile(t, db, "t", &CSVOptions{BatchSize: 50, HasHeader: true})

	var sawOrderBy, sawOffset bool
	for _, q := range fakeState.queries {
		if strings.Contains(q, "SELECT * FROM") && strings.Contains(q, "ORDER BY") {
			sawOrderBy = true
		}
		if strings.Contains(q, "OFFSET") && !strings.Contains(q, "LIMIT 1") {
			sawOffset = true
		}
	}
	if !sawOrderBy {
		t.Error("no batch query used ORDER BY; ordering is not deterministic")
	}
	if sawOffset {
		t.Error("a batch query used OFFSET; pagination should be key-based")
	}
}

// TestExportCSVWithOptions_NoKeyFallsBackToOffset checks the table-without-a-key
// path still exports every row when the scan order happens to be stable.
func TestExportCSVWithOptions_NoKeyFallsBackToOffset(t *testing.T) {
	db := fakeDB(t, map[string][][]string{"t": buildTable(120)}, "")

	out := exportFile(t, db, "t", &CSVOptions{BatchSize: 50, HasHeader: true})
	if len(out) != 121 {
		t.Fatalf("wrote %d lines, want 121", len(out))
	}
	seen := map[string]bool{}
	for _, r := range out[1:] {
		seen[r[0]] = true
	}
	if len(seen) != 120 {
		t.Errorf("exported %d distinct rows, want 120", len(seen))
	}
}

// TestCursorColumn_RejectsNonUniqueKeys covers the guard that only a
// single-column unique, non-null key is used for pagination.
func TestCursorColumn_RejectsNonUniqueKeys(t *testing.T) {
	cols := []string{"id", "payload"}

	db := fakeDB(t, map[string][][]string{"t": buildTable(2)}, "")
	if got := cursorColumn(db, "t", cols); got != -1 {
		t.Errorf("cursorColumn without a unique key = %d, want -1", got)
	}

	fakeState.keyCol = "id"
	if got := cursorColumn(db, "public.t", cols); got != 0 {
		t.Errorf("cursorColumn = %d, want 0", got)
	}

	// A key that is unique but not part of the result set is rejected rather
	// than producing a bad index.
	fakeState.keyCol = "other_column"
	if got := cursorColumn(db, "t", cols); got != -1 {
		t.Errorf("cursorColumn for absent column = %d, want -1", got)
	}
}

func TestQuoteIdent(t *testing.T) {
	cases := map[string]string{
		"id":        `"id"`,
		"user_id":   `"user_id"`,
		`weird"col`: `"weird""col"`,
		"MixedCase": `"MixedCase"`,
	}
	for in, want := range cases {
		if got := quoteIdent(in); got != want {
			t.Errorf("quoteIdent(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestExportDigestStable confirms two exports of the same data produce
// identical files, which is the user-visible promise of a correct export.
func TestExportDigestStable(t *testing.T) {
	data := buildTable(300)

	digest := func(rotate int) string {
		db := fakeDB(t, map[string][][]string{"t": data}, "id")
		fakeState.rotate = rotate
		out := exportFile(t, db, "t", &CSVOptions{BatchSize: 64, HasHeader: true})
		h := sha256.New()
		for _, r := range out {
			fmt.Fprintln(h, strings.Join(r, "|"))
		}
		return hex.EncodeToString(h.Sum(nil))
	}

	a := digest(7)
	b := digest(911)
	if a != b {
		t.Errorf("exports of identical data differ:\n a=%s\n b=%s", a, b)
	}
}

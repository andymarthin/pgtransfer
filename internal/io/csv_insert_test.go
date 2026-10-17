package io

import (
	"strings"
	"testing"
)

// TestNullableValue covers the CSV-to-SQL value mapping used on import.
//
// Export writes SQL NULL as an empty field; import must invert that, or every
// NULL becomes an empty string and a round-trip changes the data (measured:
// 66,666 of 200,000 rows differed before this was handled).
func TestNullableValue(t *testing.T) {
	cases := []struct {
		in   string
		want interface{}
	}{
		{"", nil},
		{"x", "x"},
		{"0", "0"},
		{" ", " "},
		{"NULL", "NULL"},
		{"false", "false"},
	}
	for _, c := range cases {
		if got := nullableValue(c.in); got != c.want {
			t.Errorf("nullableValue(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

// TestNullRoundTripSymmetry pins the invariant that makes round-trips safe: the
// value written for NULL must read back as NULL.
func TestNullRoundTripSymmetry(t *testing.T) {
	// What export writes for a NULL column.
	written := FormatCSVValue(nil)
	if written != "" {
		t.Fatalf("export writes %q for NULL, expected an empty field", written)
	}
	// What import must produce from that field.
	if got := nullableValue(written); got != nil {
		t.Errorf("import of %q = %#v, want nil", written, got)
	}
}

// TestBuildInsertSQL covers the statement built for CSV import. Column names
// come from the CSV header, so they are user data and must never reach the
// statement unquoted: an unquoted numeric header produced
// "INSERT INTO t (1,2) VALUES ($1,$2)", which PostgreSQL rejects with a syntax
// error, breaking the ordinary case of importing a header-less export.
func TestBuildInsertSQL(t *testing.T) {
	cases := []struct {
		name    string
		table   string
		headers []string
		want    string
	}{
		{
			name:    "plain columns",
			table:   "users",
			headers: []string{"id", "name"},
			want:    `INSERT INTO users ("id","name") VALUES ($1,$2)`,
		},
		{
			name:    "numeric header from a header-less file",
			table:   "public.compat_rt",
			headers: []string{"1", "customer_1", "user1@example.com"},
			want:    `INSERT INTO public.compat_rt ("1","customer_1","user1@example.com") VALUES ($1,$2,$3)`,
		},
		{
			name:    "reserved word column",
			table:   "t",
			headers: []string{"select", "from", "order"},
			want:    `INSERT INTO t ("select","from","order") VALUES ($1,$2,$3)`,
		},
		{
			name:    "mixed case is preserved",
			table:   "t",
			headers: []string{"UserId", "CreatedAt"},
			want:    `INSERT INTO t ("UserId","CreatedAt") VALUES ($1,$2)`,
		},
		{
			name:    "embedded quote is doubled",
			table:   "t",
			headers: []string{`we"ird`},
			want:    `INSERT INTO t ("we""ird") VALUES ($1)`,
		},
		{
			name:    "attempted injection cannot escape the identifier",
			table:   "t",
			headers: []string{`a" ) VALUES (1); DROP TABLE users; --`},
			want:    `INSERT INTO t ("a"" ) VALUES (1); DROP TABLE users; --") VALUES ($1)`,
		},
		{
			name:    "single column",
			table:   "t",
			headers: []string{"only"},
			want:    `INSERT INTO t ("only") VALUES ($1)`,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := buildInsertSQL(c.table, c.headers)
			if got != c.want {
				t.Errorf("buildInsertSQL() =\n  %s\nwant\n  %s", got, c.want)
			}
		})
	}
}

// TestBuildInsertSQLAlwaysQuotes is a property check: every header must appear
// enclosed in double quotes, so no header can ever be interpreted as SQL.
func TestBuildInsertSQLAlwaysQuotes(t *testing.T) {
	headers := []string{
		"id", "1", "2.5", "-x", "a b", "a,b", "a)b", "a;b", `a"b`,
		"select", "table", "--", "/*", "*/", "\u00e9t\u00e9", "col'quote",
	}
	sql := buildInsertSQL("t", headers)

	columnList := strings.TrimSuffix(strings.SplitN(sql, ") VALUES (", 2)[0], ")")
	columnList = strings.TrimPrefix(columnList, "INSERT INTO t (")

	got := strings.Split(columnList, `","`)
	for i := range got {
		got[i] = strings.Trim(got[i], `"`)
	}
	if len(got) != len(headers) {
		t.Fatalf("got %d columns, want %d: %s", len(got), len(headers), sql)
	}
	// Each header must survive as one quoted identifier, with inner quotes
	// doubled, so re-parsing recovers the original name.
	for i, h := range headers {
		recovered := strings.ReplaceAll(got[i], `""`, `"`)
		if recovered != h {
			t.Errorf("header %d: recovered %q, want %q (sql: %s)", i, recovered, h, sql)
		}
	}
}

// TestBuildInsertSQLPlaceholderCount guards the placeholder list, which must
// match the column count exactly or every insert fails.
func TestBuildInsertSQLPlaceholderCount(t *testing.T) {
	for n := 1; n <= 20; n++ {
		headers := make([]string, n)
		for i := range headers {
			headers[i] = "c"
		}
		sql := buildInsertSQL("t", headers)
		if got := strings.Count(sql, "$"); got != n {
			t.Errorf("%d columns produced %d placeholders: %s", n, got, sql)
		}
	}
}

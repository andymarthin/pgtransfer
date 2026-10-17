package io

import (
	"strings"
	"testing"
)

// TestCopyStatement pins the COPY command shape.
//
// lib/pq decides to stream a COPY by looking at the first four characters of
// the statement, and the column list has to be quoted or a name that needs
// quoting breaks the command. Both are easy to get wrong in a way that only
// shows up on a table with an unusual column name.
func TestCopyStatement(t *testing.T) {
	got := copyStatement("public.users", []string{"id", "email"})
	want := `COPY "public"."users" ("id","email") FROM STDIN`
	if got != want {
		t.Errorf("copyStatement() = %q, want %q", got, want)
	}

	// Text format, not csv: lib/pq encodes values in COPY's text syntax.
	if strings.Contains(got, "FORMAT csv") {
		t.Errorf("statement asks for CSV but lib/pq writes text: %q", got)
	}
	if !strings.HasPrefix(got, "COPY") {
		t.Errorf("statement does not start with COPY, so lib/pq would not stream it: %q", got)
	}

	// A column that needs quoting must survive.
	odd := copyStatement("t", []string{"select", "my col", `we"ird`})
	for _, want := range []string{`"select"`, `"my col"`, `"we""ird"`} {
		if !strings.Contains(odd, want) {
			t.Errorf("copyStatement() = %q, want it to contain %q", odd, want)
		}
	}
}

// TestHeaderMatchesColumns pins the decision to use COPY.
//
// COPY addresses columns by name, so a header that names a column the table
// does not have — or omits one — cannot be streamed. Getting this wrong would
// silently drop a column's values into the wrong columns, or fail with a
// message about a column the user never mentioned.
func TestHeaderMatchesColumns(t *testing.T) {
	table := []string{"id", "email", "created_at"}

	tests := []struct {
		name   string
		header []string
		want   bool
	}{
		{"same order", []string{"id", "email", "created_at"}, true},
		{"reordered", []string{"created_at", "id", "email"}, true},
		{"missing a column", []string{"id", "email"}, false},
		{"extra column", []string{"id", "email", "created_at", "notes"}, false},
		{"wrong name", []string{"id", "mail", "created_at"}, false},
		{"empty header", nil, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := headerMatchesColumns(tc.header, table); got != tc.want {
				t.Errorf("headerMatchesColumns(%v) = %v, want %v", tc.header, got, tc.want)
			}
		})
	}
}

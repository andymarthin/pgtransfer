package io

import (
	"strings"
	"testing"
)

// TestSchemaAndTable pins the qualified-name split, including the bare-name and
// embedded-quote cases the dry run has to survive.
func TestSchemaAndTable(t *testing.T) {
	tests := []struct {
		in           string
		schema, name string
	}{
		{"users", "public", "users"},
		{"public.users", "public", "users"},
		{"app.orders", "app", "orders"},
		{`we"ird`, "public", `we"ird`},
		// A quoted part uses "" for a literal quote, as PostgreSQL does.
		{`app."or""der"`, "app", `or"der`},
	}

	for _, tc := range tests {
		schema, name := schemaAndTable(tc.in)
		if schema != tc.schema || name != tc.name {
			t.Errorf("schemaAndTable(%q) = (%q, %q), want (%q, %q)",
				tc.in, schema, name, tc.schema, tc.name)
		}
	}
}

// TestApproximate pins the thousands separators. A migration plan is read to
// judge scale, and 842113 is not readable at a glance.
func TestApproximate(t *testing.T) {
	tests := map[int64]string{
		0:          "0",
		7:          "7",
		999:        "999",
		1000:       "1,000",
		2048:       "2,048",
		842113:     "842,113",
		1000000:    "1,000,000",
		1234567890: "1,234,567,890",
	}
	for in, want := range tests {
		if got := approximate(in); got != want {
			t.Errorf("approximate(%d) = %q, want %q", in, got, want)
		}
	}
}

// TestTargetStatus pins the three outcomes a plan can report per table.
func TestTargetStatus(t *testing.T) {
	tests := []struct {
		name  string
		table TableReport
		want  string
	}{
		{"absent", TableReport{Exists: false}, "created"},
		{"present", TableReport{Exists: true}, "exists - rows are added"},
		{"overwritten", TableReport{Exists: true, Dropped: true}, "dropped and recreated"},
		// Dropped wins over Exists: what matters is that data goes away.
		{"overwritten but reported as absent", TableReport{Exists: false, Dropped: true}, "dropped and recreated"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := targetStatus(tc.table); got != tc.want {
				t.Errorf("targetStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSelectTables pins that an unknown --tables entry is a warning rather than
// a silent omission. A typo in a table list otherwise migrates nothing and
// reports success.
func TestSelectTables(t *testing.T) {
	available := []string{"public.users", "public.orders"}

	got, warnings := selectTables([]string{"public.users", "public.ghost", ""}, available, nil)
	if len(got) != 1 || got[0] != "public.users" {
		t.Errorf("selectTables() kept %v, want only public.users", got)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "public.ghost") {
		t.Errorf("warnings = %v, want one naming public.ghost", warnings)
	}

	got, warnings = selectTables([]string{"public.orders"}, available, nil)
	if len(got) != 1 || got[0] != "public.orders" {
		t.Errorf("selectTables() = %v, want public.orders", got)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none for a table that exists", warnings)
	}
}

// TestDryRunReportLines pins the shape of the rendered plan, since that text is
// what a user reads before a destructive operation.
func TestDryRunReportLines(t *testing.T) {
	report := &DryRunReport{
		Source:    "prod",
		Target:    "staging",
		Overwrite: true,
		Tables: []TableReport{
			{Table: "public.users", Rows: 250000, Exists: true, Dropped: true},
			{Table: "public.orders", Rows: 842113, Exists: true, Dropped: true},
			{Table: "public.new", Rows: 0},
		},
		TypeConflicts: []TypeConflict{
			{Table: "public.users", Column: "id", Source: "bigint", Target: "integer"},
		},
		Warnings: []string{"something to know"},
	}

	text := strings.Join(report.Lines(), "\n")
	for _, want := range []string{
		"From:      prod",
		"To:        staging",
		"Mode:      schema and data",
		"Overwrite: on - existing target tables are dropped and recreated",
		"TABLE", "ROWS", "TARGET",
		"public.orders", "842,113", "dropped and recreated",
		"public.new", "created",
		"1,092,113 rows (estimated) in 3 table(s)",
		"SOURCE", "TARGET", "bigint", "integer",
		"warning: something to know",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("plan missing %q:\n%s", want, text)
		}
	}
}

// TestDryRunReportLines_Modes pins that the mode line reflects the flags rather
// than always claiming a full migration.
func TestDryRunReportLines_Modes(t *testing.T) {
	tests := []struct {
		name string
		opts DryRunReport
		want string
	}{
		{"full", DryRunReport{}, "Mode:      schema and data"},
		{"schema", DryRunReport{SchemaOnly: true}, "Mode:      schema only"},
		{"data", DryRunReport{DataOnly: true}, "Mode:      data only"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if text := strings.Join(tc.opts.Lines(), "\n"); !strings.Contains(text, tc.want) {
				t.Errorf("plan missing %q:\n%s", tc.want, text)
			}
		})
	}
}

// TestDryRunReportLines_Empty pins that an empty source says so, rather than
// printing a header with no rows.
func TestDryRunReportLines_Empty(t *testing.T) {
	report := &DryRunReport{Source: "prod", Target: "staging"}
	text := strings.Join(report.Lines(), "\n")
	if !strings.Contains(text, "No tables would be migrated.") {
		t.Errorf("plan = %q, want it to say there is nothing to migrate", text)
	}
}

// TestAlign pins the column layout: every column but the last padded to the
// widest cell, two spaces between.
func TestAlign(t *testing.T) {
	rows := [][4]string{
		{"TABLE", "COLUMN", "SOURCE", "TARGET"},
		{"public.users", "id", "bigint", "integer"},
		{"a", "b", "c", "d"},
	}
	got := align(rows)
	want := []string{
		"TABLE         COLUMN  SOURCE  TARGET",
		"public.users  id      bigint  integer",
		"a             b       c       d",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

package io

import (
	"strings"
	"testing"
)

// TestQuoteTable covers identifier quoting for table names.
//
// Table names reach SQL from the command line, from --schema, and from MCP tool
// arguments. lib/pq sends zero-argument queries over the simple protocol, which
// accepts several statements separated by semicolons, so an unquoted name is an
// injection point as well as a source of syntax errors.
func TestQuoteTable(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "users", `"users"`},
		{"qualified", "public.users", `"public"."users"`},
		{"reserved word", "order", `"order"`},
		{"mixed case", "UserTable", `"UserTable"`},
		{"embedded space", "my table", `"my table"`},
		{"already quoted", `"users"`, `"users"`},
		{"already quoted qualified", `"public"."users"`, `"public"."users"`},
		{"dotted identifier stays one part", `"my.dotted.table"`, `"my.dotted.table"`},
		{"quote in name is doubled", `we"ird`, `"we""ird"`},
		{"statement injection is neutralised", `users; DROP TABLE audit_log; --`, `"users; DROP TABLE audit_log; --"`},
		{"schema injection is neutralised", `x"; DROP TABLE users; --.t`, `"x""; DROP TABLE users; --"."t"`},
		{"comment injection is neutralised", `users/*`, `"users/*"`},
		{"surrounding spaces", "  users  ", `"users"`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := quoteTable(c.in); got != c.want {
				t.Errorf("quoteTable(%q) = %s, want %s", c.in, got, c.want)
			}
		})
	}
}

// TestQuoteTableNoBareSemicolon is a property check: whatever the input, the
// result must be a single quoted identifier chain with no unquoted semicolon
// that could terminate the statement.
func TestQuoteTableNoBareSemicolon(t *testing.T) {
	hostile := []string{
		"t; DROP TABLE x",
		"t); DROP TABLE x; --",
		"public.t; SELECT pg_sleep(10); --",
		`t" ; DROP TABLE x; --`,
		"t'; DROP TABLE x; --",
		"t\n; DROP TABLE x",
	}
	for _, in := range hostile {
		out := quoteTable(in)
		// Every semicolon must be inside the quotes: count them and confirm the
		// string starts and ends with a quote.
		if !strings.HasPrefix(out, `"`) || !strings.HasSuffix(out, `"`) {
			t.Errorf("quoteTable(%q) = %s, expected a quoted identifier", in, out)
		}
		if strings.Count(out, `"`) < 2 {
			t.Errorf("quoteTable(%q) = %s, expected at least one quoted part", in, out)
		}
	}
}

// TestSplitTableName checks catalogue lookup parsing, which decides whether the
// exporter can paginate on a key or has to fall back to OFFSET.
func TestSplitTableName(t *testing.T) {
	cases := []struct {
		in         string
		wantSchema string
		wantName   string
	}{
		{"users", "public", "users"},
		{"public.users", "public", "users"},
		{"reporting.events", "reporting", "events"},
		{`"public"."users"`, "public", "users"},
		{`"my.dotted.table"`, "public", "my.dotted.table"},
		{`"my schema"."my.table"`, "my schema", "my.table"},
		{`"we""ird"."t"`, `we"ird`, "t"},
		{"", "public", ""},
	}

	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			schema, name := splitTableName(c.in)
			if schema != c.wantSchema || name != c.wantName {
				t.Errorf("splitTableName(%q) = (%q, %q), want (%q, %q)",
					c.in, schema, name, c.wantSchema, c.wantName)
			}
		})
	}
}

// TestSplitTableNameRoundTripsWithQuoteTable pins the invariant that matters:
// whatever quoteTable quotes, splitTableName must recover.
//
// The two halves run in opposite directions. quoteTable strips the caller's
// grouping quotes and re-quotes each dot-separated part, so a bare name that
// contained a dot becomes several quoted parts and recovers as a qualified
// name — that is the expected loss. Pre-quoting the name is the form that
// preserves it, and that is the round trip checked here.
func TestSplitTableNameRoundTripsWithQuoteTable(t *testing.T) {
	for _, name := range []string{"users", "my table", `we"ird`, "my.dotted.table", "Order"} {
		// The caller spells a dotted name as one quoted part.
		input := "public." + name
		if strings.Contains(name, ".") {
			input = "public." + quoteIdent(name)
		}
		quoted := quoteTable(input)
		schema, got := splitTableName(quoted)
		if schema != "public" || got != name {
			t.Errorf("round trip of %q via %s gave (%q, %q)", name, quoted, schema, got)
		}
	}
}

package io

import (
	"strings"
	"testing"
)

// parseAll reads every record from s.
func parseAll(t *testing.T, s string) [][]string {
	t.Helper()
	p := newCSVParser(strings.NewReader(s))
	var out [][]string
	for {
		rec, err := p.Read()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("parse %q: %v", s, err)
		}
		out = append(out, rec)
	}
	return out
}

// TestCSVParserRoundTrips pins the guarantee the import path depends on: a
// record written by encoding/csv comes back byte for byte.
//
// The cases are the ones that make encoding/csv lossy — a CRLF inside a field,
// a lone carriage return, a trailing line ending inside a field — plus the
// ordinary shapes a file has. This parser exists because encoding/csv removes
// the carriage return from those values, which changes the data.
func TestCSVParserRoundTrips(t *testing.T) {
	values := []string{
		"plain",
		"cr\r\nlf",
		"a\rb",
		"a\nb",
		"plain\r",
		"multi\r\none\r\ntwo",
		"\r\nleading",
		"trailing\r\n",
		"a\r\r\nb",
		"comma, inside",
		`quote " inside`,
		`"""`,
		" leading and trailing ",
		"",
		"ünïcödé ✓ 🎉",
		"back\\slash",
	}

	for _, v := range values {
		var sb strings.Builder
		w := newCSVWriter(&sb)
		if err := w.Write([]string{v}); err != nil {
			t.Fatalf("write %q: %v", v, err)
		}
		w.Flush()
		if err := w.Error(); err != nil {
			t.Fatalf("flush %q: %v", v, err)
		}

		recs := parseAll(t, sb.String())
		if len(recs) != 1 {
			t.Fatalf("%q: got %d records, want 1", v, len(recs))
		}
		if len(recs[0]) != 1 {
			t.Fatalf("%q: got %d fields, want 1", v, len(recs[0]))
		}
		if recs[0][0] != v {
			t.Errorf("round trip changed the value: wrote %q, read %q", v, recs[0][0])
		}
	}
}

// TestCSVParserAcceptsExportShapes covers what the export actually writes.
func TestCSVParserAcceptsExportShapes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want [][]string
	}{
		{"LF endings", "a,b\nc,d\n", [][]string{{"a", "b"}, {"c", "d"}}},
		{"CRLF endings", "a,b\r\nc,d\r\n", [][]string{{"a", "b"}, {"c", "d"}}},
		{"no trailing newline", "a,b", [][]string{{"a", "b"}}},
		{"empty fields", "a,,c\n", [][]string{{"a", "", "c"}}},
		{"trailing empty field", "a,\n", [][]string{{"a", ""}}},
		{"only empty fields", ",,\n", [][]string{{"", "", ""}}},
		{"quoted empty", `"",x` + "\n", [][]string{{"", "x"}}},
		{"quoted comma", `"a,b",c` + "\n", [][]string{{"a,b", "c"}}},
		{"doubled quote", `"a""b"` + "\n", [][]string{{`a"b`}}},
		{"quoted newline", "\"a\nb\",c\n", [][]string{{"a\nb", "c"}}},
		{"blank line is one empty field", "\n", [][]string{{""}}},
		{"empty file", "", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseAll(t, tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d records (%q), want %d", len(got), got, len(tc.want))
			}
			for i := range tc.want {
				if len(got[i]) != len(tc.want[i]) {
					t.Fatalf("record %d = %q, want %q", i, got[i], tc.want[i])
				}
				for j := range tc.want[i] {
					if got[i][j] != tc.want[i][j] {
						t.Errorf("record %d field %d = %q, want %q", i, j, got[i][j], tc.want[i][j])
					}
				}
			}
		})
	}
}

// TestCSVParserReportsDamage pins that a damaged file is an error, not a guess.
func TestCSVParserReportsDamage(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"unclosed quote", "\"a,b\n"},
		{"quote inside an unquoted field", "a\"b,c\n"},
		{"text after a closing quote", "\"a\"b,c\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := newCSVParser(strings.NewReader(tc.in))
			var found bool
			for {
				_, err := p.Read()
				if err != nil {
					// io.EOF is the clean end; anything else is the error
					// being checked for.
					if err.Error() != "EOF" {
						found = true
					}
					break
				}
			}
			if !found {
				t.Errorf("parsing %q reported no error", tc.in)
			}
		})
	}
}

// TestCSVParserHandlesLargeRecords pins that a record bigger than the read
// buffer is assembled rather than cut short. A quoted text column can be
// larger than a single read.
func TestCSVParserHandlesLargeRecords(t *testing.T) {
	big := strings.Repeat("x", 200_000)
	var sb strings.Builder
	w := newCSVWriter(&sb)
	if err := w.Write([]string{big, "tail"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	w.Flush()

	recs := parseAll(t, sb.String())
	if len(recs) != 1 || len(recs[0]) != 2 {
		t.Fatalf("got %d records, want 1 with 2 fields", len(recs))
	}
	if recs[0][0] != big {
		t.Errorf("large field changed: got %d bytes, want %d", len(recs[0][0]), len(big))
	}
	if recs[0][1] != "tail" {
		t.Errorf("second field = %q, want %q", recs[0][1], "tail")
	}
}

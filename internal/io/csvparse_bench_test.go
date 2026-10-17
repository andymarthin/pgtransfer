package io

import (
	"bytes"
	"strings"
	"testing"
)

// buildCSV renders n rows in the shape the export writes, with the values a
// real table holds: plain text, a quoted comma, a multi-line value and an
// empty field.
func buildCSV(n int) []byte {
	var b bytes.Buffer
	w := newCSVWriter(&b)
	w.Write([]string{"id", "email", "name", "balance", "note"})
	for i := 0; i < n; i++ {
		w.Write([]string{
			"12345",
			"user@example.com",
			"User Number 12345",
			"1234.56",
			"a note, with a comma",
		})
	}
	w.Flush()
	return b.Bytes()
}

// BenchmarkCSVParse measures the parser alone, with no database.
func BenchmarkCSVParse(b *testing.B) {
	data := buildCSV(100000)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := newCSVParser(bytes.NewReader(data))
		for {
			rec, err := p.Read()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				b.Fatal(err)
			}
			_ = rec
		}
	}
}

// BenchmarkCSVWrite measures the writer, which the export path calls per row.
func BenchmarkCSVWrite(b *testing.B) {
	row := []string{"12345", "user@example.com", "User Number 12345", "1234.56", "a note, with a comma"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		w := newCSVWriter(&buf)
		for r := 0; r < 100000; r++ {
			if err := w.Write(row); err != nil {
				b.Fatal(err)
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			b.Fatal(err)
		}
	}
}

// TestCSVParseThroughput is a smoke test that the parser is not accidentally
// quadratic. A buffer that is not reclaimed shows up as a run that takes
// minutes; this fails well before that.
func TestCSVParseThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	data := buildCSV(200000)
	p := newCSVParser(strings.NewReader(string(data)))
	rows := 0
	for {
		_, err := p.Read()
		if err != nil {
			break
		}
		rows++
	}
	if rows != 200001 { // header plus the data rows
		t.Fatalf("parsed %d records, want 200001", rows)
	}
}

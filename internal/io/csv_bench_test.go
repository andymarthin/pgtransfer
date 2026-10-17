package io

import (
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The benchmarks in this file measure the CPU-side cost of the CSV pipeline.
// They deliberately avoid a live database: the query layer is I/O bound and
// belongs in integration benchmarks, while FormatCSVValue and the writer loop
// are pure CPU and are where a regression would silently slow every export.

// benchValueKinds returns one representative value per type handled by
// FormatCSVValue, so benchmarks cover the whole type switch rather than just
// the common string case.
func benchValueKinds() map[string]interface{} {
	return map[string]interface{}{
		"nil":         nil,
		"string":      "some text value",
		"int64":       int64(1234567890),
		"int":         int(42),
		"float64":     float64(3.14159),
		"bool":        true,
		"bytes_text":  []byte("plain text"),
		"bytes_num":   []byte("123.45"),
		"date":        time.Date(2024, 5, 17, 0, 0, 0, 0, time.UTC),
		"timestamp":   time.Date(2024, 5, 17, 13, 45, 12, 0, time.UTC),
		"byte_array":  "[54 51 57 52 50 46 48 48]",
		"with_tz_str": "2024-05-17 13:45:12 +0000 +0000",
	}
}

func BenchmarkFormatCSVValue(b *testing.B) {
	for _, name := range []string{
		"nil", "string", "int64", "int", "float64", "bool",
		"bytes_text", "bytes_num", "date", "timestamp",
		"byte_array", "with_tz_str",
	} {
		v := benchValueKinds()[name]
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = FormatCSVValue(v)
			}
		})
	}
}

// BenchmarkFormatCSVValue_Column measures a realistic 8-column row, which is
// the unit of work repeated once per row during an export.
func BenchmarkFormatCSVValue_Column(b *testing.B) {
	row := []interface{}{
		int64(1),
		"a moderately long text column value",
		float64(99.95),
		true,
		time.Date(2024, 5, 17, 13, 45, 12, 0, time.UTC),
		[]byte("123.45"),
		nil,
		int64(1234567890),
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(row)))
	for i := 0; i < b.N; i++ {
		for _, v := range row {
			_ = FormatCSVValue(v)
		}
	}
}

// BenchmarkCSVWriteRows isolates encoding/csv write cost so that a future
// optimisation of the value formatting can be compared against the fixed cost
// of the standard library writer.
func BenchmarkCSVWriteRows(b *testing.B) {
	row := []string{
		"1", "a moderately long text column value", "99.95", "true",
		"2024-05-17 13:45:12", "123.45", "", "1234567890",
	}

	for _, rows := range []int{100, 1000, 10000} {
		b.Run(rowCount(rows), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(row) * rows))
			for i := 0; i < b.N; i++ {
				w := csv.NewWriter(io.Discard)
				for r := 0; r < rows; r++ {
					if err := w.Write(row); err != nil {
						b.Fatal(err)
					}
				}
				w.Flush()
				if err := w.Error(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkExportCSVPipeline runs the full value-formatting plus CSV-encoding
// path to a temporary file, without a database. It is the closest in-process
// approximation of export throughput.
func BenchmarkExportCSVPipeline(b *testing.B) {
	const rows = 10000
	row := []interface{}{
		int64(1),
		"a moderately long text column value",
		float64(99.95),
		true,
		time.Date(2024, 5, 17, 13, 45, 12, 0, time.UTC),
		[]byte("123.45"),
		nil,
		int64(1234567890),
	}
	record := make([]string, len(row))
	for i, v := range row {
		record[i] = FormatCSVValue(v)
	}

	dir := b.TempDir()
	path := filepath.Join(dir, "bench.csv")

	b.ReportAllocs()
	b.SetBytes(int64(len(record) * rows))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		f, err := os.Create(path)
		if err != nil {
			b.Fatal(err)
		}
		w := csv.NewWriter(f)
		for r := 0; r < rows; r++ {
			for c, v := range row {
				record[c] = FormatCSVValue(v)
			}
			if err := w.Write(record); err != nil {
				f.Close()
				b.Fatal(err)
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			f.Close()
			b.Fatal(err)
		}
		f.Close()
	}
}

// BenchmarkSplitCSVColumns measures parsing of a wide column list, which the
// import path performs once per import.
func BenchmarkSplitCSVColumns(b *testing.B) {
	inputs := map[string]string{
		"empty":  "   ",
		"single": "id",
		"wide":   "id, name, email, created_at, updated_at, status, amount, notes",
		"messy":  " id ,, name ,  email ,created_at, ,status",
	}
	for _, name := range []string{"empty", "single", "wide", "messy"} {
		in := inputs[name]
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = splitColumns(in)
			}
		})
	}
}

// splitColumns mirrors splitCSV in the mcpserver package so the shared parsing
// cost can be measured here without an import cycle.
func splitColumns(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := trimSpace(s[start:i])
			if part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	return out
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}

func rowCount(n int) string {
	switch n {
	case 100:
		return "100_rows"
	case 1000:
		return "1000_rows"
	case 10000:
		return "10000_rows"
	}
	return "rows"
}

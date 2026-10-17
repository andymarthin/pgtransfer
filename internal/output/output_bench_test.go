package output

import (
	"bytes"
	"io"
	"testing"
)

// benchData mimics the payload a typical command emits: a small struct with a
// handful of scalar fields. Benchmarks cover emission because every command run
// with --json pays this cost exactly once, and encoding is the only work the
// package does.
type benchData struct {
	Table        string `json:"table"`
	RowsExported int64  `json:"rows_exported"`
	Output       string `json:"output"`
	ElapsedMs    int64  `json:"elapsed_ms"`
}

// resetEmitted clears the at-most-once guard so each benchmark iteration
// measures a fresh invocation. SetWriters performs the reset under the lock.
func resetEmitted(w io.Writer) {
	SetWriters(w, w)
}

var benchPayload = benchData{
	Table:        "public.customers",
	RowsExported: 100000,
	Output:       "/tmp/export/customers.csv",
	ElapsedMs:    1234,
}

// BenchmarkSuccess measures the write path for a successful result, including
// the once-guard and JSON encoding into a discarded sink.
func BenchmarkSuccess(b *testing.B) {
	resetEmitted(io.Discard)
	SetJSON(true)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resetEmitted(io.Discard)
		Success("export csv", benchPayload)
	}
}

// BenchmarkSuccessForcedEncode encodes into a real buffer so the work is not
// optimised away and the byte volume is visible.
func BenchmarkSuccessForcedEncode(b *testing.B) {
	var buf bytes.Buffer
	resetEmitted(&buf)
	SetJSON(true)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		resetEmitted(&buf)
		Success("export csv", benchPayload)
	}
	b.StopTimer()
	b.SetBytes(int64(buf.Len()))
}

// BenchmarkFail measures the error envelope path.
func BenchmarkFail(b *testing.B) {
	resetEmitted(io.Discard)
	SetJSON(true)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resetEmitted(io.Discard)
		_ = Fail("export csv", "connection_failed", "failed to connect to %s: %v", "db:5432", "timeout")
	}
}

// BenchmarkIsJSON measures the mode check that sits in front of every output
// call, including the environment lookup fallback.
func BenchmarkIsJSON(b *testing.B) {
	SetJSON(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsJSON()
	}
}

// BenchmarkPrintErrorLineText measures the human-readable error path, which is
// taken on every failure in text mode.
func BenchmarkPrintErrorLineText(b *testing.B) {
	resetEmitted(io.Discard)
	SetJSON(false)
	err := NewError("connection_failed", "failed to connect to %s: %v", "db:5432", "timeout")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PrintErrorLine(err)
	}
}

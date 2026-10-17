package mcpserver

import "testing"

// The MCP server is driven by an AI client that may call tools in a tight loop,
// so the per-call argument handling and input decoding paths are benchmarked
// here. Tool bodies that touch a database are covered by integration tests.

func BenchmarkQualifyTable(b *testing.B) {
	cases := map[string]string{
		"bare":       "customers",
		"qualified":  "public.customers",
		"schema_url": "sales.orders",
	}
	for _, name := range []string{"bare", "qualified", "schema_url"} {
		in := cases[name]
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = qualifyTable(in)
			}
		})
	}
}

func BenchmarkSplitCSV(b *testing.B) {
	cases := map[string]string{
		"empty":  "",
		"single": "id",
		"wide":   "id,name,email,created_at,updated_at,status,amount,notes",
		"messy":  " id ,, name ,  email ,created_at, ,status",
	}
	for _, name := range []string{"empty", "single", "wide", "messy"} {
		in := cases[name]
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = splitCSV(in)
			}
		})
	}
}

// BenchmarkDecodeInput measures unmarshalling of the JSON arguments an MCP
// client sends with every tool call.
func BenchmarkDecodeInput(b *testing.B) {
	raw := []byte(`{"profile":"production","table":"public.customers","output":"/tmp/out.csv","include_headers":true}`)

	type input struct {
		Profile        string `json:"profile"`
		Table          string `json:"table"`
		Output         string `json:"output"`
		IncludeHeaders bool   `json:"include_headers"`
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var in input
		if err := decodeInput(raw, &in); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkOkResult measures construction of a successful tool result, which
// happens once per tool call.
func BenchmarkOkResult(b *testing.B) {
	payload := map[string]interface{}{
		"table":         "public.customers",
		"rows_exported": 100000,
		"output":        "/tmp/out.csv",
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := okResult(payload); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFailResult measures the in-band error result path.
func BenchmarkFailResult(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := failResult("unknown_profile", "profile 'x' not found"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNewServer measures construction of the server and its tool
// registrations. It runs once per process, so this is a startup-cost guard
// rather than a throughput benchmark.
func BenchmarkNewServer(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = New()
	}
}

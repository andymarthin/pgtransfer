package io

import (
	"database/sql/driver"
	"testing"
	"time"
)

// valuer is a helper implementing driver.Valuer for testing.
type valuer struct {
	v interface{}
}

func (x valuer) Value() (driver.Value, error) { return x.v, nil }

func TestFormatCSVValue(t *testing.T) {
	ts := time.Date(2026, 3, 15, 13, 45, 30, 0, time.UTC)
	dateOnly := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		in   interface{}
		want string
	}{
		{name: "nil", in: nil, want: ""},
		{name: "string", in: "hello", want: "hello"},
		{name: "int", in: 42, want: "42"},
		{name: "int64 negative", in: int64(-7), want: "-7"},
		{name: "uint", in: uint(9), want: "9"},
		{name: "float", in: 3.14, want: "3.14"},
		{name: "bool true", in: true, want: "true"},
		{name: "bool false", in: false, want: "false"},
		{name: "timestamp", in: ts, want: "2026-03-15 13:45:30"},
		{name: "date only", in: dateOnly, want: "2026-03-15"},
		{name: "bytes numeric", in: []byte("123.45"), want: "123.45"},
		{name: "bytes text", in: []byte("abc"), want: "abc"},
		{name: "valuer", in: valuer{v: int64(55)}, want: "55"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatCSVValue(tc.in)
			if got != tc.want {
				t.Fatalf("FormatCSVValue(%#v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDefaultCSVOptions(t *testing.T) {
	opts := DefaultCSVOptions()
	if opts.BatchSize != 500 {
		t.Fatalf("expected default BatchSize 500, got %d", opts.BatchSize)
	}
	if !opts.HasHeader {
		t.Fatalf("expected default HasHeader true")
	}
}

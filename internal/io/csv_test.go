package io

import (
	"database/sql/driver"
	"errors"
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

// valuerErr implements driver.Valuer and always errors.
type valuerErr struct{}

func (valuerErr) Value() (driver.Value, error) { return nil, errors.New("nope") }

// byteSlice is an arbitrary type that stringifies to a bracketed byte list.
type byteSlice []byte

func TestFormatCSVValue_DefaultBranches(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want string
	}{
		// A struct with no special handling falls through to the default case.
		{name: "plain struct", in: struct{ A int }{A: 7}, want: "{7}"},
		// Bracketed, space-separated byte list is decoded back to a string.
		{name: "byte array repr", in: byteSlice{104, 105}, want: "hi"},
		// Bracketed list containing a non-byte token is returned unchanged.
		{name: "invalid byte array", in: "[abc def]", want: "[abc def]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatCSVValue(tc.in); got != tc.want {
				t.Fatalf("FormatCSVValue(%#v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFormatCSVValue_ValuerError(t *testing.T) {
	// A Valuer whose Value() errors must fall back to a string representation
	// of the Valuer itself rather than panicking.
	got := FormatCSVValue(valuerErr{})
	if got == "" {
		t.Fatalf("expected a non-empty fallback for a failing Valuer")
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

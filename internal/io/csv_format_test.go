package io

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// TestFormatCSVValueStable pins the exact output for every numeric type so the
// switch from fmt.Sprintf to strconv cannot silently change the CSV format.
func TestFormatCSVValueStable(t *testing.T) {
	cases := []struct {
		name string
		in   interface{}
		want string
	}{
		{"int64", int64(1234567890), "1234567890"},
		{"int64 negative", int64(-42), "-42"},
		{"int64 zero", int64(0), "0"},
		{"int64 max", int64(math.MaxInt64), "9223372036854775807"},
		{"int64 min", int64(math.MinInt64), "-9223372036854775808"},
		{"int", int(42), "42"},
		{"int8", int8(-128), "-128"},
		{"int16", int16(32767), "32767"},
		{"int32", int32(-2147483648), "-2147483648"},
		{"uint64", uint64(18446744073709551615), "18446744073709551615"},
		{"uint", uint(7), "7"},
		{"uint8", uint8(255), "255"},
		{"uint16", uint16(65535), "65535"},
		{"uint32", uint32(4294967295), "4294967295"},
		{"float64 round", float64(99.95), "99.95"},
		{"float64 whole", float64(100), "100"},
		{"float64 small", float64(0.000001), "1e-06"},
		{"float64 neg", float64(-3.14159), "-3.14159"},
		{"float32", float32(1.5), "1.5"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"string", "hello", "hello"},
		{"nil", nil, ""},
		{"bytes numeric", []byte("123.45"), "123.45"},
		{"bytes text", []byte("abc"), "abc"},
		{
			"date only",
			time.Date(2024, 5, 17, 0, 0, 0, 0, time.UTC),
			"2024-05-17",
		},
		{
			"timestamp",
			time.Date(2024, 5, 17, 13, 45, 12, 0, time.UTC),
			"2024-05-17 13:45:12",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if got := FormatCSVValue(c.in); got != c.want {
				t.Errorf("FormatCSVValue(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestFormatCSVValueMatchesSprintf cross-checks the strconv fast paths against
// the fmt-based implementation they replaced, over a spread of representative
// values. Any divergence is a format change and a test failure.
func TestFormatCSVValueMatchesSprintf(t *testing.T) {
	ints := []interface{}{
		int64(0), int64(1), int64(-1), int64(1234567890), int64(math.MaxInt64), int64(math.MinInt64),
	}
	uints := []interface{}{
		uint64(0), uint64(1), uint64(18446744073709551615),
	}
	floats := []interface{}{
		float64(0), float64(1), float64(-1), float64(99.95), float64(1e-6),
		float64(1e21), float64(math.Pi), float64(math.MaxFloat64), float64(-0.0),
	}
	float32s := []interface{}{float32(1.5), float32(-0.25), float32(3.4e38)}

	for _, v := range ints {
		if got, want := FormatCSVValue(v), fmt.Sprintf("%d", v); got != want {
			t.Errorf("int %v: got %q want %q", v, got, want)
		}
	}
	for _, v := range uints {
		if got, want := FormatCSVValue(v), fmt.Sprintf("%d", v); got != want {
			t.Errorf("uint %v: got %q want %q", v, got, want)
		}
	}
	for _, v := range floats {
		if got, want := FormatCSVValue(v), fmt.Sprintf("%g", v); got != want {
			t.Errorf("float64 %v: got %q want %q", v, got, want)
		}
	}
	for _, v := range float32s {
		if got, want := FormatCSVValue(v), fmt.Sprintf("%g", v); got != want {
			t.Errorf("float32 %v: got %q want %q", v, got, want)
		}
	}
}

// TestFormatCSVValueTimeEdgeCases covers the hand-rolled date formatting
// against time.Format, including the single-digit components that a naive
// implementation gets wrong.
func TestFormatCSVValueTimeEdgeCases(t *testing.T) {
	times := []time.Time{
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 0, 0, 1, 0, time.UTC),
		time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
		time.Date(1999, 9, 9, 9, 9, 9, 0, time.UTC),
		time.Date(2000, 2, 29, 12, 0, 0, 0, time.UTC),
		time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 5, 17, 13, 45, 12, 0, time.UTC),
		time.Date(2024, 5, 17, 10, 5, 3, 0, time.UTC),
		// Non-zero nanoseconds still count as a timestamp, not a date.
		time.Date(2024, 5, 17, 0, 0, 0, 1, time.UTC),
	}

	for _, tm := range times {
		tm := tm
		want := tm.Format("2006-01-02 15:04:05")
		if tm.Hour() == 0 && tm.Minute() == 0 && tm.Second() == 0 && tm.Nanosecond() == 0 {
			want = tm.Format("2006-01-02")
		}
		t.Run(want, func(t *testing.T) {
			if got := FormatCSVValue(tm); got != want {
				t.Errorf("FormatCSVValue(%v) = %q, want %q", tm, got, want)
			}
		})
	}
}

// TestFormatCSVValueNoAllocForIntegers documents the performance contract: the
// integer fast path must not allocate beyond the returned string. Returning a
// string still allocates once for the caller, so the assertion targets
// conversions of values whose fmt path allocated twice (boxing + formatting).
func TestFormatCSVValueNoAllocForIntegers(t *testing.T) {
	v := int64(1234567890)
	got := testing.AllocsPerRun(100, func() {
		_ = FormatCSVValue(v)
	})
	if got > 1 {
		t.Errorf("int64 formatting does %v allocations per call, want at most 1", got)
	}
}

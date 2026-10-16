package utils

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		in   time.Duration
		want string
	}{
		{name: "milliseconds", in: 250 * time.Millisecond, want: "250ms"},
		{name: "sub-second", in: 999 * time.Millisecond, want: "999ms"},
		{name: "one second", in: time.Second, want: "1.00s"},
		{name: "seconds", in: 4500 * time.Millisecond, want: "4.50s"},
		{name: "minutes", in: 2*time.Minute + 30*time.Second, want: "2m30s"},
		{name: "zero", in: 0, want: "0ms"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatDuration(tc.in); got != tc.want {
				t.Fatalf("FormatDuration(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

package io

import "testing"

func TestNewProgressBarWithTimer_KnownTotal(t *testing.T) {
	bar := NewProgressBarWithTimer(10, "test")
	if bar == nil {
		t.Fatalf("expected non-nil bar")
	}
	if bar.GetMax() != 10 {
		t.Fatalf("expected max 10, got %d", bar.GetMax())
	}
}

func TestNewProgressBarWithTimer_UnknownTotal(t *testing.T) {
	// A non-positive total (e.g. -1 from a failed COUNT(*)) must still yield a
	// usable, indeterminate bar rather than panicking.
	for _, total := range []int64{0, -1} {
		bar := NewProgressBarWithTimer(total, "test")
		if bar == nil {
			t.Fatalf("expected non-nil bar for total %d", total)
		}
	}
}

func TestNewProgressBarWithTimer_AddAndFinish(t *testing.T) {
	bar := NewProgressBarWithTimer(3, "test")
	bar.Add(1)
	bar.Add(1)
	bar.Add(1)
	if err := bar.Finish(); err != nil {
		t.Fatalf("unexpected finish error: %v", err)
	}
	if !bar.IsFinished() {
		t.Fatalf("expected bar to be finished")
	}
}

package io

import (
	"fmt"
	"time"

	"github.com/schollz/progressbar/v3"
)

// NewProgressBarWithTimer creates a progress bar (or a spinner when the total
// is unknown/non-positive) and starts a goroutine that continuously appends an
// elapsed-time suffix to the bar's description.
func NewProgressBarWithTimer(total int64, description string) *progressbar.ProgressBar {
	start := time.Now()

	// If total is unknown (0 or negative), create an indeterminate spinner
	// instead of a progress bar. Callers pass -1 as a fallback when a COUNT(*)
	// query fails, so we must treat any non-positive value as "unknown".
	if total <= 0 {
		bar := progressbar.NewOptions64(
			-1, // -1 creates an indeterminate progress bar (spinner)
			progressbar.OptionSetDescription(description),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionSetWidth(40),
			progressbar.OptionThrottle(100*time.Millisecond),
			progressbar.OptionSetRenderBlankState(true),
		)

		go func() {
			for !bar.IsFinished() {
				elapsed := time.Since(start).Round(time.Second)
				bar.Describe(fmt.Sprintf("%s (elapsed %s)", description, elapsed))
				time.Sleep(time.Second)
			}
		}()

		return bar
	}

	// Original progress bar for known totals.
	bar := progressbar.NewOptions64(
		total,
		progressbar.OptionSetDescription(description),
		progressbar.OptionShowCount(),
		progressbar.OptionFullWidth(),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionSetWidth(20),
		progressbar.OptionShowIts(),
		progressbar.OptionSetRenderBlankState(true),
	)

	go func() {
		for !bar.IsFinished() {
			elapsed := time.Since(start).Round(time.Second)
			bar.Describe(fmt.Sprintf("%s (elapsed %s)", description, elapsed))
			time.Sleep(time.Second)
		}
	}()

	return bar
}

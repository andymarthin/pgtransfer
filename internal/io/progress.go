package io

import (
	"fmt"
	"time"

	"github.com/schollz/progressbar/v3"
)

func NewProgressBarWithTimer(total int64, description string) *progressbar.ProgressBar {
	start := time.Now()

	// If total is 0, create a spinner instead of a progress bar
	if total == 0 {
		bar := progressbar.NewOptions64(
			-1, // -1 creates an indeterminate progress bar (spinner)
			progressbar.OptionSetDescription(description),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionSetWidth(40),
			progressbar.OptionThrottle(100*time.Millisecond),
			progressbar.OptionSetRenderBlankState(true),
		)

		go func() {
			for {
				elapsed := time.Since(start).Round(time.Second)
				bar.Describe(fmt.Sprintf("%s (elapsed %s)", description, elapsed))
				time.Sleep(time.Second)
			}
		}()

		return bar
	}

	// Original progress bar for known totals
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
		for bar.State().CurrentNum < total {
			elapsed := time.Since(start).Round(time.Second)
			bar.Describe(fmt.Sprintf("%s (elapsed %s)", description, elapsed))
			time.Sleep(time.Second)
		}
	}()

	return bar
}

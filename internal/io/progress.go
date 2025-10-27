package io

import (
	"fmt"
	"time"

	"github.com/schollz/progressbar/v3"
)

func NewProgressBarWithTimer(total int64, description string) *progressbar.ProgressBar {
	start := time.Now()

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

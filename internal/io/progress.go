package io

import (
	"fmt"
	"time"

	"github.com/schollz/progressbar/v3"
)

// Bar is a progress bar whose elapsed-time goroutine is tied to its lifetime.
//
// The elapsed-time suffix is driven by a goroutine that polls the bar. Stopping
// it on IsFinished alone leaves that goroutine alive for any caller that never
// finishes the bar — an error return, a cancelled command — for the rest of the
// process. A CLI that runs one command and exits does not care; a process that
// hosts the MCP tools, or a test binary running thousands of cases, accumulates
// one leaked goroutine per bar.
//
// Wrapping the progress bar means every existing Finish call releases the
// goroutine, because Finish is the method being called.
type Bar struct {
	*progressbar.ProgressBar
	stop chan struct{}
}

// Add advances the bar.
func (b *Bar) Add(n int) error {
	if b == nil || b.ProgressBar == nil {
		return nil
	}
	return b.ProgressBar.Add(n)
}

// Describe replaces the bar description, keeping the elapsed-time suffix.
func (b *Bar) Describe(description string) {
	if b == nil || b.ProgressBar == nil {
		return
	}
	b.ProgressBar.Describe(description)
}

// Finish completes the bar and stops its elapsed-time goroutine. Safe to call
// more than once.
func (b *Bar) Finish() error {
	if b == nil {
		return nil
	}
	b.stopTimer()
	if b.ProgressBar == nil {
		return nil
	}
	return b.ProgressBar.Finish()
}

// stopTimer ends the elapsed-time goroutine. Safe to call more than once.
func (b *Bar) stopTimer() {
	select {
	case <-b.stop:
		// already closed
	default:
		close(b.stop)
	}
}

// NewProgressBarWithTimer creates a progress bar (or a spinner when the total
// is unknown/non-positive) and starts a goroutine that continuously appends an
// elapsed-time suffix to the bar's description.
//
// Call Finish on the returned bar, normally with defer, so the goroutine ends
// on every path out of the function.
func NewProgressBarWithTimer(total int64, description string) *Bar {
	start := time.Now()

	var inner *progressbar.ProgressBar
	if total <= 0 {
		// Unknown total: an indeterminate spinner. Callers pass -1 as a
		// fallback when a COUNT(*) query fails, so any non-positive value means
		// "unknown".
		inner = progressbar.NewOptions64(
			-1, // -1 creates an indeterminate progress bar (spinner)
			progressbar.OptionSetDescription(description),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionSetWidth(40),
			progressbar.OptionThrottle(100*time.Millisecond),
			progressbar.OptionSetRenderBlankState(true),
		)
	} else {
		inner = progressbar.NewOptions64(
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
	}

	bar := &Bar{ProgressBar: inner, stop: make(chan struct{})}
	go bar.describeElapsed(start, description)
	return bar
}

// describeElapsed appends "(elapsed Ns)" to the description once a second until
// the bar is finished or Finish releases it.
func (b *Bar) describeElapsed(start time.Time, description string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-b.stop:
			return
		case <-ticker.C:
			if b.ProgressBar.IsFinished() {
				return
			}
			elapsed := time.Since(start).Round(time.Second)
			b.ProgressBar.Describe(fmt.Sprintf("%s (elapsed %s)", description, elapsed))
		}
	}
}

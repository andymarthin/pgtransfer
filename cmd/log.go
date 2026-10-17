package cmd

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/andymarthin/pgtransfer/internal/log"
	"github.com/andymarthin/pgtransfer/internal/output"
	"github.com/andymarthin/pgtransfer/internal/utils"
	"github.com/spf13/cobra"
)

var (
	logDate  string
	logLimit int
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Show recorded operations",
	Long: `Show the operations pgtransfer has recorded, newest last.

Migrating a database writes an entry per run to ~/.pgtransfer/logs/YYYY-MM-DD.log.
This command reads them back. With no arguments it shows today.

Use it to answer "what did the last migration do, and did it finish?"`,
	Example: `  # Today
  pgtransfer log

  # A specific day
  pgtransfer log --date 2026-10-17

  # The last 5 entries today
  pgtransfer log --limit 5

  # Machine-readable
  pgtransfer --json log --date 2026-10-17`,
	Args: cobra.NoArgs,
	RunE: runLog,
}

func runLog(cmd *cobra.Command, args []string) error {
	if logDate != "" {
		if _, err := time.Parse("2006-01-02", logDate); err != nil {
			return output.Fail("log", "usage_error", "invalid --date %q: use YYYY-MM-DD", logDate)
		}
	}

	entries, err := log.Entries(logDate)
	if err != nil {
		return output.Fail("log", "config_load_failed", "%v", err)
	}

	day := logDate
	if day == "" {
		day = time.Now().Format("2006-01-02")
	}

	// Apply the limit after reading, so the newest entries are the ones kept.
	total := len(entries)
	if logLimit > 0 && len(entries) > logLimit {
		entries = entries[len(entries)-logLimit:]
	}

	if output.IsJSON() {
		if entries == nil {
			entries = []log.Entry{}
		}
		output.Success("log", map[string]interface{}{
			"date":    day,
			"entries": entries,
			"total":   total,
		})
		return nil
	}

	if len(entries) == 0 {
		if logDate == "" {
			utils.PrintInfo(cmd, "No operations recorded today")
		} else {
			utils.PrintInfo(cmd, "No operations recorded on %s", day)
		}
		if dates := log.AvailableDates(); len(dates) > 0 {
			utils.PrintInfo(cmd, "Days with entries: %s", strings.Join(dates, ", "))
		}
		return nil
	}

	title := fmt.Sprintf("📘 Operations on %s", day)
	utils.PrintTitle(cmd, title)

	// One writer for the whole page, so a caller that redirects output (a test,
	// or a future pager) captures every line instead of only the rows.
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tSTATUS\tCOMMAND\tPROFILE\tDETAIL")
	for _, e := range entries {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			shortTime(e.Timestamp),
			statusLabel(e.Status),
			e.Command,
			dashIfEmpty(e.Profile),
			detail(e),
		)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if total > len(entries) {
		fmt.Fprintf(&buf, "\nShowing the last %d of %d entries. Use --limit to see more.\n", len(entries), total)
	}
	_, err = io.Copy(cmd.OutOrStdout(), &buf)
	return err
}

// shortTime renders an RFC3339 timestamp as a local wall clock time.
func shortTime(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.Local().Format("15:04:05")
}

// statusLabel turns a recorded status into a fixed-width marker. The width is
// padded rather than left to the emoji so the tabwriter columns do not depend
// on how the terminal measures a two-codepoint glyph.
func statusLabel(status string) string {
	switch strings.ToUpper(status) {
	case "SUCCESS":
		return "ok  "
	case "FAILURE":
		return "fail"
	case "INFO":
		return "info"
	default:
		return strings.ToUpper(status)
	}
}

// detail combines the message with the duration when there is one.
func detail(e log.Entry) string {
	if e.Duration == "" {
		return e.Message
	}
	return fmt.Sprintf("%s (%s)", e.Message, e.Duration)
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func init() {
	rootCmd.AddCommand(logCmd)
	logCmd.Flags().StringVar(&logDate, "date", "", "Day to show, as YYYY-MM-DD (default: today)")
	logCmd.Flags().IntVar(&logLimit, "limit", 0, "Show only the last N entries")
}

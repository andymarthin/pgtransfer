package utils

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorGray   = "\033[90m"
	ColorBold   = "\033[1m"
)

// PrintSuccess prints a green success message.
func PrintSuccess(cmd *cobra.Command, format string, args ...interface{}) {
	printCmd(cmd, ColorGreen+"✅ "+format+ColorReset+"\n", args...)
}

// PrintError prints a red error message.
func PrintError(cmd *cobra.Command, format string, args ...interface{}) {
	fmt.Fprintf(writerFor(cmd, false), ColorRed+"❌ "+format+ColorReset+"\n", args...)
}

// PrintWarning prints a yellow warning message.
func PrintWarning(cmd *cobra.Command, format string, args ...interface{}) {
	printCmd(cmd, ColorYellow+"⚠️  "+format+ColorReset+"\n", args...)
}

// PrintInfo prints a cyan informational message.
func PrintInfo(cmd *cobra.Command, format string, args ...interface{}) {
	printCmd(cmd, ColorCyan+"ℹ️  "+format+ColorReset+"\n", args...)
}

// PrintNote prints a blue note-style message.
func PrintNote(cmd *cobra.Command, format string, args ...interface{}) {
	printCmd(cmd, ColorBlue+"🔷 "+format+ColorReset+"\n", args...)
}

// PrintMuted prints a gray, low-importance message.
func PrintMuted(cmd *cobra.Command, format string, args ...interface{}) {
	printCmd(cmd, ColorGray+format+ColorReset+"\n", args...)
}

// PrintTitle prints a bold section title, often used for CLI headers.
func PrintTitle(cmd *cobra.Command, title string) {
	formatted := fmt.Sprintf("\n%s%s%s%s\n", ColorBold, ColorBlue, title, ColorReset)
	if cmd != nil {
		fmt.Fprint(cmd.OutOrStdout(), formatted)
	} else {
		fmt.Fprint(os.Stdout, formatted)
	}
}

// PrintDivider prints a cyan horizontal divider.
func PrintDivider(cmd *cobra.Command) {
	printCmd(cmd, ColorCyan+"----------------------------------------"+ColorReset+"\n")
}

// Internal helpers to avoid nil pointer panics if cmd is nil
func printCmd(cmd *cobra.Command, format string, args ...interface{}) {
	fmt.Fprintf(writerFor(cmd, false), format, args...)
}

// writerFor resolves where a message should go. A nil command means the process
// streams, os.Stdout or os.Stderr; a command means its cobra streams, which is
// what makes output redirectable in-process.
//
// The store is re-read on every call rather than captured once, so a test that
// replaces os.Stdout after package initialisation still captures the output.
func writerFor(cmd *cobra.Command, stderr bool) io.Writer {
	if stderr {
		if cmd == nil {
			return os.Stderr
		}
		return cmd.ErrOrStderr()
	}
	if cmd == nil {
		return os.Stdout
	}
	return cmd.OutOrStdout()
}

// errWriter is the destination for error output.
func errWriter(cmd *cobra.Command) io.Writer { return writerFor(cmd, true) }

// Inline color helpers for other packages (e.g., log viewer)
func ColorTextGreen(s string) string  { return ColorGreen + s + ColorReset }
func ColorTextRed(s string) string    { return ColorRed + s + ColorReset }
func ColorTextCyan(s string) string   { return ColorCyan + s + ColorReset }
func ColorTextBlue(s string) string   { return ColorBlue + s + ColorReset }
func ColorTextYellow(s string) string { return ColorYellow + s + ColorReset }
func ColorTextGray(s string) string   { return ColorGray + s + ColorReset }
func ColorTextBold(s string) string   { return ColorBold + s + ColorReset }

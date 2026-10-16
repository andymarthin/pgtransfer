package cmd

import (
	"fmt"
	"runtime"

	"github.com/andymarthin/pgtransfer/internal/output"
	"github.com/spf13/cobra"
)

// Version information (set during build)
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Run: func(cmd *cobra.Command, args []string) {
		if output.IsJSON() {
			output.Success("version", versionInfo())
			return
		}
		PrintVersion()
	},
}

// versionInfo is the machine-readable form of the version output.
func versionInfo() map[string]interface{} {
	return map[string]interface{}{
		"version": Version,
		"commit":  Commit,
		"date":    Date,
		"go":      runtime.Version(),
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
	}
}

// PrintVersion writes the version, commit and build date to stdout. It is
// shared between the `version` subcommand and the top-level --version/-v flags.
func PrintVersion() {
	fmt.Printf("pgtransfer %s\n", Version)
	fmt.Printf("Commit: %s\n", Commit)
	fmt.Printf("Built: %s\n", Date)
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate("pgtransfer {{.Version}}\n")
}

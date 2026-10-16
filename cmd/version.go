package cmd

import (
	"fmt"

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
		PrintVersion()
	},
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

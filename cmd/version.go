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
		fmt.Printf("pgtransfer %s\n", Version)
		fmt.Printf("Commit: %s\n", Commit)
		fmt.Printf("Built: %s\n", Date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
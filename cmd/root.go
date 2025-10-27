package cmd

import (
	"github.com/andymarthin/pgtransfer/cmd/profile"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pgtransfer",
	Short: "Transfer PostgreSQL data between databases or CSV files",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(profile.ProfileCmd)
	rootCmd.AddCommand(testConnectionCmd)
}

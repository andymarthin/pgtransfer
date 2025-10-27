package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var profileName, profileDB string

var addProfileCmd = &cobra.Command{
	Use:   "profile:add",
	Short: "Add or update a database profile",
	Run: func(cmd *cobra.Command, args []string) {
		if profileName == "" || profileDB == "" {
			fmt.Println("Usage: pgtransfer profile:add --name <name> --db <url>")
			return
		}
		cfg, _ := loadConfig()
		cfg.Profiles[profileName] = Profile{DbURL: profileDB}
		saveConfig(cfg)
		fmt.Printf("✅ Profile '%s' saved.\n", profileName)
	},
}

func init() {
	addProfileCmd.Flags().StringVar(&profileName, "name", "", "Profile name")
	addProfileCmd.Flags().StringVar(&profileDB, "db", "", "Database URL")
	rootCmd.AddCommand(addProfileCmd)
}

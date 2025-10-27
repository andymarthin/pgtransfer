package profile

import (
	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/utils"
	"github.com/spf13/cobra"
)

var activeCmd = &cobra.Command{
	Use:   "active",
	Short: "Show the currently active profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			utils.PrintError(cmd, "Failed to load configuration: %v", err)
			return err
		}

		if cfg.ActiveProfile == "" {
			utils.PrintWarning(cmd, "No active profile selected.")
			utils.PrintMuted(cmd, `Use "pgtransfer profile use <name>" to set one.`)
			return nil
		}

		profile, ok := cfg.Profiles[cfg.ActiveProfile]
		if !ok {
			utils.PrintError(cmd, "Active profile '%s' not found in config.", cfg.ActiveProfile)
			return nil
		}

		utils.PrintTitle(cmd, "Active Profile")
		utils.PrintDivider(cmd)
		utils.PrintInfo(cmd, "Name: %s", profile.Name)
		utils.PrintInfo(cmd, "Host: %s", profile.Host)
		utils.PrintInfo(cmd, "Port: %d", profile.Port)
		utils.PrintInfo(cmd, "Database: %s", profile.Database)
		utils.PrintInfo(cmd, "User: %s", profile.User)

		if profile.SSH.Enabled {
			utils.PrintNote(cmd, "SSH: %s@%s:%d", profile.SSH.User, profile.SSH.Host, profile.SSH.Port)
		}

		return nil
	},
}

func init() {
	ProfileCmd.AddCommand(activeCmd)
}

package profile

import (
	"fmt"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/utils"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.ListProfiles()
		if err != nil {
			utils.PrintError(cmd, "Failed to load profiles: %v", err)
			return err
		}

		if len(cfg.Profiles) == 0 {
			utils.PrintInfo(cmd, "No profiles found.")
			return nil
		}

		utils.PrintTitle(cmd, "Profiles:")
		utils.PrintDivider(cmd)

		for name, p := range cfg.Profiles {
			activeMark := ""
			if cfg.ActiveProfile == name {
				activeMark = utils.ColorTextGreen(" (active)")
			}

			fmt.Printf("%s%s\n", name, activeMark)
			fmt.Printf("  Host: %s:%d  DB: %s  User: %s\n", p.Host, p.Port, p.Database, p.User)
			if p.SSH.Enabled {
				fmt.Printf("  SSH: %s@%s:%d\n", p.SSH.User, p.SSH.Host, p.SSH.Port)
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	ProfileCmd.AddCommand(listCmd)
}

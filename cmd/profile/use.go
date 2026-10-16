package profile

import (
	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/output"
	"github.com/andymarthin/pgtransfer/internal/utils"
	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the active profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := config.SetActiveProfile(name); err != nil {
			if output.IsJSON() {
				return output.Fail("profile use", "profile_not_found",
					"could not set active profile %q: %v", name, err)
			}
			return err
		}
		if output.IsJSON() {
			output.Success("profile use", map[string]interface{}{"active_profile": name})
			return nil
		}
		utils.PrintSuccess(cmd, "Profile '%s' is now active", name)
		return nil
	},
}

func init() {
	ProfileCmd.AddCommand(useCmd)
}

package profile

import (
	"fmt"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/output"
	"github.com/andymarthin/pgtransfer/internal/utils"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <profile-name>",
	Short: "Delete a saved profile",
	Long: `Delete a saved PostgreSQL connection profile.

If the deleted profile is currently active, the active profile will be unset.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]

		// Check if profile exists
		exists, err := config.ProfileExists(profileName)
		if err != nil {
			if output.IsJSON() {
				return output.Fail("profile delete", "config_load_failed", "failed to check profile existence: %v", err)
			}
			utils.PrintError(cmd, "Failed to check profile existence: %v", err)
			return err
		}

		if !exists {
			if output.IsJSON() {
				return output.Fail("profile delete", "profile_not_found", "profile %q does not exist", profileName)
			}
			utils.PrintError(cmd, "Profile '%s' does not exist", profileName)
			return fmt.Errorf("profile '%s' does not exist", profileName)
		}

		// Delete the profile
		if err := config.DeleteProfile(profileName); err != nil {
			if output.IsJSON() {
				return output.Fail("profile delete", "delete_failed", "failed to delete profile: %v", err)
			}
			utils.PrintError(cmd, "Failed to delete profile: %v", err)
			return err
		}

		if output.IsJSON() {
			output.Success("profile delete", map[string]interface{}{"deleted": profileName})
			return nil
		}

		utils.PrintSuccess(cmd, "Profile '%s' deleted successfully", profileName)
		return nil
	},
}

func init() {
	ProfileCmd.AddCommand(deleteCmd)
}

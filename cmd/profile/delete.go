package profile

import (
	"fmt"

	"github.com/andymarthin/pgtransfer/internal/config"
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
			utils.PrintError(cmd, "Failed to check profile existence: %v", err)
			return err
		}

		if !exists {
			utils.PrintError(cmd, "Profile '%s' does not exist", profileName)
			return fmt.Errorf("profile '%s' does not exist", profileName)
		}

		// Delete the profile
		if err := config.DeleteProfile(profileName); err != nil {
			utils.PrintError(cmd, "Failed to delete profile: %v", err)
			return err
		}

		utils.PrintSuccess(cmd, "Profile '%s' deleted successfully", profileName)
		return nil
	},
}

func init() {
	ProfileCmd.AddCommand(deleteCmd)
}

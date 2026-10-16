package profile

import (
	"fmt"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/output"
	"github.com/andymarthin/pgtransfer/internal/utils"
	"github.com/spf13/cobra"
)

// profileView is the machine-readable, secret-free representation of a profile.
// Passwords and passphrases are replaced by a boolean has_password flag so that
// JSON output is always safe to log or hand to an agent.
type profileView struct {
	Name        string `json:"name"`
	User        string `json:"user,omitempty"`
	Host        string `json:"host,omitempty"`
	Port        int    `json:"port,omitempty"`
	Database    string `json:"database,omitempty"`
	SSLMode     string `json:"sslmode,omitempty"`
	HasPassword bool   `json:"has_password"`
	UsesURL     bool   `json:"uses_url"`
	SSHEnabled  bool   `json:"ssh_enabled"`
	SSHUser     string `json:"ssh_user,omitempty"`
	SSHHost     string `json:"ssh_host,omitempty"`
	SSHPort     int    `json:"ssh_port,omitempty"`
	Active      bool   `json:"active"`
}

// viewProfile converts a stored profile into its redacted view.
func viewProfile(name string, p config.Profile, active bool) profileView {
	return profileView{
		Name:        name,
		User:        p.User,
		Host:        p.Host,
		Port:        p.Port,
		Database:    p.Database,
		SSLMode:     p.SSLMode,
		HasPassword: p.Password != "",
		UsesURL:     p.DBURL != "",
		SSHEnabled:  p.SSH.Enabled,
		SSHUser:     p.SSH.User,
		SSHHost:     p.SSH.Host,
		SSHPort:     p.SSH.Port,
		Active:      active,
	}
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.ListProfiles()
		if err != nil {
			if output.IsJSON() {
				return output.Fail("profile list", "config_load_failed", "failed to load profiles: %v", err)
			}
			utils.PrintError(cmd, "Failed to load profiles: %v", err)
			return err
		}

		if output.IsJSON() {
			profiles := make([]profileView, 0, len(cfg.Profiles))
			for name, p := range cfg.Profiles {
				profiles = append(profiles, viewProfile(name, p, cfg.ActiveProfile == name))
			}
			output.Success("profile list", map[string]interface{}{
				"active_profile": cfg.ActiveProfile,
				"count":          len(profiles),
				"profiles":       profiles,
			})
			return nil
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

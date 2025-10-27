package profile

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/db"
	"github.com/andymarthin/pgtransfer/internal/utils"
	"github.com/spf13/cobra"
)

var (
	flagUser, flagPassword, flagHost, flagDbURL, flagDatabase          string
	flagSSHHost, flagSSHUser, flagSSHKey, flagSSHPassword, flagSSHPassphrase string
	flagSSLMode                                                        string
	flagPort, flagSSHPort, flagSSHTimeout                              int
	flagForce, flagInteractive                                         bool
)

var addCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Create or update a connection profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		reader := bufio.NewReader(os.Stdin)

		exists, err := config.ProfileExists(name)
		if err != nil {
			utils.PrintError(cmd, "Failed to load config: %v", err)
			return err
		}

		// Ask confirmation if profile exists and not using --force
		if exists && !flagForce {
			utils.PrintWarning(cmd, "Profile '%s' already exists.", name)
			fmt.Print("Do you want to overwrite it? [y/N]: ")
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				utils.PrintMuted(cmd, "Cancelled profile overwrite.")
				return nil
			}
		}

		var p config.Profile

		if flagInteractive {
			p = promptProfileInput(cmd, name)
		} else {
			p = config.Profile{
				Name:     name,
				User:     flagUser,
				Password: flagPassword,
				Host:     flagHost,
				Port:     flagPort,
				Database: flagDatabase,
				DBURL:    flagDbURL,
				SSLMode:  flagSSLMode,
				SSH: config.SSHConfig{
					Enabled:    flagSSHHost != "",
					User:       flagSSHUser,
					Host:       flagSSHHost,
					Port:       flagSSHPort,
					KeyPath:    flagSSHKey,
					Passphrase: flagSSHPassphrase,
					Password:   flagSSHPassword,
					Timeout:    flagSSHTimeout,
				},
			}
		}

		utils.PrintInfo(cmd, "Validating connection for profile '%s'...", name)
		if err := config.AddOrUpdateProfile(p, db.TestConnection); err != nil {
			utils.PrintError(cmd, "Failed to save profile: %v", err)
			return err
		}

		utils.PrintSuccess(cmd, "Profile '%s' saved successfully.", name)
		return nil
	},
}

func init() {
	ProfileCmd.AddCommand(addCmd)

	// Database options
	addCmd.Flags().StringVar(&flagDbURL, "db", "", "Database connection URL (overrides other options)")
	addCmd.Flags().StringVar(&flagUser, "user", "", "Database user")
	addCmd.Flags().StringVar(&flagPassword, "password", "", "Database password")
	addCmd.Flags().StringVar(&flagHost, "host", "localhost", "Database host")
	addCmd.Flags().IntVar(&flagPort, "port", 5432, "Database port")
	addCmd.Flags().StringVar(&flagDatabase, "database", "", "Database name")
	addCmd.Flags().StringVar(&flagSSLMode, "sslmode", "disable", "SSL mode (disable, require, verify-full)")

	// SSH options
	addCmd.Flags().StringVar(&flagSSHHost, "ssh-host", "", "SSH host (optional)")
	addCmd.Flags().StringVar(&flagSSHUser, "ssh-user", "", "SSH username")
	addCmd.Flags().StringVar(&flagSSHKey, "ssh-key", "", "SSH private key path")
	addCmd.Flags().StringVar(&flagSSHPassphrase, "ssh-passphrase", "", "SSH private key passphrase (optional)")
	addCmd.Flags().StringVar(&flagSSHPassword, "ssh-password", "", "SSH password (optional)")
	addCmd.Flags().IntVar(&flagSSHPort, "ssh-port", 22, "SSH port")
	addCmd.Flags().IntVar(&flagSSHTimeout, "ssh-timeout", 10, "SSH timeout in seconds")

	addCmd.Flags().BoolVar(&flagForce, "force", false, "Overwrite if profile exists without prompt")
	addCmd.Flags().BoolVarP(&flagInteractive, "interactive", "i", false, "Run in interactive mode")
}

// interactive prompt helper
func promptProfileInput(cmd *cobra.Command, name string) config.Profile {
	reader := bufio.NewReader(os.Stdin)
	utils.PrintTitle(cmd, "🧩 Interactive Profile Setup")

	prompt := func(label, defaultValue string) string {
		if defaultValue != "" {
			fmt.Printf("%s [%s]: ", label, defaultValue)
		} else {
			fmt.Printf("%s: ", label)
		}
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "" {
			return defaultValue
		}
		return text
	}

	user := prompt("Database user", "postgres")
	fmt.Print("Database password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)
	host := prompt("Database host", "localhost")
	portStr := prompt("Database port", "5432")
	port, _ := strconv.Atoi(portStr)
	database := prompt("Database name", "")
	sslmode := prompt("SSL mode", "disable")

	useSSH := strings.ToLower(prompt("Use SSH tunnel? (y/N)", "n")) == "y"
	var sshCfg config.SSHConfig
	if useSSH {
		sshCfg.Enabled = true
		sshCfg.Host = prompt("SSH host", "")
		sshCfg.User = prompt("SSH user", "")
		portStr := prompt("SSH port", "22")
		sshCfg.Port, _ = strconv.Atoi(portStr)
		
		// Ask user to choose authentication method
		authMethod := strings.ToLower(prompt("SSH authentication method - use (k)ey or (p)assword? [k/p]", "k"))
		if authMethod == "p" || authMethod == "password" {
			sshCfg.Password = prompt("SSH password", "")
		} else {
			sshCfg.KeyPath = prompt("SSH key path", "")
		}
		
		timeoutStr := prompt("SSH timeout (seconds)", "10")
		sshCfg.Timeout, _ = strconv.Atoi(timeoutStr)
	}

	return config.Profile{
		Name:     name,
		User:     user,
		Password: password,
		Host:     host,
		Port:     port,
		Database: database,
		SSLMode:  sslmode,
		SSH:      sshCfg,
	}
}

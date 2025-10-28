package profile

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/db"
	"github.com/andymarthin/pgtransfer/internal/utils"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	flagUser, flagPassword, flagHost, flagDbURL, flagDatabase          string
	flagSSHHost, flagSSHUser, flagSSHKey, flagSSHPassword, flagSSHPassphrase string
	flagSSLMode                                                        string
	flagPort, flagSSHPort, flagSSHTimeout                              int
	flagForce, flagInteractive, flagSkipTest, flagNonInteractive       bool
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
			// Load and display current profile values
			cfg, err := config.LoadConfig()
			if err == nil {
				if currentProfile, ok := cfg.Profiles[name]; ok {
					utils.PrintWarning(cmd, "Profile '%s' already exists with the following configuration:", name)
					fmt.Printf("  Database: %s@%s:%d/%s\n", currentProfile.User, currentProfile.Host, currentProfile.Port, currentProfile.Database)
					fmt.Printf("  SSL Mode: %s\n", currentProfile.SSLMode)
					if currentProfile.SSH.Enabled {
						fmt.Printf("  SSH: %s@%s:%d\n", currentProfile.SSH.User, currentProfile.SSH.Host, currentProfile.SSH.Port)
						if currentProfile.SSH.KeyPath != "" {
							fmt.Printf("  SSH Key: %s\n", currentProfile.SSH.KeyPath)
						}
					}
					fmt.Println()
				}
			}
			
			fmt.Print("Do you want to overwrite it? [y/N]: ")
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				utils.PrintMuted(cmd, "Cancelled profile overwrite.")
				return nil
			}
		}

		var p config.Profile
		var existingProfile *config.Profile

		// Load existing profile if it exists for use as defaults in interactive mode
		if exists {
			cfg, err := config.LoadConfig()
			if err == nil {
				if existing, ok := cfg.Profiles[name]; ok {
					existingProfile = &existing
				}
			}
		}

		// Determine if we should use interactive mode
		// Interactive mode is default unless:
		// 1. --non-interactive flag is used, or
		// 2. --interactive=false is explicitly set, or
		// 3. Any connection flags are provided (indicating non-interactive intent)
		useInteractive := !flagNonInteractive && 
			(flagInteractive || (!cmd.Flags().Changed("interactive") && !hasConnectionFlags(cmd)))

		if useInteractive {
			p = promptProfileInput(cmd, name, existingProfile)
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

		var testFunc func(config.Profile) error
		if !flagSkipTest {
			utils.PrintInfo(cmd, "Validating connection for profile '%s'...", name)
			testFunc = db.TestConnection
		} else {
			utils.PrintInfo(cmd, "Skipping connection test for profile '%s'...", name)
			testFunc = nil
		}

		if err := config.AddOrUpdateProfile(p, testFunc); err != nil {
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
	addCmd.Flags().BoolVarP(&flagInteractive, "interactive", "i", false, "Force interactive mode (default unless flags provided)")
	addCmd.Flags().BoolVar(&flagNonInteractive, "non-interactive", false, "Force non-interactive mode")
	addCmd.Flags().BoolVar(&flagSkipTest, "skip-test", false, "Skip connection testing when adding profile")
}

// securePrompt prompts for sensitive information without echoing to terminal
func securePrompt(label string) string {
	fmt.Printf("%s: ", label)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // Print newline after password input
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(bytePassword))
}

// interactive prompt helper
func promptProfileInput(cmd *cobra.Command, name string, existingProfile *config.Profile) config.Profile {
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

	// Set defaults from existing profile if available
	defaultUser := "postgres"
	defaultHost := "localhost"
	defaultPort := "5432"
	defaultDatabase := ""
	defaultSSLMode := "disable"
	
	if existingProfile != nil {
		defaultUser = existingProfile.User
		defaultHost = existingProfile.Host
		defaultPort = strconv.Itoa(existingProfile.Port)
		defaultDatabase = existingProfile.Database
		defaultSSLMode = existingProfile.SSLMode
	}

	user := prompt("Database user", defaultUser)
	password := securePrompt("Database password")
	host := prompt("Database host", defaultHost)
	portStr := prompt("Database port", defaultPort)
	port, _ := strconv.Atoi(portStr)
	database := prompt("Database name", defaultDatabase)
	sslmode := prompt("SSL mode", defaultSSLMode)

	// Set SSH defaults from existing profile if available
	defaultUseSSH := "n"
	defaultSSHHost := ""
	defaultSSHUser := ""
	defaultSSHPort := "22"
	defaultSSHTimeout := "10"
	defaultAuthMethod := "k"
	defaultKeyPath := ""
	
	if existingProfile != nil && existingProfile.SSH.Enabled {
		defaultUseSSH = "y"
		defaultSSHHost = existingProfile.SSH.Host
		defaultSSHUser = existingProfile.SSH.User
		defaultSSHPort = strconv.Itoa(existingProfile.SSH.Port)
		defaultSSHTimeout = strconv.Itoa(existingProfile.SSH.Timeout)
		if existingProfile.SSH.KeyPath != "" {
			defaultAuthMethod = "k"
			defaultKeyPath = existingProfile.SSH.KeyPath
		} else {
			defaultAuthMethod = "p"
		}
	}

	useSSH := strings.ToLower(prompt("Use SSH tunnel? (y/N)", defaultUseSSH)) == "y"
	var sshCfg config.SSHConfig
	if useSSH {
		sshCfg.Enabled = true
		sshCfg.Host = prompt("SSH host", defaultSSHHost)
		sshCfg.User = prompt("SSH user", defaultSSHUser)
		portStr := prompt("SSH port", defaultSSHPort)
		sshCfg.Port, _ = strconv.Atoi(portStr)
		
		// Ask user to choose authentication method
		authMethod := strings.ToLower(prompt("SSH authentication method - use (k)ey or (p)assword? [k/p]", defaultAuthMethod))
		if authMethod == "p" || authMethod == "password" {
			sshCfg.Password = securePrompt("SSH password")
		} else {
			sshCfg.KeyPath = prompt("SSH key path", defaultKeyPath)
			// Ask for passphrase if key path is provided
			if sshCfg.KeyPath != "" {
				needsPassphrase := strings.ToLower(prompt("Does the SSH key require a passphrase? (y/N)", "n"))
				if needsPassphrase == "y" || needsPassphrase == "yes" {
					sshCfg.Passphrase = securePrompt("SSH key passphrase")
				}
			}
		}
		
		timeoutStr := prompt("SSH timeout (seconds)", defaultSSHTimeout)
		sshCfg.Timeout, _ = strconv.Atoi(timeoutStr)
	}

	return config.Profile{
		Name:     name,
		User:     user,
		Password: password,
		Host:     host,
		Port:     port,
		Database: database,
		DBURL:    "", // Interactive mode doesn't use DBURL
		SSLMode:  sslmode,
		SSH:      sshCfg,
	}
}

// hasConnectionFlags checks if any connection-related flags have been provided
func hasConnectionFlags(cmd *cobra.Command) bool {
	connectionFlags := []string{
		"user", "password", "host", "port", "database", "db", "sslmode",
		"ssh-host", "ssh-user", "ssh-key", "ssh-passphrase", "ssh-password", "ssh-port", "ssh-timeout",
	}
	
	for _, flag := range connectionFlags {
		if cmd.Flags().Changed(flag) {
			return true
		}
	}
	return false
}

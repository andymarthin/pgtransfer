package cmd

import (
	"fmt"

	"github.com/andymarthin/pgtransfer/internal/config"
	"github.com/andymarthin/pgtransfer/internal/db"
	"github.com/andymarthin/pgtransfer/internal/utils"
	"github.com/spf13/cobra"
)

var (
	profileName                                                        string
	testUser, testPassword, testHost, testDbURL, testDatabase          string
	testSSHHost, testSSHUser, testSSHKey, testSSHPassword, testSSHPassphrase string
	testSSLMode                                                        string
	testPort, testSSHPort, testSSHTimeout                              int
)

var testConnectionCmd = &cobra.Command{
	Use:   "test-connection",
	Short: "Test database connection",
	Long: `Test database connection using either a saved profile or direct connection parameters.

Examples:
  # Test using a saved profile
  pgtransfer test-connection --profile myprofile

  # Test using direct connection parameters
  pgtransfer test-connection --user postgres --host localhost --database mydb

  # Test with SSH tunnel using ssh-agent (automatic key detection)
  pgtransfer test-connection --user postgres --host localhost --database mydb --ssh-host example.com --ssh-user myuser

  # Test with SSH tunnel using specific key file
  pgtransfer test-connection --user postgres --host localhost --database mydb --ssh-host example.com --ssh-user myuser --ssh-key ~/.ssh/id_rsa

  # Test with SSH tunnel using passphrase-protected key
  pgtransfer test-connection --user postgres --host localhost --database mydb --ssh-host example.com --ssh-user myuser --ssh-key ~/.ssh/id_rsa --ssh-passphrase mypassphrase
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var profile config.Profile

		if profileName != "" {
			// Load profile from config
			cfg, err := config.LoadConfig()
			if err != nil {
				utils.PrintError(cmd, "Failed to load config: %v", err)
				return err
			}

			var exists bool
			profile, exists = cfg.Profiles[profileName]
			if !exists {
				utils.PrintError(cmd, "Profile '%s' not found", profileName)
				return fmt.Errorf("profile '%s' not found", profileName)
			}
			utils.PrintInfo(cmd, "Testing connection for profile '%s'...", profileName)
		} else {
			// Use direct connection parameters
			if testUser == "" || testHost == "" || testDatabase == "" {
				utils.PrintError(cmd, "When not using --profile, you must specify at least --user, --host, and --database")
				return fmt.Errorf("missing required connection parameters")
			}

			profile = config.Profile{
				Name:     "test",
				User:     testUser,
				Password: testPassword,
				Host:     testHost,
				Port:     testPort,
				Database: testDatabase,
				DBURL:    testDbURL,
				SSLMode:  testSSLMode,
				SSH: config.SSHConfig{
					Enabled:    testSSHHost != "",
					User:       testSSHUser,
					Host:       testSSHHost,
					Port:       testSSHPort,
					KeyPath:    testSSHKey,
					Passphrase: testSSHPassphrase,
					Password:   testSSHPassword,
					Timeout:    testSSHTimeout,
				},
			}
			utils.PrintInfo(cmd, "Testing direct connection to %s@%s:%d/%s...", testUser, testHost, testPort, testDatabase)
		}

		// Test the connection
		if err := db.TestConnection(profile); err != nil {
			utils.PrintError(cmd, "Connection test failed: %v", err)
			return err
		}

		utils.PrintSuccess(cmd, "Connection test successful!")
		return nil
	},
}

func init() {
	// Profile option
	testConnectionCmd.Flags().StringVar(&profileName, "profile", "", "Name of the saved profile to test")

	// Direct connection options
	testConnectionCmd.Flags().StringVar(&testDbURL, "db", "", "Database connection URL (overrides other options)")
	testConnectionCmd.Flags().StringVar(&testUser, "user", "", "Database user")
	testConnectionCmd.Flags().StringVar(&testPassword, "password", "", "Database password")
	testConnectionCmd.Flags().StringVar(&testHost, "host", "localhost", "Database host")
	testConnectionCmd.Flags().IntVar(&testPort, "port", 5432, "Database port")
	testConnectionCmd.Flags().StringVar(&testDatabase, "database", "", "Database name")
	testConnectionCmd.Flags().StringVar(&testSSLMode, "sslmode", "disable", "SSL mode (disable, require, verify-full)")

	// SSH options
	testConnectionCmd.Flags().StringVar(&testSSHHost, "ssh-host", "", "SSH host (optional)")
	testConnectionCmd.Flags().StringVar(&testSSHUser, "ssh-user", "", "SSH username")
	testConnectionCmd.Flags().StringVar(&testSSHKey, "ssh-key", "", "SSH private key path")
	testConnectionCmd.Flags().StringVar(&testSSHPassphrase, "ssh-passphrase", "", "SSH private key passphrase (optional)")
	testConnectionCmd.Flags().StringVar(&testSSHPassword, "ssh-password", "", "SSH password (optional)")
	testConnectionCmd.Flags().IntVar(&testSSHPort, "ssh-port", 22, "SSH port")
	testConnectionCmd.Flags().IntVar(&testSSHTimeout, "ssh-timeout", 10, "SSH timeout in seconds")
}

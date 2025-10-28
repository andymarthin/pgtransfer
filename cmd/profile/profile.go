package profile

import "github.com/spf13/cobra"

var ProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage PostgreSQL connection profiles",
	Long: `Manage saved PostgreSQL connection profiles.

Examples:
  # Add a local profile
  pgtransfer profile add local --user postgres --host localhost --database mydb
  
  # Add a profile with SSH tunnel using ssh-agent
  pgtransfer profile add remote --user postgres --host localhost --database mydb --ssh-host example.com --ssh-user myuser
  
  # Add a profile with SSH tunnel using specific key
  pgtransfer profile add remote --user postgres --host localhost --database mydb --ssh-host example.com --ssh-user myuser --ssh-key ~/.ssh/id_rsa
  
  # List all profiles
  pgtransfer profile list
  
  # Set active profile
  pgtransfer profile use local
  
  # Show active profile
  pgtransfer profile active
  
  # Delete a profile
  pgtransfer profile delete remote
`,
}

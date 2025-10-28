package migrate

import (
	"github.com/spf13/cobra"
)

// MigrateCmd represents the migrate command
var MigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate data between PostgreSQL databases",
	Long: `Migrate data between PostgreSQL databases using different profiles.

This command allows you to transfer data from one database to another, with options for:
- Full database migration (schema + data)
- Schema-only migration
- Data-only migration
- Selective table migration
- Migration with validation and rollback support

Examples:
  # Full database migration
  pgtransfer migrate source_profile target_profile

  # Schema-only migration
  pgtransfer migrate source_profile target_profile --schema-only

  # Data-only migration
  pgtransfer migrate source_profile target_profile --data-only

  # Migrate specific tables
  pgtransfer migrate source_profile target_profile --tables users,orders,products

  # Migration with pre-validation
  pgtransfer migrate source_profile target_profile --validate

  # Migration with rollback support
  pgtransfer migrate source_profile target_profile --enable-rollback`,
}

func init() {
	// Add subcommands
	MigrateCmd.AddCommand(databaseCmd)
	MigrateCmd.AddCommand(rollbackCmd)
}

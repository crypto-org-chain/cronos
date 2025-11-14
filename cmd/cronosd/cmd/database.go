package cmd

import (
	"github.com/spf13/cobra"
)

// DatabaseCmd returns the database command with subcommands
func DatabaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "database",
		Short: "Database management commands",
		Long: `Commands for managing Cronos databases.

Available subcommands:
  migrate - Migrate databases between different backend types
  patch   - Patch specific block heights into existing databases

Use "cronosd database [command] --help" for more information about a command.`,
		Aliases: []string{"db"},
	}

	// Add subcommands
	cmd.AddCommand(
		MigrateCmd(),
		PatchCmd(),
	)

	return cmd
}

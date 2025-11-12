package cmd

import (
	"github.com/spf13/cobra"
)

// DatabaseCmd constructs the top-level "database" Cobra command and registers its migrate and patch subcommands.
// The command is named "database" with alias "db" and provides help text describing available subcommands.
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
		MigrateCmd(), // migrate-db -> database migrate
		PatchCmd(),   // patchdb -> database patch
	)

	return cmd
}
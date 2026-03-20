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
  migrate       - Migrate databases between different backend types
  patch         - Patch specific block heights into existing databases
  fix-unlucky-tx - Patch missing ethereum_tx events for false-failed txs
  reindex-duplicated-tx - Fix tx indexer entries that disagree with committed block results

Use "cronosd database [command] --help" for more information about a command.`,
		Aliases: []string{"db"},
	}

	// Add subcommands
	cmd.AddCommand(
		MigrateCmd(),
		PatchCmd(),
		FixUnluckyTxCmd(),
		ReindexDuplicatedTxCmd(),
	)

	return cmd
}

package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/spf13/cobra"
)

// FlagDatabaseDebug enables verbose stderr logging for database maintenance subcommands.
const FlagDatabaseDebug = "debug"

func databaseDebugf(debug bool, format string, args ...any) {
	if !debug {
		return
	}
	fmt.Fprintf(os.Stderr, "[database] debug: "+format+"\n", args...)
}

func formatExecTxResultForDebug(r *abci.ExecTxResult) string {
	if r == nil {
		return "<nil>"
	}
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Sprintf("%+v", r)
	}
	return string(b)
}

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

package executionbook

import (
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
)

// Command creates the executionbook command with subcommands
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "executionbook",
		Short: "ExecutionBook management commands",
		Long: `Commands to manage the execution book and sequencer transactions.

The execution book stores transactions that have been pre-executed by sequencers
and ensures they are included in blocks in the correct order.`,
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		SequencerCommand(),
		StatsCommand(),
	)

	return cmd
}

// SequencerCommand creates the sequencer management command with subcommands
func SequencerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sequencer",
		Short: "Manage sequencers",
		Long: `Manage sequencers that can submit pre-executed transactions.

Sequencers are identified by their public keys and can submit transactions
with cryptographic signatures for verification.`,
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		GetSequencerListCmd(),
		// Future commands: add, remove sequencers
		// These would typically require governance or admin permissions
	)

	return cmd
}

// GetSequencerListCmd returns the command to list all sequencers
func GetSequencerListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all registered sequencers",
		Long: `List all sequencers that are authorized to submit transactions
to the execution book.

Example:
  cronosd executionbook sequencer list`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			// TODO: Implement gRPC query when ready
			cmd.Println("Sequencer list command - gRPC query not yet implemented")
			return nil
		},
	}

	return cmd
}

// StatsCommand returns the command to get execution book statistics
func StatsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Get execution book statistics",
		Long: `Display statistics about the execution book including:
- Number of pending transactions
- Number of included transactions
- Current sequence number
- Number of registered sequencers

Example:
  cronosd executionbook stats`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			// TODO: Implement gRPC query when ready
			cmd.Println("Stats command - gRPC query not yet implemented")
			return nil
		},
	}

	return cmd
}

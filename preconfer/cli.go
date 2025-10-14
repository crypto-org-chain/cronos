package preconfer

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
)

// PreconferCommand creates the preconfer command with subcommands
func PreconferCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preconfer",
		Short: "Preconfer (priority transaction) management commands",
		Long: `Commands to manage the preconfer mempool whitelist.

These commands allow you to add, remove, list, and manage addresses
that are allowed to boost transaction priority using the PRIORITY: prefix.`,
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		WhitelistCommand(),
	)

	return cmd
}

// WhitelistCommand creates the whitelist management command with subcommands
func WhitelistCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whitelist",
		Short: "Manage the preconfer whitelist",
		Long: `Manage the whitelist of addresses allowed to boost transaction priority.

When the whitelist is empty, all addresses can use priority boosting.
When the whitelist is non-empty, only whitelisted addresses can boost priority.`,
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		GetWhitelistAddCmd(),
		GetWhitelistRemoveCmd(),
		GetWhitelistListCmd(),
		GetWhitelistClearCmd(),
		GetWhitelistSetCmd(),
	)

	return cmd
}

// GetWhitelistAddCmd returns the command to add an address to the whitelist
func GetWhitelistAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [eth-address]",
		Short: "Add an Ethereum address to the whitelist",
		Long: `Add an Ethereum address to the preconfer whitelist.

The address should be in Ethereum format (0x...).

Example:
  cronosd preconfer whitelist add 0x1234567890123456789012345678901234567890`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			address := args[0]
			if !strings.HasPrefix(strings.ToLower(address), "0x") || len(address) != 42 {
				return fmt.Errorf("invalid Ethereum address format (expected 0x... with 40 hex digits): %s", address)
			}

			queryClient := NewWhitelistQueryClient(clientCtx)

			req := &AddToWhitelistRequest{
				Address: address,
			}

			res, err := queryClient.AddToWhitelist(context.Background(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetWhitelistRemoveCmd returns the command to remove an address from the whitelist
func GetWhitelistRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove [eth-address]",
		Short: "Remove an Ethereum address from the whitelist",
		Long: `Remove an Ethereum address from the preconfer whitelist.

Example:
  cronosd preconfer whitelist remove 0x1234567890123456789012345678901234567890`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			address := args[0]
			if !strings.HasPrefix(strings.ToLower(address), "0x") || len(address) != 42 {
				return fmt.Errorf("invalid Ethereum address format (expected 0x... with 40 hex digits): %s", address)
			}

			queryClient := NewWhitelistQueryClient(clientCtx)

			req := &RemoveFromWhitelistRequest{
				Address: address,
			}

			res, err := queryClient.RemoveFromWhitelist(context.Background(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetWhitelistListCmd returns the command to list all whitelisted addresses
func GetWhitelistListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all whitelisted addresses",
		Long: `List all Ethereum addresses in the preconfer whitelist.

Example:
  cronosd preconfer whitelist list`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := NewWhitelistQueryClient(clientCtx)

			req := &GetWhitelistRequest{}

			res, err := queryClient.GetWhitelist(context.Background(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetWhitelistClearCmd returns the command to clear the entire whitelist
func GetWhitelistClearCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear the entire whitelist",
		Long: `Clear all addresses from the preconfer whitelist.

After clearing, all addresses will be allowed to use priority boosting.

Example:
  cronosd preconfer whitelist clear`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := NewWhitelistQueryClient(clientCtx)

			req := &ClearWhitelistRequest{}

			res, err := queryClient.ClearWhitelist(context.Background(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetWhitelistSetCmd returns the command to set/replace the entire whitelist
func GetWhitelistSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set [eth-address1] [eth-address2] ...",
		Short: "Set/replace the entire whitelist",
		Long: `Set or replace the entire preconfer whitelist with the provided addresses.

This will remove all existing addresses and add only the specified ones.

Example:
  cronosd preconfer whitelist set 0x1234567890123456789012345678901234567890 0xABCDEF1234567890123456789012345678901234`,
		Args: cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			// Validate all addresses
			for _, addr := range args {
				if !strings.HasPrefix(strings.ToLower(addr), "0x") || len(addr) != 42 {
					return fmt.Errorf("invalid Ethereum address format: %s", addr)
				}
			}

			queryClient := NewWhitelistQueryClient(clientCtx)

			req := &SetWhitelistRequest{
				Addresses: args,
			}

			res, err := queryClient.SetWhitelist(context.Background(), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

package cli

import (
	"fmt"

	rpctypes "github.com/evmos/ethermint/rpc/types"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

// GetQueryCmd returns the cli query commands for this module
func GetQueryCmd(queryRoute string) *cobra.Command {
	// Group cronos queries under a subcommand
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("Querying commands for the %s module", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		GetContractByDenomCmd(),
		GetDenomByContractCmd(),
	)

	// this line is used by starport scaffolding # 1

	return cmd
}

// GetContractByDenomCmd queries the contracts by denom
func GetContractByDenomCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contract-by-denom [denom]",
		Short: "Gets contract addresses connected with the coin denom",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			req := &types.ContractByDenomRequest{
				Denom: args[0],
			}

			res, err := queryClient.ContractByDenom(rpctypes.ContextWithHeight(clientCtx.Height), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetDenomByContractCmd queries the denom name by contract address
func GetDenomByContractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "denom-by-contract [contract]",
		Short: "Gets the denom of the coin connected with the contract",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			req := &types.DenomByContractRequest{
				Contract: args[0],
			}

			res, err := queryClient.DenomByContract(rpctypes.ContextWithHeight(clientCtx.Height), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

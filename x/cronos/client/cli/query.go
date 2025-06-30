package cli

import (
	"fmt"
	"strings"

	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	rpctypes "github.com/evmos/ethermint/rpc/types"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
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
		QueryParamsCmd(),
		GetPermissions(),
	)

	// this line is used by starport scaffolding # 1

	return cmd
}

// QueryParamsCmd returns the command handler for evidence parameter querying.
func QueryParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the current cronos parameters",
		Args:  cobra.NoArgs,
		Long: strings.TrimSpace(`Query the current cronos parameters:

$ <appd> query cronos params
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.Params(cmd.Context(), &types.QueryParamsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(&res.Params)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

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

// GetPermissions queries the permission for a specific address
func GetPermissions() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "permissions [addr]",
		Short: "Gets the permissions of a specific address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			req := &types.QueryPermissionsRequest{
				Address: args[0],
			}

			res, err := queryClient.Permissions(rpctypes.ContextWithHeight(clientCtx.Height), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/version"
	govcli "github.com/cosmos/cosmos-sdk/x/gov/client/cli"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"

	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
)

// GetTxCmd returns the transaction commands for this module
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	// this line is used by starport scaffolding # 1

	cmd.AddCommand(CmdConvertTokens())
	cmd.AddCommand(CmdSendToCryptoOrg())
	cmd.AddCommand(CmdUpdateTokenMapping())
	cmd.AddCommand(CmdTurnBridge())
	cmd.AddCommand(CmdUpdatePermissions())
	cmd.AddCommand(EventQueryTxFor())

	return cmd
}

func CmdConvertTokens() *cobra.Command {
	cmd := &cobra.Command{
		Use: "convert-vouchers [address] [amount]",
		Short: "Convert ibc vouchers to cronos tokens, Note, the'--from' flag is" +
			" ignored as it is implied from [address].`",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := cmd.Flags().Set(flags.FlagFrom, args[0])
			if err != nil {
				return err
			}
			coins, err := sdk.ParseCoinsNormalized(args[1])
			if err != nil {
				return err
			}

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.NewMsgConvertVouchers(clientCtx.GetFromAddress().String(), coins)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func CmdSendToCryptoOrg() *cobra.Command {
	cmd := &cobra.Command{
		Use: "transfer-tokens [from] [to] [amount]",
		Short: "Transfer cronos tokens to the origin chain through IBC , Note, the'--from' flag is" +
			" ignored as it is implied from [from].`",
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := cmd.Flags().Set(flags.FlagFrom, args[0])
			if err != nil {
				return err
			}
			argsTo := args[1]
			coins, err := sdk.ParseCoinsNormalized(args[2])
			if err != nil {
				return err
			}

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg := types.NewMsgTransferTokens(clientCtx.GetFromAddress().String(), argsTo, coins)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// TokenMappingChangeProposalTxCmd flags
const (
	FlagSymbol   = "symbol"
	FlagDecimals = "decimals"
)

// NewSubmitTokenMappingChangeProposalTxCmd returns a CLI command handler for creating
// a token mapping change proposal governance transaction.
// Deprecated: please use submit-proposal instead.
func NewSubmitTokenMappingChangeProposalTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token-mapping-change [denom] [contract]",
		Args:  cobra.ExactArgs(2),
		Short: "Submit a token mapping change proposal",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Submit a token mapping change proposal.

Example:
$ %s tx gov submit-legacy-proposal token-mapping-change gravity0x0000...0000 0x0000...0000 --from=<key_or_address>
`,
				version.AppName,
			),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			title, err := cmd.Flags().GetString(govcli.FlagTitle) //nolint:staticcheck
			if err != nil {
				return err
			}

			description, err := cmd.Flags().GetString(govcli.FlagDescription) //nolint:staticcheck
			if err != nil {
				return err
			}

			var contract *common.Address
			if len(args[1]) > 0 {
				addr := common.HexToAddress(args[1])
				contract = &addr
			}

			denom := args[0]
			if !types.IsValidCoinDenom(denom) {
				return fmt.Errorf("invalid coin denom: %s", denom)
			}

			symbol := ""
			decimal := uint(0)
			if types.IsSourceCoin(denom) {
				symbol, err = cmd.Flags().GetString(FlagSymbol)
				if err != nil {
					return err
				}

				decimal, err = cmd.Flags().GetUint(FlagDecimals)
				if err != nil {
					return err
				}
			}

			content := types.NewTokenMappingChangeProposal(
				title, description, args[0], symbol, uint32(decimal), contract,
			)

			from := clientCtx.GetFromAddress()

			strDeposit, err := cmd.Flags().GetString(govcli.FlagDeposit)
			if err != nil {
				return err
			}

			deposit, err := sdk.ParseCoinsNormalized(strDeposit)
			if err != nil {
				return err
			}

			msg, err := govtypes.NewMsgSubmitProposal(content, deposit, from)
			if err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	cmd.Flags().String(govcli.FlagTitle, "", "The proposal title")             //nolint:staticcheck
	cmd.Flags().String(govcli.FlagDescription, "", "The proposal description") //nolint:staticcheck
	cmd.Flags().String(govcli.FlagDeposit, "", "The proposal deposit")
	cmd.Flags().String(FlagSymbol, "", "The coin symbol")
	cmd.Flags().Uint(FlagDecimals, 0, "The coin decimal")

	return cmd
}

// CmdUpdateTokenMapping returns a CLI command handler for update token mapping
func CmdUpdateTokenMapping() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-token-mapping [denom] [contract]",
		Short: "Update token mapping",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			denom := args[0]
			if !types.IsValidCoinDenom(denom) {
				return fmt.Errorf("invalid coin denom: %s", denom)
			}

			symbol := ""
			decimal := uint(0)
			if types.IsSourceCoin(denom) {
				symbol, err = cmd.Flags().GetString(FlagSymbol)
				if err != nil {
					return err
				}

				decimal, err = cmd.Flags().GetUint(FlagDecimals)
				if err != nil {
					return err
				}
			}

			msg := types.NewMsgUpdateTokenMapping(clientCtx.GetFromAddress().String(), denom, args[1], symbol, uint32(decimal))
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().String(FlagSymbol, "", "The coin symbol")
	cmd.Flags().Uint(FlagDecimals, 0, "The coin decimal")

	return cmd
}

// CmdTurnBridge returns a CLI command handler for enable or disable the bridge
func CmdTurnBridge() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "turn-bridge [true/false]",
		Short: "Turn Bridge",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			enable, err := strconv.ParseBool(args[0])
			if err != nil {
				return err
			}
			msg := types.NewMsgTurnBridge(clientCtx.GetFromAddress().String(), enable)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// CmdUpdatePermissions returns a CLI command handler for updating cronos permissions
func CmdUpdatePermissions() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-permissions [address] [permissions]",
		Short: "Update Permissions, permission value: 1=CanChangeTokenMapping, 2:=CanTurnBridge, 3=All",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			argsAddress := args[0]
			argPermissions, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				return err
			}
			msg := types.NewMsgUpdatePermissions(clientCtx.GetFromAddress().String(), argsAddress, argPermissions)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// EventQueryTxFor returns a CLI command that subscribes to a WebSocket connection and waits for a transaction event with the given hash.
func EventQueryTxFor() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event-query-tx-for [hash]",
		Short: "event-query-tx-for [hash]",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			c, err := rpchttp.New(clientCtx.NodeURI, "/websocket")
			if err != nil {
				return err
			}
			if err := c.Start(); err != nil {
				return err
			}
			defer c.Stop()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
			defer cancel()

			hash := args[0]
			query := fmt.Sprintf("%s='%s' AND %s='%s'", tmtypes.EventTypeKey, tmtypes.EventTx, tmtypes.TxHashKey, hash)
			const subscriber = "subscriber"
			eventCh, err := c.Subscribe(ctx, subscriber, query)
			if err != nil {
				return fmt.Errorf("failed to subscribe to tx: %w", err)
			}
			defer c.UnsubscribeAll(context.Background(), subscriber)

			select {
			case evt := <-eventCh:
				if txe, ok := evt.Data.(tmtypes.EventDataTx); ok {
					res := &coretypes.ResultBroadcastTxCommit{
						DeliverTx: txe.Result,
						Hash:      tmtypes.Tx(txe.Tx).Hash(),
						Height:    txe.Height,
					}
					return clientCtx.PrintProto(sdk.NewResponseFormatBroadcastTxCommit(res))
				}
			case <-ctx.Done():
				return errors.New("timed out waiting for event")
			}
			return nil
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

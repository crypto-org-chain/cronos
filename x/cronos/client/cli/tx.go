package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/version"
	govcli "github.com/cosmos/cosmos-sdk/x/gov/client/cli"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	// "github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
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
	FlagSigner   = "signer"
)

// proposal defines the new Msg-based proposal.
type proposal struct {
	// Msgs defines an array of sdk.Msgs proto-JSON-encoded as Anys.
	Messages []json.RawMessage `json:"messages,omitempty"`
	Metadata string            `json:"metadata"`
	Deposit  string            `json:"deposit"`
}

// NewSubmitTokenMappingChangeProposalTxCmd returns a CLI command handler for creating
// a token mapping change proposal governance transaction.
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

			title, err := cmd.Flags().GetString(govcli.FlagTitle)
			if err != nil {
				return err
			}

			description, err := cmd.Flags().GetString(govcli.FlagDescription)
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

			signer, err := cmd.Flags().GetString(FlagSigner)
			if err != nil {
				return err
			}

			content := types.NewTokenMappingChangeProposal(
				title, description, args[0], symbol, signer, uint32(decimal), contract,
			)
			m, err := govtypes.NewLegacyContent(content, signer)
			if err != nil {
				return err
			}
			strDeposit, err := cmd.Flags().GetString(govcli.FlagDeposit)
			if err != nil {
				return err
			}
			deposit, err := sdk.ParseCoinsNormalized(strDeposit)
			if err != nil {
				return err
			}
			msg, err := govtypes.NewMsgSubmitProposal(
				[]sdk.Msg{m},
				deposit,
				clientCtx.GetFromAddress().String(),
				"",
			)
			if err != nil {
				return err
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}
	cmd.Flags().String(govcli.FlagTitle, "", "The proposal title")
	cmd.Flags().String(govcli.FlagDescription, "", "The proposal description")
	cmd.Flags().String(govcli.FlagDeposit, "", "The proposal deposit")
	cmd.Flags().String(FlagSymbol, "", "The coin symbol")
	cmd.Flags().Uint(FlagDecimals, 0, "The coin decimal")
	cmd.Flags().String(FlagSigner, "", "The governance module account")
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

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
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v5/modules/apps/29-fee/types"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	feemarkettypes "github.com/evmos/ethermint/x/feemarket/types"
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
	cmd.AddCommand(MigrateGenesisCmd())

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

type ExportEvmGenesisState struct {
	evmtypes.GenesisState
	Params ExportEvmParams `json:"params"`
}

type ExportEvmParams struct {
	evmtypes.Params
	ExtraEIPs []string `json:"extra_eips"`
}

type ExportFeemarketGenesisState struct {
	feemarkettypes.GenesisState
	Params   ExportFeemarketParams `json:"params"`
	BlockGas uint64                `json:"block_gas,string"`
}

type ExportFeemarketParams struct {
	feemarkettypes.Params
	EnableHeight int64 `json:"enable_height,string"`
}

func Migrate(appState genutiltypes.AppMap, clientCtx client.Context) genutiltypes.AppMap {
	// Add feeibc with default genesis.
	if appState[ibcfeetypes.ModuleName] == nil {
		appState[ibcfeetypes.ModuleName] = clientCtx.Codec.MustMarshalJSON(ibcfeetypes.DefaultGenesisState())
	}
	var evmState ExportEvmGenesisState
	err := json.Unmarshal(appState[evmtypes.ModuleName], &evmState)
	if err != nil {
		panic(err)
	}
	data, err := json.Marshal(evmState)
	if err != nil {
		panic(err)
	}
	appState[evmtypes.ModuleName] = data

	var feemarketState ExportFeemarketGenesisState
	err = json.Unmarshal(appState[feemarkettypes.ModuleName], &feemarketState)
	if err != nil {
		panic(err)
	}
	feemarketState.GenesisState.BlockGas = feemarketState.BlockGas
	data, err = json.Marshal(feemarketState)
	if err != nil {
		panic(err)
	}
	appState[feemarkettypes.ModuleName] = data
	return appState
}

const flagGenesisTime = "genesis-time"

// migrationMap is a map of SDK versions to their respective genesis migration functions.
var migrationMap = genutiltypes.MigrationMap{
	"v1.0": Migrate,
}

// MigrateGenesisCmd returns a command to execute genesis state migration.
func MigrateGenesisCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate [target-version] [genesis-file]",
		Short: "Migrate genesis to a specified target version",
		Long: fmt.Sprintf(`Migrate the source genesis into the target version and print to STDOUT.

Example:
$ %s migrate v1.0 /path/to/genesis.json --chain-id=cronos_777-1 --genesis-time=2019-04-22T17:00:00Z
`, version.AppName),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return genutilcli.MigrateHandler(cmd, args, migrationMap)
		},
	}

	cmd.Flags().String(flagGenesisTime, "", "override genesis_time with this flag")
	cmd.Flags().String(flags.FlagChainID, "", "override chain_id with this flag")

	return cmd
}

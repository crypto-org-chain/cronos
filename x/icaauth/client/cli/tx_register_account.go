package cli

import (
	"strconv"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/gogoproto/proto"
	cronosprecompiles "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper/precompiles"
	"github.com/crypto-org-chain/cronos/v2/x/icaauth/types"
	"github.com/spf13/cobra"
)

var _ = strconv.Itoa(0)

func CmdRegisterAccount() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register-account [connection-id]",
		Short: "Registers an interchain account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			argConnectionID := args[0]

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			version, err := cmd.Flags().GetString(flagVersion)
			if err != nil {
				return err
			}

			printProtoOnly, err := cmd.Flags().GetBool(flagPrintProtoOnly)
			if err != nil {
				return err
			}

			msg := types.NewMsgRegisterAccount(
				clientCtx.GetFromAddress().String(),
				argConnectionID,
				version,
			)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			if printProtoOnly {
				msgBytes, err := proto.Marshal(msg)
				if err != nil {
					return err
				}
				input := AddLengthPrefix(cronosprecompiles.PrefixRegisterAccount, msgBytes)
				cmd.Println(string(input))
				return nil
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String(flagVersion, "", "version of the ICA channel")
	cmd.Flags().Bool(flagPrintProtoOnly, false, "print proto only or not")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

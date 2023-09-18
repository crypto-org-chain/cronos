package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/version"
	authclient "github.com/cosmos/cosmos-sdk/x/auth/client"
	"github.com/cosmos/gogoproto/proto"
	icacontrollertypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/controller/types"
	icatypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/types"
	cronosprecompiles "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper/precompiles"
	"github.com/crypto-org-chain/cronos/v2/x/icaauth/types"
	"github.com/spf13/cobra"
)

var _ = strconv.Itoa(0)

func CmdSubmitTx() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-tx [connection-id] [msg_tx_json_file]",
		Short: "Submit a transaction on host chain on behalf of the interchain account",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Submit a transaction on host chain on behalf of the interchain account:
Example:
  $ %s tx %s submit-tx connection-1 tx.json --from mykey
  $ %s tx bank send <myaddress> <recipient> <amount> --generate-only > tx.json && %s tx %s submit-tx connection-1 tx.json --from mykey
			`, version.AppName, types.ModuleName, version.AppName, version.AppName, types.ModuleName),
		),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			argConnectionID := args[0]
			argMsg := args[1]

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			theTx, err := authclient.ReadTxFromFile(clientCtx, argMsg)
			if err != nil {
				return err
			}

			timeoutDuration, err := cmd.Flags().GetDuration(flagTimeoutDuration)
			if err != nil {
				return err
			}

			printProtoOnly, err := cmd.Flags().GetBool(flagPrintProtoOnly)
			if err != nil {
				return err
			}

			msg := types.NewMsgSubmitTx(
				clientCtx.GetFromAddress().String(),
				argConnectionID,
				theTx.GetMsgs(),
				&timeoutDuration,
			)
			if err := msg.ValidateBasic(); err != nil {
				return err
			}
			if printProtoOnly {
				msgBytes, err := proto.Marshal(msg)
				if err != nil {
					return err
				}
				input := AddLengthPrefix(cronosprecompiles.PrefixSubmitMsgs, msgBytes)
				cmd.Println(string(input))
				return nil
			}
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().Duration(flagTimeoutDuration, time.Minute*5, "Timeout duration for the transaction (default: 5 minutes)")
	cmd.Flags().Bool(flagPrintProtoOnly, false, "print proto only or not")

	return cmd
}

func CmdPrintSubmitTxProto() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "print-submit-tx-proto [connection-id]",
		Short: "Print the proto of submit a transaction on host chain on behalf of the interchain account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			argConnectionID := args[0]
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			relativeTimeoutTimestamp, err := cmd.Flags().GetUint64(flagRelativePacketTimeout)
			if err != nil {
				return err
			}

			owner := clientCtx.GetFromAddress().String()
			packetDataStr, err := cmd.Flags().GetString(flagPacketDataStr)
			if err != nil {
				return err
			}
			cdc := codec.NewProtoCodec(clientCtx.InterfaceRegistry)
			var packetData icatypes.InterchainAccountPacketData
			if err = cdc.UnmarshalJSON([]byte(packetDataStr), &packetData); err != nil {
				return err
			}
			msg := &icacontrollertypes.MsgSendTx{
				Owner:           owner,
				ConnectionId:    argConnectionID,
				PacketData:      packetData,
				RelativeTimeout: relativeTimeoutTimestamp,
			}
			msgBytes, err := proto.Marshal(msg)
			if err != nil {
				return err
			}
			input := AddLengthPrefix(cronosprecompiles.PrefixSubmitMsgs, msgBytes)
			cmd.Println(string(input))
			return nil
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	cmd.Flags().Uint64(flagRelativePacketTimeout, icatypes.DefaultRelativePacketTimeoutTimestamp, "Relative packet timeout in nanoseconds from now. Default is 10 minutes.")
	cmd.Flags().String(flagPacketDataStr, "", "packet data string")

	return cmd
}

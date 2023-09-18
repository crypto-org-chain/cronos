package cli

import (
	"encoding/binary"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	cronosprecompiles "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper/precompiles"
	"github.com/crypto-org-chain/cronos/v2/x/icaauth/types"
	"github.com/spf13/cobra"
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

	cmd.AddCommand(CmdRegisterAccount())
	cmd.AddCommand(CmdSubmitTx())
	cmd.AddCommand(CmdPrintSubmitTxProto())

	return cmd
}

func AddLengthPrefix(prefix int, input []byte) []byte {
	prefixBytes := make([]byte, cronosprecompiles.PrefixSize4Bytes)
	binary.LittleEndian.PutUint32(prefixBytes, uint32(prefix))
	return append(prefixBytes, input...)
}

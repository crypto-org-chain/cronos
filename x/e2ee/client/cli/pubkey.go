package cli

import (
	"fmt"
	"os"

	"filippo.io/age"
	"github.com/crypto-org-chain/cronos/x/e2ee/keyring"
	"github.com/crypto-org-chain/cronos/x/e2ee/types"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
)

func PubKeyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pubkey",
		Short: "Show the recipient of current identity stored in keyring",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			krName, err := cmd.Flags().GetString(FlagKeyringName)
			if err != nil {
				return err
			}

			kr, err := keyring.New("cronosd", clientCtx.Keyring.Backend(), clientCtx.HomeDir, os.Stdin)
			if err != nil {
				return err
			}

			bz, err := kr.Get(krName)
			if err != nil {
				return err
			}

			k, err := age.ParseX25519Identity(string(bz))
			if err != nil {
				return err
			}

			fmt.Println(k.Recipient())
			return nil
		},
	}

	cmd.Flags().String(FlagKeyringName, types.DefaultKeyringName, "The keyring name to use")

	return cmd
}

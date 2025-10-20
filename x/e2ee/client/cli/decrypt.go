package cli

import (
	"fmt"
	"io"
	"os"

	"filippo.io/age"
	"github.com/crypto-org-chain/cronos/x/e2ee/keyring"
	"github.com/crypto-org-chain/cronos/x/e2ee/types"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
)

const FlagIdentity = "identity"

func DecryptCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decrypt [input-file]",
		Short: "Decrypt input file to local identity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			kr, err := keyring.New("cronosd", clientCtx.Keyring.Backend(), clientCtx.HomeDir, os.Stdin)
			if err != nil {
				return err
			}

			outputFile, err := cmd.Flags().GetString(flags.FlagOutput)
			if err != nil {
				return err
			}

			identityNames, err := cmd.Flags().GetStringArray(FlagIdentity)
			if err != nil {
				return err
			}

			if len(identityNames) == 0 {
				return fmt.Errorf("no identity provided")
			}

			identities := make([]age.Identity, len(identityNames))
			for i, name := range identityNames {
				secret, err := kr.Get(name)
				if err != nil {
					return err
				}

				identity, err := age.ParseX25519Identity(string(secret))
				if err != nil {
					return err
				}

				identities[i] = identity
			}

			var input io.Reader
			inputFile := args[0]
			if inputFile == "-" {
				input = os.Stdin
			} else {
				f, err := os.Open(inputFile)
				if err != nil {
					return err
				}
				defer f.Close()
				input = f
			}

			var output io.Writer
			if outputFile == "-" {
				output = os.Stdout
			} else {
				f, err := os.Create(outputFile)
				if err != nil {
					return err
				}
				defer f.Close()
				output = f
			}
			return decrypt(identities, input, output)
		},
	}

	cmd.Flags().StringArrayP(FlagIdentity, "i", []string{types.DefaultKeyringName}, "identity (can be repeated)")
	cmd.Flags().StringP(flags.FlagOutput, "o", "-", "output file (default stdout)")

	return cmd
}

func decrypt(identities []age.Identity, in io.Reader, out io.Writer) error {
	r, err := age.Decrypt(in, identities...)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		return err
	}
	return nil
}

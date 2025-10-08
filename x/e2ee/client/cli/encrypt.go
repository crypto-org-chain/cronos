package cli

import (
	"errors"
	"io"
	"os"

	"filippo.io/age"
	"github.com/crypto-org-chain/cronos/x/e2ee/types"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
)

const (
	FlagRecipient = "recipient"
)

func EncryptCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "encrypt [input-file]",
		Short: "Encrypt input file to one or multiple recipients",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			outputFile, err := cmd.Flags().GetString(flags.FlagOutput)
			if err != nil {
				return err
			}

			recs, err := cmd.Flags().GetStringArray(FlagRecipient)
			if err != nil {
				return err
			}

			// query encryption key from chain state
			client := types.NewQueryClient(clientCtx)
			rsp, err := client.Keys(clientCtx.CmdContext, &types.KeysRequest{
				Addresses: recs,
			})
			if err != nil {
				return err
			}

			recipients := make([]age.Recipient, len(recs))
			for i, key := range rsp.Keys {
				recipient, err := age.ParseX25519Recipient(key)
				if err != nil {
					return err
				}
				recipients[i] = recipient
			}

			inputFile := args[0]
			var input io.Reader
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
				fp, err := os.Create(outputFile)
				if err != nil {
					return err
				}
				defer fp.Close()
				output = fp
			}
			return encrypt(recipients, input, output)
		},
	}
	f := cmd.Flags()
	f.StringArrayP(FlagRecipient, "r", []string{}, "recipients")
	f.StringP(flags.FlagOutput, "o", "-", "output file (default stdout)")
	return cmd
}

func encrypt(recipients []age.Recipient, in io.Reader, out io.Writer) (err error) {
	var w io.WriteCloser
	w, err = age.Encrypt(out, recipients...)
	if err != nil {
		return err
	}

	defer func() {
		err = errors.Join(err, w.Close())
	}()

	_, err = io.Copy(w, in)
	return
}

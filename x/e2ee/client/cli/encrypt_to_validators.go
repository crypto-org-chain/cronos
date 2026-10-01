package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"filippo.io/age"
	"github.com/crypto-org-chain/cronos/x/e2ee/types"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func EncryptToValidatorsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "encrypt-to-validators [input-file]",
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

			ctx := context.Background()

			// get validator list
			stakingClient := stakingtypes.NewQueryClient(clientCtx)
			valsRsp, err := stakingClient.Validators(ctx, &stakingtypes.QueryValidatorsRequest{
				Status: stakingtypes.BondStatusBonded,
			})
			if err != nil {
				return err
			}

			recs := make([]string, len(valsRsp.Validators))
			for i, val := range valsRsp.Validators {
				bz, err := sdk.ValAddressFromBech32(val.OperatorAddress)
				if err != nil {
					return err
				}
				// convert to account address
				recs[i] = sdk.AccAddress(bz).String()
			}

			// query encryption key from chain state
			client := types.NewQueryClient(clientCtx)
			rsp, err := client.Keys(context.Background(), &types.KeysRequest{
				Addresses: recs,
			})
			if err != nil {
				return err
			}

			recipients := make([]age.Recipient, len(recs))
			for i, key := range rsp.Keys {
				if len(key) == 0 {
					fmt.Fprintf(os.Stderr, "missing encryption key for validator %s\n", recs[i])
					continue
				}

				recipient, err := age.ParseX25519Recipient(key)
				if err != nil {
					fmt.Fprintf(os.Stderr, "invalid encryption key for validator %s, %v\n", recs[i], err)
					continue
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
	f.StringP(flags.FlagOutput, "o", "-", "output file (default stdout)")
	return cmd
}

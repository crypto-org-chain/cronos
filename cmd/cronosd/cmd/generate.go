package cmd

import (
	"fmt"
	"os"

	"filippo.io/age"
	"github.com/spf13/cobra"
)

func KeygenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keygen",
		Short: "Generates a new native X25519 key pair",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			output, err := f.GetString(flagO)
			if err != nil {
				return err
			}
			out := os.Stdout
			if output != "" {
				f, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.ModePerm)
				if err != nil {
					return err
				}
				defer func() {
					if err := f.Close(); err != nil {
						fmt.Println("file close error", err)
					}
				}()
				out = f
			}
			return generate(out)
		},
	}
	cmd.Flags().String(flagO, "", "output to `FILE`")
	return cmd
}

func generate(out *os.File) error {
	k, err := age.GenerateX25519Identity()
	if err != nil {
		return err
	}
	pubkey := k.Recipient()
	fmt.Fprintf(out, "# public key: %s\n", pubkey)
	fmt.Fprintf(out, "%s\n", k)
	fmt.Println(pubkey)
	return nil
}

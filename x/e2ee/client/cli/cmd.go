package cli

import "github.com/spf13/cobra"

func E2EECommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "e2ee",
		Short: "End-to-end encryption commands",
	}

	cmd.AddCommand(
		KeygenCommand(),
		EncryptCommand(),
		DecryptCommand(),
		EncryptToValidatorsCommand(),
		PubKeyCommand(),
	)

	return cmd
}

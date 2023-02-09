package client

import (
	"fmt"

	"github.com/spf13/cobra"
)

func ListStoresCmd(stores []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stores",
		Short: "List the store names in current binary version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, name := range stores {
				fmt.Println(name)
			}

			return nil
		},
	}
	return cmd
}

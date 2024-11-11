package client

import (
	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
	"github.com/spf13/cobra"
)

func FixDataCmd(stores []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fixdata",
		Args:  cobra.ExactArgs(1),
		Short: "Fix wrong data in versiondb, see: https://github.com/crypto-org-chain/cronos/issues/1683",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			versionDB, err := tsrocksdb.NewStore(dir)
			if err != nil {
				return err
			}

			// see: https://github.com/crypto-org-chain/cronos/issues/1683
			if err := versionDB.FixData(stores); err != nil {
				return err
			}

			return nil
		},
	}
	return cmd
}

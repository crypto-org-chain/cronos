package client

import (
	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
	"github.com/linxGnu/grocksdb"
	"github.com/spf13/cobra"
)

const (
	FlagDryRun = "dry-run"
	FlagStore  = "store"
)

func FixDataCmd(defaultStores []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fixdata <dir>",
		Args:  cobra.ExactArgs(1),
		Short: "Fix wrong data in versiondb, see: https://github.com/crypto-org-chain/cronos/issues/1683",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			dryRun, err := cmd.Flags().GetBool(FlagDryRun)
			if err != nil {
				return err
			}
			stores, err := cmd.Flags().GetStringArray(FlagStore)
			if err != nil {
				return err
			}
			if len(stores) == 0 {
				stores = defaultStores
			}

			var (
				db       *grocksdb.DB
				cfHandle *grocksdb.ColumnFamilyHandle
			)

			if dryRun {
				db, cfHandle, err = tsrocksdb.OpenVersionDBForReadOnly(dir, false)
			} else {
				db, cfHandle, err = tsrocksdb.OpenVersionDB(dir)
			}
			if err != nil {
				return err
			}

			versionDB := tsrocksdb.NewStoreWithDB(db, cfHandle)
			if err := versionDB.FixData(stores, dryRun); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().Bool(FlagDryRun, false, "Dry run, do not write to the database, open the database in read-only mode.")
	cmd.Flags().StringArray(FlagStore, nil, "Store names to fix, if not specified, all stores will be fixed.")
	return cmd
}

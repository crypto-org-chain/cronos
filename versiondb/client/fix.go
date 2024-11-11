package client

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/crypto-org-chain/cronos/versiondb/tsrocksdb"
	"github.com/linxGnu/grocksdb"
	"github.com/spf13/cobra"
)

const (
	FlagDryRun = "dryrun"
	FlagStore  = "store"
)

// DefaultStores is the list of store names in cronos v1.3
var DefaultStores = []string{
	"acc", "bank", "staking", "mint", "dist", "slashing", "gov",
	"params", "upgrade", "evidence", "capability", "consensus",
	"feegrant", "crisis", "ibc", "transfer", "feeibc", "icacontroller",
	"icahost", "icaauth", "evm", "feemarket", "e2ee", "cronos",
}

func FixCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix",
		Args:  cobra.ExactArgs(1),
		Short: "Fix versiondb data for a specific issue, see: https://github.com/crypto-org-chain/cronos/issues/1683",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			dryRun, err := cmd.Flags().GetBool(FlagDryRun)
			if err != nil {
				return err
			}

			storeNames, err := cmd.Flags().GetStringArray(FlagStore)
			if err != nil {
				return err
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
				return fmt.Errorf("failed to open versiondb: %w", err)
			}

			version := int64(0)
			for _, storeName := range storeNames {
				store := tsrocksdb.NewStoreWithDB(db, cfHandle)
				iter, err := store.IteratorAtVersion(storeName, nil, nil, &version)
				if err != nil {
					return fmt.Errorf("failed to create iterator: %w", err)
				}
				for ; iter.Valid(); iter.Next() {
					if dryRun {
						// print the key value pairs
						key := iter.Key()
						value := iter.Value()
						ts := binary.LittleEndian.Uint64(key[len(key)-8:])
						key = key[:len(key)-8]
						fmt.Printf("key: %s, ts: %d, value: %s\n", hex.EncodeToString(key), ts, hex.EncodeToString(value))
					}
				}
				if err := iter.Close(); err != nil {
					return fmt.Errorf("failed to close iterator: %w", err)
				}
			}
			return nil
		},
	}
	cmd.Flags().Bool(FlagDryRun, false, "Dry run mode, do not modify the database")
	cmd.Flags().StringArray(FlagStore, DefaultStores, "List of store names to fix")
	return cmd
}

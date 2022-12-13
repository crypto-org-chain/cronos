package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/gorocksdb"
	dbm "github.com/tendermint/tm-db"
)

var _ gorocksdb.CompactionFilter = RangeCompactionFilter{}

type RangeCompactionFilter struct {
	startKey []byte
	endKey   []byte
}

func (rcf RangeCompactionFilter) Filter(level int, key, val []byte) (remove bool, newVal []byte) {
	if bytes.Compare(key, rcf.startKey) >= 0 && bytes.Compare(key, rcf.endKey) < 0 {
		// Delete the key.
		return true, nil
	}
	// Keep the key.
	return false, nil
}

func (rcf RangeCompactionFilter) Name() string {
	return "range-deletion"
}

func DeleteOrphanNodes(dbpath string, store string) error {
	prefix := []byte(fmt.Sprintf("s/k:%s/o", store))
	endKey := []byte(fmt.Sprintf("s/k:%s/p", store))

	filter := RangeCompactionFilter{
		startKey: prefix,
		endKey:   endKey,
	}

	// Open a RocksDB database.
	options := gorocksdb.NewDefaultOptions()
	options.SetCompactionFilter(filter)
	db, err := gorocksdb.OpenDb(options, dbpath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Perform a manual compaction to apply the filter.
	db.CompactRange(gorocksdb.Range{Start: filter.startKey, Limit: filter.endKey})
	return nil
}

// DeleteOrphanRecordsCommand defines a keys command to add a generated or recovered private key to keybase.
func DeleteOrphanRecordsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-orphan-records <store>",
		Short: "Delete orphan records from iavl tree, only support rocksdb backend.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := server.GetServerContextFromCmd(cmd)

			// Bind flags to the Context's Viper so the app construction can set
			// options accordingly.
			if err := ctx.Viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			backend := server.GetAppDBBackend(ctx.Viper)
			if backend != dbm.RocksDBBackend {
				return errors.New("only support rocksdb backend for now")
			}
			home := ctx.Viper.GetString(flags.FlagHome)
			dataDir := filepath.Join(home, "data", "application.db")

			return DeleteOrphanNodes(dataDir, args[1])
		},
	}
	return cmd
}

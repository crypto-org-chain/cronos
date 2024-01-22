package client

import (
	"sort"

	dbm "github.com/cometbft/cometbft-db"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/linxGnu/grocksdb"
	"github.com/spf13/cobra"
)

// Options defines the customizable settings of ChangeSetGroupCmd
type Options struct {
	DefaultStores     []string
	OpenReadOnlyDB    func(home string, backend dbm.BackendType) (dbm.DB, error)
	AppRocksDBOptions func(sstFileWriter bool) *grocksdb.Options
}

func ChangeSetGroupCmd(opts Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "changeset",
		Short: "dump and manage change sets files and ingest into versiondb",
	}
	cmd.AddCommand(
		ListDefaultStoresCmd(opts.DefaultStores),
		DumpChangeSetCmd(opts),
		PrintChangeSetCmd(),
		VerifyChangeSetCmd(opts.DefaultStores),
		BuildVersionDBSSTCmd(opts.DefaultStores),
		IngestVersionDBSSTCmd(),
		ChangeSetToVersionDBCmd(),
		RestoreAppDBCmd(opts),
		RestoreVersionDBCmd(),
		GetLatestVersionCmd(),
	)
	return cmd
}

func GetStoreNames(keys map[string]*storetypes.KVStoreKey) []string {
	storeNames := make([]string, 0, len(keys))
	for name := range keys {
		storeNames = append(storeNames, name)
	}
	sort.Strings(storeNames)
	return storeNames
}

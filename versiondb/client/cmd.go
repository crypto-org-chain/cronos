package client

import (
	"github.com/linxGnu/grocksdb"
	"github.com/spf13/cobra"
	dbm "github.com/tendermint/tm-db"
)

// Options defines the customizable settings of ChangeSetGroupCmd
type Options struct {
	DefaultStores     []string
	OpenAppDBReadOnly func(home string, backend dbm.BackendType) (dbm.DB, error)
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
	)
	return cmd
}

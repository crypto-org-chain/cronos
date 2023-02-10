//go:build rocksdb
// +build rocksdb

package cmd

import (
	"github.com/crypto-org-chain/cronos/v2/cmd/cronosd/opendb"
	versiondbclient "github.com/crypto-org-chain/cronos/versiondb/client"
	"github.com/spf13/cobra"
)

func ChangeSetCmd(storeNames []string) *cobra.Command {
	return versiondbclient.ChangeSetGroupCmd(versiondbclient.Options{
		DefaultStores:     storeNames,
		OpenAppDBReadOnly: opendb.OpenAppDBReadOnly,
		AppRocksDBOptions: opendb.NewRocksdbOptions,
	})
}

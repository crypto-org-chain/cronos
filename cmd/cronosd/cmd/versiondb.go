//go:build rocksdb
// +build rocksdb

package cmd

import (
	"github.com/crypto-org-chain/cronos/v2/app"
	versiondbclient "github.com/crypto-org-chain/cronos/versiondb/client"
	"github.com/spf13/cobra"
)

func ChangeSetCmd() *cobra.Command {
	keys, _, _ := app.StoreKeys(true)
	opts := app.GetOptions(versiondbclient.GetStoreNames(keys))
	return versiondbclient.ChangeSetGroupCmd(opts)
}

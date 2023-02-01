//go:build rocksdb
// +build rocksdb

package cmd

import (
	"github.com/cosmos/cosmos-sdk/server/types"
	versiondbclient "github.com/crypto-org-chain/cronos/versiondb/client"
	"github.com/spf13/cobra"
)

func ChangeSetCmd(appCreator types.AppCreator) *cobra.Command {
	return versiondbclient.ChangeSetGroupCmd(appCreator)
}

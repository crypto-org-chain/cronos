//go:build rocksdb
// +build rocksdb

package cmd

import (
	versiondbclient "github.com/crypto-org-chain/cronos/versiondb/client"
	"github.com/spf13/cobra"
)

func ChangeSetCmd() *cobra.Command {
	return versiondbclient.ChangeSetGroupCmd()
}

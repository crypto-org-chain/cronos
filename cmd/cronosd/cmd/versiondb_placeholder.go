//go:build !rocksdb
// +build !rocksdb

package cmd

import (
	"github.com/cosmos/cosmos-sdk/server/types"
	"github.com/spf13/cobra"
)

func ChangeSetCmd(types.AppCreator) *cobra.Command {
	return nil
}

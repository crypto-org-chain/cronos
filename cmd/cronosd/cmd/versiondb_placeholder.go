//go:build !rocksdb
// +build !rocksdb

package cmd

import (
	"github.com/spf13/cobra"
)

func ChangeSetCmd(storeNames []string) *cobra.Command {
	return nil
}

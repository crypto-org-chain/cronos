//go:build !rocksdb
// +build !rocksdb

package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func CompactDBCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "compact-db [dbpath]",
		Short: "Compact application.db with optimal parameters, not supported in this binary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("don't support rocksdb backend")
		},
	}
}

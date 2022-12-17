//go:build rocksdb
// +build rocksdb

package cmd

import (
	"github.com/cosmos/gorocksdb"
	"github.com/spf13/cobra"
)

func CompactDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compact-db [dbpath]",
		Short: "Compact application.db with optimal parameters",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := args[0]

			blockSize, err := cmd.Flags().GetInt("block-size")
			if err != nil {
				return err
			}
			fileSizeBase, err := cmd.Flags().GetUint64("file-size-base")
			if err != nil {
				return err
			}

			bbto := gorocksdb.NewDefaultBlockBasedTableOptions()
			bbto.SetBlockSize(blockSize)

			compressionOpts := gorocksdb.NewDefaultCompressionOptions()
			compressionOpts.Level = 12
			compressionOpts.MaxDictBytes = 110 * 1024

			zstdOpts := gorocksdb.NewZSTDCompressionOptions(1, compressionOpts.MaxDictBytes*100)

			opts := gorocksdb.NewDefaultOptions()
			opts.SetBlockBasedTableFactory(bbto)
			opts.SetTargetFileSizeBase(fileSizeBase)
			opts.SetCompressionOptions(compressionOpts)
			opts.SetZSTDCompressionOptions(zstdOpts)
			opts.SetCompression(gorocksdb.ZSTDCompression)
			db, err := gorocksdb.OpenDb(opts, dbPath)
			if err != nil {
				return err
			}
			db.CompactRange(gorocksdb.Range{Start: nil, Limit: nil})
			return nil
		},
	}
	cmd.Flags().Int("block-size", 32*1024, "block size")
	cmd.Flags().Uint64("file-size-base", 320*1024*1024, "sst target file size base")
	return cmd
}

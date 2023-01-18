//go:build rocksdb
// +build rocksdb

package cmd

import (
	"path/filepath"

	"github.com/linxGnu/grocksdb"
	dbm "github.com/tendermint/tm-db"
)

func openDB(rootDir string, backendType dbm.BackendType) (dbm.DB, error) {
	dataDir := filepath.Join(rootDir, "data")
	if backendType == dbm.RocksDBBackend {
		// customize rocksdb options
		db, err := grocksdb.OpenDb(newRocksdbOptions(), filepath.Join(dataDir, "application.db"))
		if err != nil {
			return nil, err
		}
		ro := grocksdb.NewDefaultReadOptions()
		wo := grocksdb.NewDefaultWriteOptions()
		woSync := grocksdb.NewDefaultWriteOptions()
		woSync.SetSync(true)
		return dbm.NewRocksDBWithRawDB(db, ro, wo, woSync), nil
	} else {
		return dbm.NewDB("application", backendType, dataDir)
	}
}

func newRocksdbOptions() *grocksdb.Options {
	opts := grocksdb.NewDefaultOptions()
	opts.SetTargetFileSizeMultiplier(2)
	opts.SetLevelCompactionDynamicLevelBytes(true)
	opts.SetCreateIfMissing(true)

	// block based table options
	blkOpts := grocksdb.NewDefaultBlockBasedTableOptions()
	blkOpts.SetBlockSize(32 * 1024)
	blkOpts.SetFilterPolicy(grocksdb.NewRibbonHybridFilterPolicy(9.9, 1))
	blkOpts.SetIndexType(grocksdb.KTwoLevelIndexSearchIndexType)
	blkOpts.SetPartitionFilters(true)
	blkOpts.SetDataBlockIndexType(grocksdb.KDataBlockIndexTypeBinarySearchAndHash)
	opts.SetBlockBasedTableFactory(blkOpts)
	opts.SetOptimizeFiltersForHits(true)

	// compression options at bottommost level
	opts.SetBottommostCompression(grocksdb.ZSTDCompression)
	compressOpts := grocksdb.NewDefaultCompressionOptions()
	compressOpts.MaxDictBytes = 112640 // 110k
	compressOpts.Level = 12
	opts.SetBottommostCompressionOptions(compressOpts, true)
	opts.SetBottommostCompressionOptionsZstdMaxTrainBytes(compressOpts.MaxDictBytes*100, true)
	return opts
}

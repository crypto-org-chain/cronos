//go:build rocksdb
// +build rocksdb

package cmd

import (
	"path/filepath"
	"runtime"

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
	opts.SetCreateIfMissing(true)
	opts.IncreaseParallelism(runtime.NumCPU())
	opts.OptimizeLevelStyleCompaction(512 * 1024 * 1024)
	opts.SetTargetFileSizeMultiplier(2)

	// block based table options
	bbto := grocksdb.NewDefaultBlockBasedTableOptions()
	bbto.SetBlockCache(grocksdb.NewLRUCache(1 << 30))
	bbto.SetBlockSize(32 * 1024)
	bbto.SetFilterPolicy(grocksdb.NewRibbonHybridFilterPolicy(9.9, 1))
	bbto.SetIndexType(grocksdb.KTwoLevelIndexSearchIndexType)
	bbto.SetPartitionFilters(true)
	bbto.SetDataBlockIndexType(grocksdb.KDataBlockIndexTypeBinarySearchAndHash)
	opts.SetBlockBasedTableFactory(bbto)
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

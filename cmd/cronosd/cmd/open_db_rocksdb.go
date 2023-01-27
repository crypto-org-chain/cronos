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

	// 1G block cache
	bbto.SetBlockCache(grocksdb.NewLRUCache(1 << 30))

	// http://rocksdb.org/blog/2021/12/29/ribbon-filter.html
	bbto.SetFilterPolicy(grocksdb.NewRibbonHybridFilterPolicy(9.9, 1))

	// partition index
	// http://rocksdb.org/blog/2017/05/12/partitioned-index-filter.html
	bbto.SetIndexType(grocksdb.KTwoLevelIndexSearchIndexType)
	bbto.SetPartitionFilters(true)

	// hash index is better for iavl tree which mostly do point lookup.
	bbto.SetDataBlockIndexType(grocksdb.KDataBlockIndexTypeBinarySearchAndHash)

	opts.SetBlockBasedTableFactory(bbto)

	// in iavl tree, we almost always query existing keys
	opts.SetOptimizeFiltersForHits(true)

	// heavier compression option at bottommost level,
	// 110k dict bytes is default in zstd library,
	// train bytes is recommended to be set at 100x dict bytes.
	opts.SetBottommostCompression(grocksdb.ZSTDCompression)
	compressOpts := grocksdb.NewDefaultCompressionOptions()
	compressOpts.MaxDictBytes = 110 * 1024
	compressOpts.Level = 12
	opts.SetBottommostCompressionOptions(compressOpts, true)
	opts.SetBottommostCompressionOptionsZstdMaxTrainBytes(compressOpts.MaxDictBytes*100, true)
	return opts
}

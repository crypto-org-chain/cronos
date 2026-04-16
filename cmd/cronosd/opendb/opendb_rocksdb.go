//go:build rocksdb

package opendb

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	dbm "github.com/cosmos/cosmos-db"
	cronosconfig "github.com/crypto-org-chain/cronos/cmd/cronosd/config"
	"github.com/linxGnu/grocksdb"
	"github.com/spf13/cast"

	"github.com/cosmos/cosmos-sdk/server/types"
)

// BlockCacheSize 3G block cache
const BlockCacheSize = 3 << 30

type RocksDBTuneUpOptions struct {
	EnableAsyncIo                bool
	EnableAutoReadaheadSize      bool
	EnableOptimizeForPointLookup bool
	EnableHyperClockCache        bool
	EnableDirectIOForCompaction  bool
}

func OpenDB(appOpts types.AppOptions, home string, backendType dbm.BackendType) (dbm.DB, error) {
	dataDir := filepath.Join(home, "data")

	if backendType == dbm.RocksDBBackend {
		tuneUpOpts := RocksDBTuneUpOptions{}
		if appOpts != nil {
			if v := appOpts.Get("rocksdb.node_type"); v != nil {
				cfg := cronosconfig.RocksDBConfig{NodeType: cast.ToString(v)}
				if err := cfg.Validate(); err != nil {
					return nil, fmt.Errorf("invalid rocksdb configuration: %w", err)
				}
				switch cfg.NodeType {
				case cronosconfig.NodeTypeValidator:
					tuneUpOpts.EnableOptimizeForPointLookup = true
				case cronosconfig.NodeTypeRPC:
					tuneUpOpts.EnableAutoReadaheadSize = true
					tuneUpOpts.EnableOptimizeForPointLookup = true
					tuneUpOpts.EnableHyperClockCache = true
					tuneUpOpts.EnableDirectIOForCompaction = true
				case cronosconfig.NodeTypeArchive:
					tuneUpOpts.EnableAsyncIo = true
					tuneUpOpts.EnableAutoReadaheadSize = true
					tuneUpOpts.EnableHyperClockCache = true
					tuneUpOpts.EnableOptimizeForPointLookup = true
					tuneUpOpts.EnableDirectIOForCompaction = true
				}
			}
		}
		return openRocksdb(filepath.Join(dataDir, "application.db"), false, tuneUpOpts)
	}

	return dbm.NewDB("application", backendType, dataDir)
}

// OpenReadOnlyDB opens rocksdb backend in read-only mode.
func OpenReadOnlyDB(home string, backendType dbm.BackendType) (dbm.DB, error) {
	dataDir := filepath.Join(home, "data")
	if backendType == dbm.RocksDBBackend {
		return openRocksdb(filepath.Join(dataDir, "application.db"), true, RocksDBTuneUpOptions{})
	}

	return dbm.NewDB("application", backendType, dataDir)
}

func openRocksdb(dir string, readonly bool, tuneUpOpts RocksDBTuneUpOptions) (dbm.DB, error) {
	var cache *grocksdb.Cache
	if tuneUpOpts.EnableHyperClockCache {
		cache = grocksdb.NewHyperClockCache(BlockCacheSize, 0)
	} else {
		cache = grocksdb.NewLRUCache(BlockCacheSize)
	}

	// explicitly destroy cache to avoid memory leak. RocksDB holds a shared_ptr to the cache
	// after table factory setup. Destroy() just decrements the refcount.
	defer cache.Destroy()

	opts, err := loadLatestOptions(dir, cache)
	if err != nil {
		return nil, err
	}
	// customize rocksdb options
	opts = newRocksdbOptions(opts, false, tuneUpOpts, cache)
	defer opts.Destroy()

	var db *grocksdb.DB
	if readonly {
		db, err = grocksdb.OpenDbForReadOnly(opts, dir, false)
	} else {
		db, err = grocksdb.OpenDb(opts, dir)
	}
	if err != nil {
		return nil, err
	}

	ro := grocksdb.NewDefaultReadOptions()
	if tuneUpOpts.EnableAsyncIo {
		ro.SetAsyncIO(true)
	}
	if tuneUpOpts.EnableAutoReadaheadSize {
		ro.SetAutoReadaheadSize(true)
	}
	wo := grocksdb.NewDefaultWriteOptions()
	woSync := grocksdb.NewDefaultWriteOptions()
	woSync.SetSync(true)
	return dbm.NewRocksDBWithRawDB(db, ro, wo, woSync), nil
}

// loadLatestOptions try to load options from existing db, returns nil if not exists.
func loadLatestOptions(dir string, cache *grocksdb.Cache) (*grocksdb.Options, error) {
	env := grocksdb.NewDefaultEnv()
	defer env.Destroy()

	opts, err := grocksdb.LoadLatestOptions(dir, env, true, cache)
	if err != nil {
		// not found is not an error
		if strings.HasPrefix(err.Error(), "NotFound: ") {
			return nil, nil
		}
		return nil, err
	}
	defer opts.Destroy()

	cfNames := opts.ColumnFamilyNames()
	cfOpts := opts.ColumnFamilyOpts()

	for i := 0; i < len(cfNames); i++ {
		if cfNames[i] == "default" {
			return cfOpts[i].Clone(), nil
		}
	}

	return opts.Options().Clone(), nil
}

// NewRocksdbOptions build options for `application.db`,
// it overrides existing options if provided, otherwise create new one assuming it's a new database.
func NewRocksdbOptions(opts *grocksdb.Options, sstFileWriter bool, tuneUpOpts RocksDBTuneUpOptions) *grocksdb.Options {
	return newRocksdbOptions(opts, sstFileWriter, tuneUpOpts, nil)
}

func newRocksdbOptions(opts *grocksdb.Options, sstFileWriter bool, tuneUpOpts RocksDBTuneUpOptions, cache *grocksdb.Cache) *grocksdb.Options {
	if opts == nil {
		opts = grocksdb.NewDefaultOptions()
		// only enable dynamic-level-bytes on new db, don't override for existing db
		opts.SetLevelCompactionDynamicLevelBytes(true)
	}
	opts.SetCreateIfMissing(true)
	opts.IncreaseParallelism(runtime.NumCPU())
	opts.OptimizeLevelStyleCompaction(512 * 1024 * 1024)
	opts.SetTargetFileSizeMultiplier(2)
	if tuneUpOpts.EnableOptimizeForPointLookup {
		opts.SetMemTablePrefixBloomSizeRatio(0.02)
		opts.SetMemtableWholeKeyFiltering(true)
	}
	if tuneUpOpts.EnableDirectIOForCompaction {
		opts.SetUseDirectIOForFlushAndCompaction(true)
	}

	// block based table options
	bbto := grocksdb.NewDefaultBlockBasedTableOptions()
	defer bbto.Destroy()

	if cache != nil {
		bbto.SetBlockCache(cache)
	} else if tuneUpOpts.EnableHyperClockCache {
		c := grocksdb.NewHyperClockCache(BlockCacheSize, 0)
		defer c.Destroy()
		bbto.SetBlockCache(c)
	} else {
		c := grocksdb.NewLRUCache(BlockCacheSize)
		defer c.Destroy()
		bbto.SetBlockCache(c)
	}

	// http://rocksdb.org/blog/2021/12/29/ribbon-filter.html
	bbto.SetFilterPolicy(grocksdb.NewRibbonHybridFilterPolicy(9.9, 1))

	// partition index
	// http://rocksdb.org/blog/2017/05/12/partitioned-index-filter.html
	bbto.SetIndexType(grocksdb.KTwoLevelIndexSearchIndexType)
	bbto.SetPartitionFilters(true)
	bbto.SetOptimizeFiltersForMemory(true)

	// reduce memory usage
	bbto.SetCacheIndexAndFilterBlocks(true)
	bbto.SetPinTopLevelIndexAndFilter(true)
	bbto.SetPinL0FilterAndIndexBlocksInCache(true)

	// hash index is better for iavl tree which mostly do point lookup.
	bbto.SetDataBlockIndexType(grocksdb.KDataBlockIndexTypeBinarySearchAndHash)
	if tuneUpOpts.EnableOptimizeForPointLookup {
		bbto.SetDataBlockHashRatio(0.75)
	}

	opts.SetBlockBasedTableFactory(bbto)

	// in iavl tree, we almost always query existing keys
	opts.SetOptimizeFiltersForHits(true)

	// heavier compression option at bottommost level,
	// 110k dict bytes is default in zstd library,
	// train bytes is recommended to be set at 100x dict bytes.
	opts.SetBottommostCompression(grocksdb.ZSTDCompression)
	compressOpts := grocksdb.NewDefaultCompressionOptions()
	compressOpts.Level = 12
	if !sstFileWriter {
		compressOpts.MaxDictBytes = 110 * 1024
		opts.SetBottommostCompressionOptionsZstdMaxTrainBytes(compressOpts.MaxDictBytes*100, true)
	}
	opts.SetBottommostCompressionOptions(compressOpts, true)
	return opts
}

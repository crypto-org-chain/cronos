package tsrocksdb

import (
	"runtime"

	"github.com/linxGnu/grocksdb"
)

const VersionDBCFName = "versiondb"

// NewVersionDBOpts returns the options used for the versiondb column family.
// FIXME: we don't enable dict compression for SSTFileWriter, because otherwise the file writer won't report correct file size.
// https://github.com/facebook/rocksdb/issues/11146
func NewVersionDBOpts(sstFileWriter bool) *grocksdb.Options {
	opts := grocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	opts.SetComparator(CreateTSComparator())
	opts.IncreaseParallelism(runtime.NumCPU())
	opts.OptimizeLevelStyleCompaction(512 * 1024 * 1024)
	opts.SetTargetFileSizeMultiplier(2)
	opts.SetLevelCompactionDynamicLevelBytes(true)

	// block based table options
	bbto := grocksdb.NewDefaultBlockBasedTableOptions()

	// 1G block cache
	bbto.SetBlockSize(32 * 1024)
	bbto.SetBlockCache(grocksdb.NewLRUCache(1 << 30))

	bbto.SetFilterPolicy(grocksdb.NewRibbonHybridFilterPolicy(9.9, 1))
	bbto.SetIndexType(grocksdb.KBinarySearchWithFirstKey)
	opts.SetBlockBasedTableFactory(bbto)
	// improve sst file creation speed: compaction or sst file writer.
	opts.SetCompressionOptionsParallelThreads(4)

	// compression options at bottommost level
	opts.SetBottommostCompression(grocksdb.ZSTDCompression)
	compressOpts := grocksdb.NewDefaultCompressionOptions()
	if !sstFileWriter {
		compressOpts.MaxDictBytes = 112640 // 110k
	}
	compressOpts.Level = 12
	opts.SetBottommostCompressionOptions(compressOpts, true)
	if !sstFileWriter {
		opts.SetBottommostCompressionOptionsZstdMaxTrainBytes(compressOpts.MaxDictBytes*100, true)
	}
	return opts
}

func OpenVersionDB(dir string) (*grocksdb.DB, *grocksdb.ColumnFamilyHandle, error) {
	opts := grocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	opts.SetCreateIfMissingColumnFamilies(true)
	db, cfHandles, err := grocksdb.OpenDbColumnFamilies(
		opts, dir, []string{"default", VersionDBCFName},
		[]*grocksdb.Options{opts, NewVersionDBOpts(false)},
	)
	if err != nil {
		return nil, nil, err
	}
	return db, cfHandles[1], nil
}

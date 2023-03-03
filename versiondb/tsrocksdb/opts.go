package tsrocksdb

import (
	"encoding/binary"
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

	// use zstd dict compression for all level start from 2
	opts.SetCompression(grocksdb.ZSTDCompression)
	compressOpts := grocksdb.NewDefaultCompressionOptions()
	// 110k dict bytes is default in zstd library,
	// train bytes is recommended to be set at 100x dict bytes.
	compressOpts.MaxDictBytes = 112640 // 110k
	compressOpts.Level = 12
	opts.SetCompressionOptions(compressOpts)
	if !sstFileWriter {
		opts.SetMinLevelToCompress(2)
		// don't enable dict compression for sst file writer.
		// see: https://github.com/facebook/rocksdb/issues/11146
		opts.SetCompressionOptionsZstdDictTrainer(true)
		opts.SetCompressionOptionsZstdMaxTrainBytes(compressOpts.MaxDictBytes * 100)
	}
	return opts
}

// OpenVersionDB opens versiondb, the default column family is used for metadata,
// actually key-value pairs are stored on another column family named with "versiondb",
// which has user-defined timestamp enabled.
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

// OpenVersionDBAndTrimHistory opens versiondb similar to `OpenVersionDB`,
// but it also trim the versions newer than target one, can be used for rollback.
func OpenVersionDBAndTrimHistory(dir string, version int64) (*grocksdb.DB, *grocksdb.ColumnFamilyHandle, error) {
	var ts [TimestampSize]byte
	binary.LittleEndian.PutUint64(ts[:], uint64(version))

	opts := grocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	opts.SetCreateIfMissingColumnFamilies(true)
	db, cfHandles, err := grocksdb.OpenDbAndTrimHistory(
		opts, dir, []string{"default", VersionDBCFName},
		[]*grocksdb.Options{opts, NewVersionDBOpts(false)},
		ts[:],
	)
	if err != nil {
		return nil, nil, err
	}
	return db, cfHandles[1], nil
}

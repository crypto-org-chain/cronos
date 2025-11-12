//go:build rocksdb
// +build rocksdb

package dbmigrate

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/linxGnu/grocksdb"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
)

// newRocksDBOptions creates RocksDB options similar to the app configuration
func newRocksDBOptions() *grocksdb.Options {
	opts := grocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	opts.SetLevelCompactionDynamicLevelBytes(true)
	opts.IncreaseParallelism(runtime.NumCPU())
	opts.OptimizeLevelStyleCompaction(512 * 1024 * 1024)
	opts.SetTargetFileSizeMultiplier(2)

	// block based table options
	bbto := grocksdb.NewDefaultBlockBasedTableOptions()
	bbto.SetBlockCache(grocksdb.NewLRUCache(3 << 30)) // 3GB
	bbto.SetFilterPolicy(grocksdb.NewRibbonHybridFilterPolicy(9.9, 1))
	bbto.SetIndexType(grocksdb.KTwoLevelIndexSearchIndexType)
	bbto.SetPartitionFilters(true)
	bbto.SetOptimizeFiltersForMemory(true)
	bbto.SetCacheIndexAndFilterBlocks(true)
	bbto.SetPinTopLevelIndexAndFilter(true)
	bbto.SetPinL0FilterAndIndexBlocksInCache(true)
	bbto.SetDataBlockIndexType(grocksdb.KDataBlockIndexTypeBinarySearchAndHash)
	opts.SetBlockBasedTableFactory(bbto)
	opts.SetOptimizeFiltersForHits(true)

	return opts
}

// setupRocksDB creates a test RocksDB database with sample data
func setupRocksDB(t *testing.T, numKeys int) (string, dbm.DB) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	err := os.MkdirAll(dataDir, 0755)
	require.NoError(t, err)

	opts := newRocksDBOptions()
	t.Cleanup(func() { opts.Destroy() })
	rocksDir := filepath.Join(dataDir, "application.db")
	rawDB, err := grocksdb.OpenDb(opts, rocksDir)
	require.NoError(t, err)

	ro := grocksdb.NewDefaultReadOptions()
	t.Cleanup(func() { ro.Destroy() })
	wo := grocksdb.NewDefaultWriteOptions()
	t.Cleanup(func() { wo.Destroy() })
	woSync := grocksdb.NewDefaultWriteOptions()
	t.Cleanup(func() { woSync.Destroy() })
	woSync.SetSync(true)
	db := dbm.NewRocksDBWithRawDB(rawDB, ro, wo, woSync)

	// Populate with test data
	for i := 0; i < numKeys; i++ {
		key := []byte(fmt.Sprintf("key-%06d", i))
		value := []byte(fmt.Sprintf("value-%06d-data-for-testing-rocksdb-migration", i))
		err := db.Set(key, value)
		require.NoError(t, err)
	}

	return tempDir, db
}

// TestMigrateLevelDBToRocksDB tests migration from LevelDB to RocksDB
func TestMigrateLevelDBToRocksDB(t *testing.T) {
	numKeys := 1000

	// Setup source database with LevelDB
	sourceDir, sourceDB := setupTestDB(t, dbm.GoLevelDBBackend, numKeys)

	// Store expected key-value pairs
	expectedData := make(map[string]string)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%06d", i)
		value := fmt.Sprintf("value-%06d-data-for-testing-migration", i)
		expectedData[key] = value
	}
	sourceDB.Close()

	// Create target directory
	targetDir := t.TempDir()

	// Perform migration
	rocksOpts := newRocksDBOptions()
	defer rocksOpts.Destroy()
	opts := MigrateOptions{
		SourceHome:     sourceDir,
		TargetHome:     targetDir,
		SourceBackend:  dbm.GoLevelDBBackend,
		TargetBackend:  dbm.RocksDBBackend,
		BatchSize:      100,
		Logger:         log.NewTestLogger(t),
		RocksDBOptions: rocksOpts,
		Verify:         true,
	}

	stats, err := Migrate(opts)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, int64(numKeys), stats.TotalKeys.Load())
	require.Equal(t, int64(numKeys), stats.ProcessedKeys.Load())
	require.Equal(t, int64(0), stats.ErrorCount.Load())

	// Verify the migrated data by opening the target database
	targetDBPath := filepath.Join(targetDir, "data", "application.db.migrate-temp")
	targetDB, err := openRocksDBForRead(targetDBPath)
	require.NoError(t, err)
	defer targetDB.Close()

	// Check a few random keys
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key-%06d", i))
		value, err := targetDB.Get(key)
		require.NoError(t, err)
		expectedValue := []byte(expectedData[string(key)])
		require.Equal(t, expectedValue, value)
	}
}

// TestMigrateRocksDBToLevelDB tests migration from RocksDB to LevelDB
func TestMigrateRocksDBToLevelDB(t *testing.T) {
	numKeys := 500

	// Setup source database with RocksDB
	sourceDir, sourceDB := setupRocksDB(t, numKeys)
	sourceDB.Close()

	// Create target directory
	targetDir := t.TempDir()

	// Perform migration
	opts := MigrateOptions{
		SourceHome:     sourceDir,
		TargetHome:     targetDir,
		SourceBackend:  dbm.RocksDBBackend,
		TargetBackend:  dbm.GoLevelDBBackend,
		BatchSize:      50,
		Logger:         log.NewTestLogger(t),
		RocksDBOptions: newRocksDBOptions(),
		Verify:         true,
	}

	stats, err := Migrate(opts)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, int64(numKeys), stats.TotalKeys.Load())
	require.Equal(t, int64(numKeys), stats.ProcessedKeys.Load())
	require.Equal(t, int64(0), stats.ErrorCount.Load())
}

// TestMigrateRocksDBToRocksDB tests migration between RocksDB instances
func TestMigrateRocksDBToRocksDB(t *testing.T) {
	numKeys := 300

	// Setup source database with RocksDB
	sourceDir, sourceDB := setupRocksDB(t, numKeys)
	sourceDB.Close()

	// Create target directory
	targetDir := t.TempDir()

	// Perform migration (useful for compaction or options change)
	rocksOpts := newRocksDBOptions()
	defer rocksOpts.Destroy()
	opts := MigrateOptions{
		SourceHome:     sourceDir,
		TargetHome:     targetDir,
		SourceBackend:  dbm.RocksDBBackend,
		TargetBackend:  dbm.RocksDBBackend,
		BatchSize:      100,
		Logger:         log.NewTestLogger(t),
		RocksDBOptions: rocksOpts,
		Verify:         true,
	}

	stats, err := Migrate(opts)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, int64(numKeys), stats.TotalKeys.Load())
	require.Equal(t, int64(numKeys), stats.ProcessedKeys.Load())
	require.Equal(t, int64(0), stats.ErrorCount.Load())
}

// TestMigrateRocksDBLargeDataset tests RocksDB migration with a large dataset
func TestMigrateRocksDBLargeDataset(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large dataset test in short mode")
	}

	numKeys := 50000

	// Setup source database with RocksDB
	sourceDir, sourceDB := setupRocksDB(t, numKeys)
	sourceDB.Close()

	// Create target directory
	targetDir := t.TempDir()

	// Perform migration
	opts := MigrateOptions{
		SourceHome:     sourceDir,
		TargetHome:     targetDir,
		SourceBackend:  dbm.RocksDBBackend,
		TargetBackend:  dbm.RocksDBBackend,
		BatchSize:      1000,
		Logger:         log.NewTestLogger(t),
		RocksDBOptions: newRocksDBOptions(),
		Verify:         false, // Skip verification for speed
	}

	stats, err := Migrate(opts)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, int64(numKeys), stats.TotalKeys.Load())
	require.Equal(t, int64(numKeys), stats.ProcessedKeys.Load())
	require.Equal(t, int64(0), stats.ErrorCount.Load())

	t.Logf("Migrated %d keys in %s", numKeys, stats.Duration())
}

// TestMigrateRocksDBWithDifferentOptions tests migration with custom RocksDB options
func TestMigrateRocksDBWithDifferentOptions(t *testing.T) {
	numKeys := 100

	// Setup source database
	sourceDir, sourceDB := setupTestDB(t, dbm.GoLevelDBBackend, numKeys)
	sourceDB.Close()

	// Create target directory
	targetDir := t.TempDir()

	// Create custom RocksDB options with different settings
	customOpts := grocksdb.NewDefaultOptions()
	defer customOpts.Destroy()
	customOpts.SetCreateIfMissing(true)
	customOpts.SetLevelCompactionDynamicLevelBytes(true)
	// Different compression
	customOpts.SetCompression(grocksdb.SnappyCompression)

	// Perform migration with custom options
	opts := MigrateOptions{
		SourceHome:     sourceDir,
		TargetHome:     targetDir,
		SourceBackend:  dbm.GoLevelDBBackend,
		TargetBackend:  dbm.RocksDBBackend,
		BatchSize:      50,
		Logger:         log.NewTestLogger(t),
		RocksDBOptions: customOpts,
		Verify:         true,
	}

	stats, err := Migrate(opts)
	require.NoError(t, err)
	require.Equal(t, int64(numKeys), stats.TotalKeys.Load())
	require.Equal(t, int64(numKeys), stats.ProcessedKeys.Load())
}

// TestMigrateRocksDBDataIntegrity tests that data integrity is maintained during migration
func TestMigrateRocksDBDataIntegrity(t *testing.T) {
	numKeys := 1000

	// Setup source database
	sourceDir, sourceDB := setupTestDB(t, dbm.GoLevelDBBackend, numKeys)

	// Read all source data before closing
	sourceData := make(map[string][]byte)
	itr, err := sourceDB.Iterator(nil, nil)
	require.NoError(t, err)
	for ; itr.Valid(); itr.Next() {
		key := make([]byte, len(itr.Key()))
		value := make([]byte, len(itr.Value()))
		copy(key, itr.Key())
		copy(value, itr.Value())
		sourceData[string(key)] = value
	}
	require.NoError(t, itr.Error())
	itr.Close()
	sourceDB.Close()

	// Perform migration
	rocksOpts := newRocksDBOptions()
	defer rocksOpts.Destroy()
	opts := MigrateOptions{
		SourceHome:     sourceDir,
		TargetHome:     targetDir,
		SourceBackend:  dbm.GoLevelDBBackend,
		TargetBackend:  dbm.RocksDBBackend,
		BatchSize:      100,
		Logger:         log.NewNopLogger(),
		RocksDBOptions: rocksOpts,
		Verify:         false,
	}

	stats, err := Migrate(opts)
	require.NoError(t, err)
	require.Equal(t, int64(numKeys), stats.TotalKeys.Load())
	require.NoError(t, err)
	require.Equal(t, int64(numKeys), stats.TotalKeys.Load())

	// Open target database and verify all data
	targetDBPath := filepath.Join(targetDir, "data", "application.db.migrate-temp")
	targetDB, err := openRocksDBForRead(targetDBPath)
	require.NoError(t, err)
	defer targetDB.Close()

	// Verify every key
	verifiedCount := 0
	for key, expectedValue := range sourceData {
		actualValue, err := targetDB.Get([]byte(key))
		require.NoError(t, err, "Failed to get key: %s", key)
		require.Equal(t, expectedValue, actualValue, "Value mismatch for key: %s", key)
		verifiedCount++
	}

	require.Equal(t, len(sourceData), verifiedCount)
	t.Logf("Verified %d keys successfully", verifiedCount)
}
}

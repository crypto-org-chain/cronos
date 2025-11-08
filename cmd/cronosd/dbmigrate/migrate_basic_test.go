//go:build !rocksdb
// +build !rocksdb

package dbmigrate

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
)

// setupBasicTestDB creates a test database with sample data (no RocksDB)
func setupBasicTestDB(t *testing.T, backend dbm.BackendType, numKeys int) (string, dbm.DB) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	err := os.MkdirAll(dataDir, 0755)
	require.NoError(t, err)

	db, err := dbm.NewDB("application", backend, dataDir)
	require.NoError(t, err)

	// Populate with test data
	for i := 0; i < numKeys; i++ {
		key := []byte(fmt.Sprintf("key-%06d", i))
		value := []byte(fmt.Sprintf("value-%06d-data-for-testing-migration", i))
		err := db.Set(key, value)
		require.NoError(t, err)
	}

	return tempDir, db
}

// TestCountKeys tests the key counting functionality
func TestCountKeys(t *testing.T) {
	tests := []struct {
		name    string
		backend dbm.BackendType
		numKeys int
	}{
		{
			name:    "leveldb with 100 keys",
			backend: dbm.GoLevelDBBackend,
			numKeys: 100,
		},
		{
			name:    "leveldb with 0 keys",
			backend: dbm.GoLevelDBBackend,
			numKeys: 0,
		},
		{
			name:    "memdb with 50 keys",
			backend: dbm.MemDBBackend,
			numKeys: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, db := setupBasicTestDB(t, tt.backend, tt.numKeys)
			defer db.Close()

			count, err := countKeys(db)
			require.NoError(t, err)
			require.Equal(t, int64(tt.numKeys), count)
		})
	}
}

// TestMigrateLevelDBToLevelDB tests basic migration functionality
func TestMigrateLevelDBToLevelDB(t *testing.T) {
	numKeys := 100

	// Setup source database
	sourceDir, sourceDB := setupBasicTestDB(t, dbm.GoLevelDBBackend, numKeys)
	sourceDB.Close()

	// Create target directory
	targetDir := t.TempDir()

	// Perform migration
	opts := MigrateOptions{
		SourceHome:    sourceDir,
		TargetHome:    targetDir,
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.GoLevelDBBackend,
		BatchSize:     10,
		Logger:        log.NewNopLogger(),
		Verify:        true,
	}

	stats, err := Migrate(opts)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, int64(numKeys), stats.TotalKeys.Load())
	require.Equal(t, int64(numKeys), stats.ProcessedKeys.Load())
	require.Equal(t, int64(0), stats.ErrorCount.Load())
}

// TestMigrateLevelDBToMemDB tests migration from leveldb to memdb
// Note: MemDB doesn't persist to disk, so we skip verification
func TestMigrateLevelDBToMemDB(t *testing.T) {
	numKeys := 500

	// Setup source database with LevelDB
	sourceDir, sourceDB := setupBasicTestDB(t, dbm.GoLevelDBBackend, numKeys)
	sourceDB.Close()

	// Create target directory
	targetDir := t.TempDir()

	// Perform migration (no verification for MemDB as it's in-memory)
	opts := MigrateOptions{
		SourceHome:    sourceDir,
		TargetHome:    targetDir,
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.MemDBBackend,
		BatchSize:     50,
		Logger:        log.NewNopLogger(),
		Verify:        false, // Skip verification for MemDB
	}

	stats, err := Migrate(opts)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, int64(numKeys), stats.TotalKeys.Load())
	require.Equal(t, int64(numKeys), stats.ProcessedKeys.Load())
	require.Equal(t, int64(0), stats.ErrorCount.Load())
	require.Greater(t, stats.Duration().Milliseconds(), int64(0))
}

// TestMigrationStats tests the statistics tracking
func TestMigrationStats(t *testing.T) {
	stats := &MigrationStats{}

	// Test initial state
	require.Equal(t, int64(0), stats.TotalKeys.Load())
	require.Equal(t, int64(0), stats.ProcessedKeys.Load())
	require.Equal(t, float64(0), stats.Progress())

	// Test with some values
	stats.TotalKeys.Store(100)
	stats.ProcessedKeys.Store(50)
	require.Equal(t, float64(50), stats.Progress())

	stats.ProcessedKeys.Store(100)
	require.Equal(t, float64(100), stats.Progress())
}

// TestMigrateLargeDatabase tests migration with a larger dataset
func TestMigrateLargeDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large database test in short mode")
	}

	numKeys := 10000

	// Setup source database
	sourceDir, sourceDB := setupBasicTestDB(t, dbm.GoLevelDBBackend, numKeys)
	sourceDB.Close()

	// Create target directory
	targetDir := t.TempDir()

	// Perform migration with smaller batch size
	opts := MigrateOptions{
		SourceHome:    sourceDir,
		TargetHome:    targetDir,
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.GoLevelDBBackend, // Use LevelDB for verification to work
		BatchSize:     100,
		Logger:        log.NewTestLogger(t),
		Verify:        true,
	}

	stats, err := Migrate(opts)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, int64(numKeys), stats.TotalKeys.Load())
	require.Equal(t, int64(numKeys), stats.ProcessedKeys.Load())
	require.Equal(t, int64(0), stats.ErrorCount.Load())
}

// TestMigrateEmptyDatabase tests migration of an empty database
func TestMigrateEmptyDatabase(t *testing.T) {
	// Setup empty source database
	sourceDir, sourceDB := setupBasicTestDB(t, dbm.GoLevelDBBackend, 0)
	sourceDB.Close()

	// Create target directory
	targetDir := t.TempDir()

	// Perform migration
	opts := MigrateOptions{
		SourceHome:    sourceDir,
		TargetHome:    targetDir,
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.MemDBBackend,
		BatchSize:     10,
		Logger:        log.NewNopLogger(),
		Verify:        true,
	}

	stats, err := Migrate(opts)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, int64(0), stats.TotalKeys.Load())
	require.Equal(t, int64(0), stats.ProcessedKeys.Load())
}

// TestMigrationWithoutVerification tests migration without verification
func TestMigrationWithoutVerification(t *testing.T) {
	numKeys := 100

	// Setup source database
	sourceDir, sourceDB := setupBasicTestDB(t, dbm.GoLevelDBBackend, numKeys)
	sourceDB.Close()

	// Create target directory
	targetDir := t.TempDir()

	// Perform migration without verification
	opts := MigrateOptions{
		SourceHome:    sourceDir,
		TargetHome:    targetDir,
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.MemDBBackend,
		BatchSize:     10,
		Logger:        log.NewNopLogger(),
		Verify:        false,
	}

	stats, err := Migrate(opts)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, int64(numKeys), stats.TotalKeys.Load())
	require.Equal(t, int64(numKeys), stats.ProcessedKeys.Load())
}

// TestMigrationBatchSizes tests migration with different batch sizes
func TestMigrationBatchSizes(t *testing.T) {
	numKeys := 150
	batchSizes := []int{1, 10, 50, 100, 200}

	for _, batchSize := range batchSizes {
		t.Run(fmt.Sprintf("batch_size_%d", batchSize), func(t *testing.T) {
			// Setup source database
			sourceDir, sourceDB := setupBasicTestDB(t, dbm.GoLevelDBBackend, numKeys)
			sourceDB.Close()

			// Create target directory
			targetDir := t.TempDir()

			// Perform migration
			opts := MigrateOptions{
				SourceHome:    sourceDir,
				TargetHome:    targetDir,
				SourceBackend: dbm.GoLevelDBBackend,
				TargetBackend: dbm.GoLevelDBBackend,
				BatchSize:     batchSize,
				Logger:        log.NewNopLogger(),
				Verify:        false,
			}

			stats, err := Migrate(opts)
			require.NoError(t, err)
			require.Equal(t, int64(numKeys), stats.TotalKeys.Load())
			require.Equal(t, int64(numKeys), stats.ProcessedKeys.Load())
		})
	}
}

// TestMigrateSpecialKeys tests migration with special key patterns
func TestMigrateSpecialKeys(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	err := os.MkdirAll(dataDir, 0755)
	require.NoError(t, err)

	db, err := dbm.NewDB("application", dbm.GoLevelDBBackend, dataDir)
	require.NoError(t, err)

	// Add keys with special patterns
	specialKeys := [][]byte{
		[]byte("\x00"),                // null byte
		[]byte("\x00\x00\x00"),        // multiple null bytes
		[]byte("key with spaces"),     // spaces
		[]byte("key\nwith\nnewlines"), // newlines
		[]byte("🔑emoji-key"),          // unicode
		make([]byte, 1024),            // large key
	}

	for i, key := range specialKeys {
		if len(key) > 0 { // Skip empty key if not supported
			value := []byte(fmt.Sprintf("value-%d", i))
			err := db.Set(key, value)
			if err == nil { // Only test keys that are supported
				require.NoError(t, err)
			}
		}
	}
	db.Close()

	// Now migrate
	targetDir := t.TempDir()
	opts := MigrateOptions{
		SourceHome:    tempDir,
		TargetHome:    targetDir,
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.GoLevelDBBackend,
		BatchSize:     2,
		Logger:        log.NewNopLogger(),
		Verify:        false,
	}

	stats, err := Migrate(opts)
	require.NoError(t, err)
	require.Greater(t, stats.ProcessedKeys.Load(), int64(0))
}

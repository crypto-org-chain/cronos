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
	t.Helper()
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	err := os.MkdirAll(dataDir, 0o755)
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
		TargetBackend: dbm.GoLevelDBBackend,
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
		TargetBackend: dbm.GoLevelDBBackend,
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
	err := os.MkdirAll(dataDir, 0o755)
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
			require.NoError(t, err)
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

// TestMigrationPathCorrectness verifies that logged paths match actual database locations
// Unified path format for all backends: <dbName>.migrate-temp.db
func TestMigrationPathCorrectness(t *testing.T) {
	tests := []struct {
		name           string
		backend        dbm.BackendType
		expectedSuffix string
	}{
		{
			name:           "LevelDB uses unified .migrate-temp.db format",
			backend:        dbm.GoLevelDBBackend,
			expectedSuffix: ".migrate-temp.db",
		},
		// Note: RocksDB also uses .migrate-temp.db but requires rocksdb build tag to test
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup source database
			sourceDir, sourceDB := setupBasicTestDB(t, tt.backend, 10)
			sourceDB.Close()

			// Create target directory
			targetDir := t.TempDir()

			// Perform migration
			opts := MigrateOptions{
				SourceHome:    sourceDir,
				TargetHome:    targetDir,
				SourceBackend: tt.backend,
				TargetBackend: tt.backend,
				DBName:        "application",
				BatchSize:     10,
				Logger:        log.NewNopLogger(),
				Verify:        false,
			}

			stats, err := Migrate(opts)
			require.NoError(t, err)
			require.NotNil(t, stats)

			// Verify the actual database directory exists
			targetDataDir := filepath.Join(targetDir, "data")
			expectedPath := filepath.Join(targetDataDir, "application"+tt.expectedSuffix)

			// Check that the directory exists
			info, err := os.Stat(expectedPath)
			require.NoError(t, err, "Database directory should exist at expected path: %s", expectedPath)
			require.True(t, info.IsDir(), "Expected path should be a directory")

			// Verify we can open the database at this path
			db, err := dbm.NewDB("application.migrate-temp", tt.backend, targetDataDir)
			require.NoError(t, err, "Should be able to open database at the expected path")
			defer db.Close()

			// Verify it has the correct data
			count, err := countKeys(db)
			require.NoError(t, err)
			require.Equal(t, int64(10), count, "Database should contain all migrated keys")
		})
	}
}

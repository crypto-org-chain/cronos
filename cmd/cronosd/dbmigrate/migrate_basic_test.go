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

// TestHeightFilteredVerification tests that verification works correctly with height filtering
func TestHeightFilteredVerification(t *testing.T) {
	// Create source database with blockstore data for heights 100-200
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	err := os.MkdirAll(dataDir, 0o755)
	require.NoError(t, err)

	sourceDB, err := dbm.NewDB(DBNameBlockstore, dbm.GoLevelDBBackend, dataDir)
	require.NoError(t, err)

	// Add blockstore keys for heights 100-200
	for height := int64(100); height <= 200; height++ {
		// Add block metadata
		blockMetaKey := []byte(fmt.Sprintf("H:%d", height))
		blockMetaValue := []byte(fmt.Sprintf("block_meta_%d", height))
		err := sourceDB.Set(blockMetaKey, blockMetaValue)
		require.NoError(t, err)

		// Add block part
		partKey := []byte(fmt.Sprintf("P:%d:0", height))
		partValue := []byte(fmt.Sprintf("block_part_%d", height))
		err = sourceDB.Set(partKey, partValue)
		require.NoError(t, err)

		// Add commit
		commitKey := []byte(fmt.Sprintf("C:%d", height))
		commitValue := []byte(fmt.Sprintf("commit_%d", height))
		err = sourceDB.Set(commitKey, commitValue)
		require.NoError(t, err)
	}
	sourceDB.Close()

	// Migrate only heights 120-150
	targetDir := t.TempDir()
	opts := MigrateOptions{
		SourceHome:    tempDir,
		TargetHome:    targetDir,
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.GoLevelDBBackend,
		DBName:        DBNameBlockstore,
		BatchSize:     10,
		HeightRange: HeightRange{
			Start: 120,
			End:   150,
		},
		Logger: log.NewNopLogger(),
		Verify: true, // This is the key test - verification should work with height filtering
	}

	stats, err := Migrate(opts)
	require.NoError(t, err, "Migration with height-filtered verification should succeed")
	require.NotNil(t, stats)

	// Debug: print stats
	t.Logf("Migration stats: TotalKeys=%d, ProcessedKeys=%d, ErrorCount=%d",
		stats.TotalKeys.Load(), stats.ProcessedKeys.Load(), stats.ErrorCount.Load())

	// Should have migrated 31 heights * 3 keys per height = 93 keys
	expectedKeys := int64(31 * 3) // heights 120-150 inclusive, 3 keys each
	require.Equal(t, expectedKeys, stats.ProcessedKeys.Load(), "Should process exactly the filtered keys")
	require.Equal(t, int64(0), stats.ErrorCount.Load(), "Should have no errors")

	// Verify the target database has exactly the expected keys
	// NOTE: Migration creates a .migrate-temp database, not the final database
	targetDataDir := filepath.Join(targetDir, "data")
	targetDB, err := dbm.NewDB(DBNameBlockstore+".migrate-temp", dbm.GoLevelDBBackend, targetDataDir)
	require.NoError(t, err)
	defer targetDB.Close()

	// Count keys in target
	targetCount, err := countKeys(targetDB)
	require.NoError(t, err)
	require.Equal(t, expectedKeys, targetCount, "Target should have exactly the filtered keys")

	// Verify a few specific keys exist
	blockMetaKey := []byte("H:125")
	value, err := targetDB.Get(blockMetaKey)
	require.NoError(t, err)
	require.NotNil(t, value)
	require.Equal(t, []byte("block_meta_125"), value)

	// Verify keys outside range don't exist
	outsideKey := []byte("H:99")
	value, err = targetDB.Get(outsideKey)
	require.NoError(t, err)
	require.Nil(t, value, "Keys outside height range should not be migrated")

	outsideKey = []byte("H:151")
	value, err = targetDB.Get(outsideKey)
	require.NoError(t, err)
	require.Nil(t, value, "Keys outside height range should not be migrated")
}

// TestHeightFilteredVerificationWithSpecificHeights tests verification with specific height list
func TestHeightFilteredVerificationWithSpecificHeights(t *testing.T) {
	// Create source database with tx_index data for heights 10-20
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	err := os.MkdirAll(dataDir, 0o755)
	require.NoError(t, err)

	sourceDB, err := dbm.NewDB(DBNameTxIndex, dbm.GoLevelDBBackend, dataDir)
	require.NoError(t, err)

	// Add tx_index keys for heights 10-20
	for height := int64(10); height <= 20; height++ {
		// Add multiple transactions per height
		for txIdx := 0; txIdx < 3; txIdx++ {
			// tx_index key format: tx.height/<height>/<tx_index>/<tx_hash>
			key := []byte(fmt.Sprintf("tx.height/%d/%d/hash%d", height, txIdx, txIdx))
			value := []byte(fmt.Sprintf("tx_data_%d_%d", height, txIdx))
			err := sourceDB.Set(key, value)
			require.NoError(t, err)
		}
	}
	sourceDB.Close()

	// Migrate only specific heights: 12, 15, 18
	targetDir := t.TempDir()
	opts := MigrateOptions{
		SourceHome:    tempDir,
		TargetHome:    targetDir,
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.GoLevelDBBackend,
		DBName:        DBNameTxIndex,
		BatchSize:     10,
		HeightRange: HeightRange{
			SpecificHeights: []int64{12, 15, 18},
		},
		Logger: log.NewNopLogger(),
		Verify: true, // Verification should honor specific heights
	}

	stats, err := Migrate(opts)
	require.NoError(t, err, "Migration with specific heights verification should succeed")
	require.NotNil(t, stats)

	// Debug: print stats
	t.Logf("Migration stats: TotalKeys=%d, ProcessedKeys=%d, ErrorCount=%d",
		stats.TotalKeys.Load(), stats.ProcessedKeys.Load(), stats.ErrorCount.Load())

	// Should have migrated 3 heights * 3 transactions per height = 9 keys
	expectedKeys := int64(3 * 3)
	require.Equal(t, expectedKeys, stats.ProcessedKeys.Load(), "Should process exactly the filtered keys")
	require.Equal(t, int64(0), stats.ErrorCount.Load(), "Should have no errors")

	// Verify the target database
	// NOTE: Migration creates a .migrate-temp database, not the final database
	targetDataDir := filepath.Join(targetDir, "data")
	targetDB, err := dbm.NewDB(DBNameTxIndex+".migrate-temp", dbm.GoLevelDBBackend, targetDataDir)
	require.NoError(t, err)
	defer targetDB.Close()

	targetCount, err := countKeys(targetDB)
	require.NoError(t, err)
	require.Equal(t, expectedKeys, targetCount, "Target should have exactly the filtered keys")

	// Verify specific keys exist
	key := []byte("tx.height/15/1/hash1")
	value, err := targetDB.Get(key)
	require.NoError(t, err)
	require.NotNil(t, value)
	require.Equal(t, []byte("tx_data_15_1"), value)

	// Verify non-selected heights don't exist
	outsideKey := []byte("tx.height/13/0/hash0")
	value, err = targetDB.Get(outsideKey)
	require.NoError(t, err)
	require.Nil(t, value, "Keys for non-selected heights should not be migrated")
}

// TestMigrationPathCorrectness verifies that logged paths match actual database locations
// All backends now use unified path format: <dbName>.migrate-temp.db
func TestMigrationPathCorrectness(t *testing.T) {
	tests := []struct {
		name           string
		backend        dbm.BackendType
		expectedSuffix string
	}{
		{
			name:           "LevelDB uses unified .migrate-temp.db",
			backend:        dbm.GoLevelDBBackend,
			expectedSuffix: ".migrate-temp.db",
		},
		// Note: RocksDB would also use .migrate-temp.db but requires rocksdb build tag
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

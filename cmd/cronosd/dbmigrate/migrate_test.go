//go:build rocksdb
// +build rocksdb

package dbmigrate

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/linxGnu/grocksdb"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
)

// setupTestDB creates a test database with sample data
func setupTestDB(t *testing.T, backend dbm.BackendType, numKeys int) (string, dbm.DB) {
	t.Helper()
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	err := os.MkdirAll(dataDir, 0o755)
	require.NoError(t, err)

	var db dbm.DB
	if backend == dbm.RocksDBBackend {
		opts := grocksdb.NewDefaultOptions()
		defer opts.Destroy()
		opts.SetCreateIfMissing(true)
		rocksDir := filepath.Join(dataDir, "application.db")
		rawDB, err := grocksdb.OpenDb(opts, rocksDir)
		require.NoError(t, err)

		ro := grocksdb.NewDefaultReadOptions()
		defer ro.Destroy()
		wo := grocksdb.NewDefaultWriteOptions()
		defer wo.Destroy()
		woSync := grocksdb.NewDefaultWriteOptions()
		defer woSync.Destroy()
		woSync.SetSync(true)
		db = dbm.NewRocksDBWithRawDB(rawDB, ro, wo, woSync)
	} else {
		db, err = dbm.NewDB("application", backend, dataDir)
		require.NoError(t, err)
	}

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
			_, db := setupTestDB(t, tt.backend, tt.numKeys)
			defer db.Close()

			count, err := countKeys(db)
			require.NoError(t, err)
			require.Equal(t, int64(tt.numKeys), count)
		})
	}
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
	sourceDir, sourceDB := setupTestDB(t, dbm.GoLevelDBBackend, numKeys)
	sourceDB.Close()

	// Create target directory
	targetDir := t.TempDir()

	// Perform migration with smaller batch size
	opts := MigrateOptions{
		SourceHome:    sourceDir,
		TargetHome:    targetDir,
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.GoLevelDBBackend,
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
	sourceDir, sourceDB := setupTestDB(t, dbm.GoLevelDBBackend, 0)
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
	sourceDir, sourceDB := setupTestDB(t, dbm.GoLevelDBBackend, numKeys)
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
			sourceDir, sourceDB := setupTestDB(t, dbm.GoLevelDBBackend, numKeys)
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

// TestVerifyMigration tests the verification functionality
func TestVerifyMigration(t *testing.T) {
	tests := []struct {
		name          string
		numKeys       int
		setupMismatch func(sourceDB, targetDB dbm.DB) error
		expectError   bool
	}{
		{
			name:          "identical databases should pass verification",
			numKeys:       50,
			setupMismatch: nil,
			expectError:   false,
		},
		{
			name:    "value mismatch should fail verification",
			numKeys: 50,
			setupMismatch: func(sourceDB, targetDB dbm.DB) error {
				// Change a value in target
				return targetDB.Set([]byte("key-000010"), []byte("different-value"))
			},
			expectError: true,
		},
		{
			name:    "extra key in target should fail verification",
			numKeys: 50,
			setupMismatch: func(sourceDB, targetDB dbm.DB) error {
				// Add an extra key to target that doesn't exist in source
				return targetDB.Set([]byte("extra-key-in-target"), []byte("extra-value"))
			},
			expectError: true,
		},
		{
			name:    "missing key in target should fail verification",
			numKeys: 50,
			setupMismatch: func(sourceDB, targetDB dbm.DB) error {
				// Delete a key from target
				return targetDB.Delete([]byte("key-000010"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup source database
			sourceDir, sourceDB := setupTestDB(t, dbm.GoLevelDBBackend, tt.numKeys)
			defer sourceDB.Close()

			// Setup target database by copying data from source
			targetDir := t.TempDir()
			targetDataDir := filepath.Join(targetDir, "data")
			err := os.MkdirAll(targetDataDir, 0o755)
			require.NoError(t, err)

			targetDB, err := dbm.NewDB("application.migrate-temp", dbm.GoLevelDBBackend, targetDataDir)
			require.NoError(t, err)

			// Copy all data from source to target
			itr, err := sourceDB.Iterator(nil, nil)
			require.NoError(t, err)
			defer itr.Close()
			for ; itr.Valid(); itr.Next() {
				err := targetDB.Set(itr.Key(), itr.Value())
				require.NoError(t, err)
			}

			// Apply mismatch if specified
			if tt.setupMismatch != nil {
				err := tt.setupMismatch(sourceDB, targetDB)
				require.NoError(t, err)
			}

			// Close databases before verification
			sourceDB.Close()
			targetDB.Close()

			opts := MigrateOptions{
				SourceHome:    sourceDir,
				TargetHome:    targetDir,
				SourceBackend: dbm.GoLevelDBBackend,
				TargetBackend: dbm.GoLevelDBBackend,
				DBName:        "application",
				Logger:        log.NewNopLogger(),
			}

			// Perform verification
			err = verifyMigration(
				filepath.Join(sourceDir, "data"),
				filepath.Join(targetDataDir, "application.migrate-temp.db"),
				opts,
			)

			if tt.expectError {
				require.Error(t, err, "expected verification to fail but it passed")
			} else {
				require.NoError(t, err, "expected verification to pass but it failed")
			}
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
	type keyValuePair struct {
		key   []byte
		value []byte
	}

	specialKeys := [][]byte{
		[]byte(""),                    // empty key (may not be supported)
		[]byte("\x00"),                // null byte
		[]byte("\x00\x00\x00"),        // multiple null bytes
		[]byte("key with spaces"),     // spaces
		[]byte("key\nwith\nnewlines"), // newlines
		[]byte("🔑emoji-key"),          // unicode
		make([]byte, 1024),            // large key
	}

	// Track successfully written keys
	var expectedKeys []keyValuePair

	for i, key := range specialKeys {
		value := []byte(fmt.Sprintf("value-%d", i))
		err := db.Set(key, value)
		if err != nil {
			// Only skip empty key if explicitly unsupported
			if len(key) == 0 {
				t.Logf("Skipping empty key (unsupported): %v", err)
				continue
			}
			// Any other key failure is unexpected and should fail the test
			require.NoError(t, err, "unexpected error setting key at index %d", i)
		}

		// Record successfully written key
		expectedKeys = append(expectedKeys, keyValuePair{
			key:   key,
			value: value,
		})
		t.Logf("Successfully wrote key %d: len=%d", i, len(key))
	}
	db.Close()

	require.Greater(t, len(expectedKeys), 0, "no keys were successfully written to source DB")

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

	// Assert no migration errors
	require.NoError(t, err, "migration should complete without error")
	require.Equal(t, int64(0), stats.ErrorCount.Load(), "migration should have zero errors")

	// Assert the number of migrated keys equals the number written
	require.Equal(t, int64(len(expectedKeys)), stats.ProcessedKeys.Load(),
		"number of migrated keys should equal number of keys written")

	// Open target DB and verify each expected key
	targetDataDir := filepath.Join(targetDir, "data")
	targetDB, err := dbm.NewDB("application.migrate-temp", dbm.GoLevelDBBackend, targetDataDir)
	require.NoError(t, err)
	defer targetDB.Close()

	for i, pair := range expectedKeys {
		gotValue, err := targetDB.Get(pair.key)
		require.NoError(t, err, "failed to get key %d from target DB", i)
		require.NotNil(t, gotValue, "key %d should exist in target DB", i)
		require.Equal(t, pair.value, gotValue,
			"value for key %d should match expected value", i)
		t.Logf("Verified key %d: value matches", i)
	}
}

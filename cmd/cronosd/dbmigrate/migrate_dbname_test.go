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

// setupTestDBWithName creates a test database with a specific name
func setupTestDBWithName(t *testing.T, backend dbm.BackendType, dbName string, numKeys int) (string, dbm.DB) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	err := os.MkdirAll(dataDir, 0755)
	require.NoError(t, err)

	db, err := dbm.NewDB(dbName, backend, dataDir)
	require.NoError(t, err)

	// Populate with test data
	for i := 0; i < numKeys; i++ {
		key := []byte(fmt.Sprintf("key-%s-%06d", dbName, i))
		value := []byte(fmt.Sprintf("value-%s-%06d-data", dbName, i))
		err := db.Set(key, value)
		require.NoError(t, err)
	}

	return tempDir, db
}

// TestMigrateWithDBName tests migration with specific database names
func TestMigrateWithDBName(t *testing.T) {
	dbNames := []string{"application", "blockstore", "state", "tx_index", "evidence"}

	for _, dbName := range dbNames {
		t.Run(dbName, func(t *testing.T) {
			numKeys := 50

			// Setup source database with specific name
			sourceDir, sourceDB := setupTestDBWithName(t, dbm.GoLevelDBBackend, dbName, numKeys)
			sourceDB.Close()

			// Create target directory
			targetDir := t.TempDir()

			// Perform migration with explicit DBName
			opts := MigrateOptions{
				SourceHome:    sourceDir,
				TargetHome:    targetDir,
				SourceBackend: dbm.GoLevelDBBackend,
				TargetBackend: dbm.GoLevelDBBackend,
				BatchSize:     10,
				Logger:        log.NewNopLogger(),
				Verify:        true,
				DBName:        dbName,
			}

			stats, err := Migrate(opts)
			require.NoError(t, err)
			require.NotNil(t, stats)
			require.Equal(t, int64(numKeys), stats.TotalKeys.Load())
			require.Equal(t, int64(numKeys), stats.ProcessedKeys.Load())
			require.Equal(t, int64(0), stats.ErrorCount.Load())

			// Verify duration is positive
			require.Greater(t, stats.Duration().Milliseconds(), int64(0))
		})
	}
}

// TestMigrateMultipleDatabases tests migrating multiple databases sequentially
func TestMigrateMultipleDatabases(t *testing.T) {
	dbNames := []string{"blockstore", "tx_index"}
	numKeys := 100

	// Setup source databases
	sourceDir := t.TempDir()
	dataDir := filepath.Join(sourceDir, "data")
	err := os.MkdirAll(dataDir, 0755)
	require.NoError(t, err)

	// Create multiple source databases
	for _, dbName := range dbNames {
		db, err := dbm.NewDB(dbName, dbm.GoLevelDBBackend, dataDir)
		require.NoError(t, err)

		// Populate with test data
		for i := 0; i < numKeys; i++ {
			key := []byte(fmt.Sprintf("key-%s-%06d", dbName, i))
			value := []byte(fmt.Sprintf("value-%s-%06d", dbName, i))
			err := db.Set(key, value)
			require.NoError(t, err)
		}
		db.Close()
	}

	// Create target directory
	targetDir := t.TempDir()

	// Migrate each database
	var totalProcessed int64
	for _, dbName := range dbNames {
		t.Run("migrate_"+dbName, func(t *testing.T) {
			opts := MigrateOptions{
				SourceHome:    sourceDir,
				TargetHome:    targetDir,
				SourceBackend: dbm.GoLevelDBBackend,
				TargetBackend: dbm.GoLevelDBBackend,
				BatchSize:     20,
				Logger:        log.NewTestLogger(t),
				Verify:        true,
				DBName:        dbName,
			}

			stats, err := Migrate(opts)
			require.NoError(t, err)
			require.NotNil(t, stats)
			require.Equal(t, int64(numKeys), stats.TotalKeys.Load())
			require.Equal(t, int64(numKeys), stats.ProcessedKeys.Load())
			require.Equal(t, int64(0), stats.ErrorCount.Load())

			totalProcessed += stats.ProcessedKeys.Load()
		})
	}

	// Verify total keys migrated
	expectedTotal := int64(numKeys * len(dbNames))
	require.Equal(t, expectedTotal, totalProcessed)
}

// TestMigrateWithDefaultDBName tests that migration defaults to "application" when DBName is not set
func TestMigrateWithDefaultDBName(t *testing.T) {
	numKeys := 50

	// Setup source database with "application" name
	sourceDir, sourceDB := setupTestDBWithName(t, dbm.GoLevelDBBackend, "application", numKeys)
	sourceDB.Close()

	// Create target directory
	targetDir := t.TempDir()

	// Perform migration without specifying DBName (should default to "application")
	opts := MigrateOptions{
		SourceHome:    sourceDir,
		TargetHome:    targetDir,
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.GoLevelDBBackend,
		BatchSize:     10,
		Logger:        log.NewNopLogger(),
		Verify:        true,
		// DBName is intentionally not set
	}

	stats, err := Migrate(opts)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, int64(numKeys), stats.TotalKeys.Load())
	require.Equal(t, int64(numKeys), stats.ProcessedKeys.Load())
	require.Equal(t, int64(0), stats.ErrorCount.Load())
}

// TestMigrateCometBFTDatabases tests migrating all CometBFT databases
func TestMigrateCometBFTDatabases(t *testing.T) {
	cometbftDBs := []string{"blockstore", "state", "tx_index", "evidence"}
	numKeys := 25

	// Setup source databases
	sourceDir := t.TempDir()
	dataDir := filepath.Join(sourceDir, "data")
	err := os.MkdirAll(dataDir, 0755)
	require.NoError(t, err)

	// Create CometBFT databases
	for _, dbName := range cometbftDBs {
		db, err := dbm.NewDB(dbName, dbm.GoLevelDBBackend, dataDir)
		require.NoError(t, err)

		// Add some data specific to each database
		for i := 0; i < numKeys; i++ {
			key := []byte(fmt.Sprintf("%s-key-%d", dbName, i))
			value := []byte(fmt.Sprintf("%s-value-%d", dbName, i))
			err := db.Set(key, value)
			require.NoError(t, err)
		}
		db.Close()
	}

	// Create target directory
	targetDir := t.TempDir()

	// Migrate each CometBFT database
	for _, dbName := range cometbftDBs {
		t.Run(dbName, func(t *testing.T) {
			opts := MigrateOptions{
				SourceHome:    sourceDir,
				TargetHome:    targetDir,
				SourceBackend: dbm.GoLevelDBBackend,
				TargetBackend: dbm.MemDBBackend,
				BatchSize:     10,
				Logger:        log.NewNopLogger(),
				Verify:        false, // MemDB verification is skipped
				DBName:        dbName,
			}

			stats, err := Migrate(opts)
			require.NoError(t, err)
			require.Equal(t, int64(numKeys), stats.TotalKeys.Load())
			require.Equal(t, int64(numKeys), stats.ProcessedKeys.Load())
		})
	}
}

// TestMigrateEmptyDatabaseWithName tests migration of an empty database with a specific name
func TestMigrateEmptyDatabaseWithName(t *testing.T) {
	dbName := "empty_db"

	// Create an empty database
	sourceDir, sourceDB := setupTestDBWithName(t, dbm.GoLevelDBBackend, dbName, 0)
	sourceDB.Close()

	targetDir := t.TempDir()

	opts := MigrateOptions{
		SourceHome:    sourceDir,
		TargetHome:    targetDir,
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.GoLevelDBBackend,
		BatchSize:     10,
		Logger:        log.NewNopLogger(),
		Verify:        false,
		DBName:        dbName,
	}

	stats, err := Migrate(opts)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Equal(t, int64(0), stats.TotalKeys.Load())
	require.Equal(t, int64(0), stats.ProcessedKeys.Load())
}

// TestMigrateDifferentDBNames tests migrating databases with different names to ensure isolation
func TestMigrateDifferentDBNames(t *testing.T) {
	numKeys := 30
	db1Name := "db_one"
	db2Name := "db_two"

	// Setup source directory with two different databases
	sourceDir := t.TempDir()
	dataDir := filepath.Join(sourceDir, "data")
	err := os.MkdirAll(dataDir, 0755)
	require.NoError(t, err)

	// Create first database
	db1, err := dbm.NewDB(db1Name, dbm.GoLevelDBBackend, dataDir)
	require.NoError(t, err)
	for i := 0; i < numKeys; i++ {
		err := db1.Set([]byte(fmt.Sprintf("db1-key-%d", i)), []byte("db1-value"))
		require.NoError(t, err)
	}
	db1.Close()

	// Create second database with different data
	db2, err := dbm.NewDB(db2Name, dbm.GoLevelDBBackend, dataDir)
	require.NoError(t, err)
	for i := 0; i < numKeys*2; i++ { // Different number of keys
		err := db2.Set([]byte(fmt.Sprintf("db2-key-%d", i)), []byte("db2-value"))
		require.NoError(t, err)
	}
	db2.Close()

	targetDir := t.TempDir()

	// Migrate first database
	opts1 := MigrateOptions{
		SourceHome:    sourceDir,
		TargetHome:    targetDir,
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.GoLevelDBBackend,
		BatchSize:     10,
		Logger:        log.NewNopLogger(),
		Verify:        false,
		DBName:        db1Name,
	}

	stats1, err := Migrate(opts1)
	require.NoError(t, err)
	require.Equal(t, int64(numKeys), stats1.TotalKeys.Load())

	// Migrate second database
	opts2 := MigrateOptions{
		SourceHome:    sourceDir,
		TargetHome:    targetDir,
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.GoLevelDBBackend,
		BatchSize:     10,
		Logger:        log.NewNopLogger(),
		Verify:        false,
		DBName:        db2Name,
	}

	stats2, err := Migrate(opts2)
	require.NoError(t, err)
	require.Equal(t, int64(numKeys*2), stats2.TotalKeys.Load())

	// Verify both databases were migrated separately
	require.NotEqual(t, stats1.TotalKeys.Load(), stats2.TotalKeys.Load(), "databases should have different key counts")
}

// TestMigrateDBNameWithSpecialCharacters tests database names with underscores
func TestMigrateDBNameWithSpecialCharacters(t *testing.T) {
	dbName := "tx_index" // Contains underscore
	numKeys := 40

	sourceDir, sourceDB := setupTestDBWithName(t, dbm.GoLevelDBBackend, dbName, numKeys)
	sourceDB.Close()

	targetDir := t.TempDir()

	opts := MigrateOptions{
		SourceHome:    sourceDir,
		TargetHome:    targetDir,
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.MemDBBackend,
		BatchSize:     15,
		Logger:        log.NewNopLogger(),
		Verify:        false,
		DBName:        dbName,
	}

	stats, err := Migrate(opts)
	require.NoError(t, err)
	require.Equal(t, int64(numKeys), stats.TotalKeys.Load())
	require.Equal(t, int64(numKeys), stats.ProcessedKeys.Load())
}

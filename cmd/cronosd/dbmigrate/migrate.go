package dbmigrate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	dbm "github.com/cosmos/cosmos-db"

	"cosmossdk.io/log"
)

const (
	// DefaultBatchSize is the number of key-value pairs to process in a single batch
	DefaultBatchSize = 10000
	// DefaultWorkers is the number of concurrent workers for migration
	DefaultWorkers = 4
)

// MigrateOptions holds the configuration for database migration
type MigrateOptions struct {
	// SourceHome is the home directory containing the source database
	SourceHome string
	// TargetHome is the home directory for the target database (if empty, uses SourceHome)
	TargetHome string
	// SourceBackend is the source database backend type
	SourceBackend dbm.BackendType
	// TargetBackend is the target database backend type
	TargetBackend dbm.BackendType
	// BatchSize is the number of key-value pairs to process in a single batch
	BatchSize int
	// Logger for progress reporting
	Logger log.Logger
	// RocksDBOptions for creating RocksDB (only used when target is RocksDB)
	// This is interface{} to avoid importing grocksdb when rocksdb tag is not used
	RocksDBOptions interface{}
	// Verify enables post-migration verification
	Verify bool
	// DBName is the name of the database to migrate (e.g., "application", "blockstore", "state")
	DBName string
	// HeightRange specifies the range of heights to migrate (only for blockstore and tx_index)
	HeightRange HeightRange
}

// MigrationStats tracks migration progress and statistics
type MigrationStats struct {
	TotalKeys     atomic.Int64
	ProcessedKeys atomic.Int64
	ErrorCount    atomic.Int64
	StartTime     time.Time
	EndTime       time.Time
}

// Progress returns the current progress as a percentage
func (s *MigrationStats) Progress() float64 {
	total := s.TotalKeys.Load()
	if total == 0 {
		return 0
	}
	return float64(s.ProcessedKeys.Load()) / float64(total) * 100
}

// Duration returns the time elapsed since start
func (s *MigrationStats) Duration() time.Duration {
	if s.EndTime.IsZero() {
		return time.Since(s.StartTime)
	}
	return s.EndTime.Sub(s.StartTime)
}

// Migrate performs database migration from source backend to target backend
func Migrate(opts MigrateOptions) (*MigrationStats, error) {
	if opts.BatchSize <= 0 {
		opts.BatchSize = DefaultBatchSize
	}
	if opts.TargetHome == "" {
		opts.TargetHome = opts.SourceHome
	}
	if opts.Logger == nil {
		opts.Logger = log.NewNopLogger()
	}

	stats := &MigrationStats{
		StartTime: time.Now(),
	}

	// Default to "application" if DBName is not specified
	if opts.DBName == "" {
		opts.DBName = "application"
	}

	// Validate height range if specified
	if err := opts.HeightRange.Validate(); err != nil {
		return stats, fmt.Errorf("invalid height range: %w", err)
	}

	logArgs := []interface{}{
		"database", opts.DBName,
		"source_backend", opts.SourceBackend,
		"target_backend", opts.TargetBackend,
		"source_home", opts.SourceHome,
		"target_home", opts.TargetHome,
	}

	// Add height range to log if specified
	if !opts.HeightRange.IsEmpty() {
		logArgs = append(logArgs, "height_range", opts.HeightRange.String())
	}

	opts.Logger.Info("Starting database migration", logArgs...)

	// Open source database in read-only mode
	sourceDataDir := filepath.Join(opts.SourceHome, "data")
	sourceDB, err := dbm.NewDB(opts.DBName, opts.SourceBackend, sourceDataDir)
	if err != nil {
		return stats, fmt.Errorf("failed to open source database: %w", err)
	}
	sourceDBClosed := false
	defer func() {
		if !sourceDBClosed {
			sourceDB.Close()
		}
	}()

	// Create target database
	targetDataDir := filepath.Join(opts.TargetHome, "data")

	// For migration, we need to ensure we don't accidentally overwrite an existing DB
	// Unified path format for all backends: <dbName>.migrate-temp.db
	tempTargetDir := filepath.Join(targetDataDir, opts.DBName+".migrate-temp.db")
	finalTargetDir := filepath.Join(targetDataDir, opts.DBName+".db")

	var targetDB dbm.DB
	if opts.TargetBackend == dbm.RocksDBBackend {
		// RocksDB: we specify the exact directory path
		// RocksDB needs the parent directory to exist
		if err := os.MkdirAll(targetDataDir, 0755); err != nil {
			return stats, fmt.Errorf("failed to create target data directory: %w", err)
		}
		targetDB, err = openRocksDBForMigration(tempTargetDir, opts.RocksDBOptions)
	} else {
		// LevelDB/others: dbm.NewDB appends .db to the name, so we pass the name without .db
		targetDB, err = dbm.NewDB(opts.DBName+".migrate-temp", opts.TargetBackend, targetDataDir)
	}
	if err != nil {
		return stats, fmt.Errorf("failed to create target database: %w", err)
	}
	targetDBClosed := false
	defer func() {
		if !targetDBClosed {
			targetDB.Close()
		}
	}()

	// Count total keys first for progress reporting
	opts.Logger.Info("Counting total keys...")
	var totalKeys int64

	// Use height-filtered counting if height range is specified
	if !opts.HeightRange.IsEmpty() && supportsHeightFiltering(opts.DBName) {
		totalKeys, err = countKeysWithHeightFilter(sourceDB, opts.DBName, opts.HeightRange)
		if err != nil {
			return stats, fmt.Errorf("failed to count keys with height filter: %w", err)
		}
		opts.Logger.Info("Total keys to migrate", "count", totalKeys, "height_range", opts.HeightRange.String())
	} else {
		if !opts.HeightRange.IsEmpty() {
			opts.Logger.Warn("Height filtering not supported for this database, migrating all keys", "database", opts.DBName)
		}

		totalKeys, err = countKeys(sourceDB)
		if err != nil {
			return stats, fmt.Errorf("failed to count keys: %w", err)
		}
		opts.Logger.Info("Total keys to migrate", "count", totalKeys)
	}

	stats.TotalKeys.Store(totalKeys)

	// Perform the migration
	// Use height-filtered migration if height range is specified and database supports it
	if !opts.HeightRange.IsEmpty() && supportsHeightFiltering(opts.DBName) {
		if err := migrateDataWithHeightFilter(sourceDB, targetDB, opts, stats); err != nil {
			return stats, fmt.Errorf("migration failed: %w", err)
		}
	} else {
		if err := migrateData(sourceDB, targetDB, opts, stats); err != nil {
			return stats, fmt.Errorf("migration failed: %w", err)
		}
	}

	// Flush memtable to SST files for RocksDB
	if opts.TargetBackend == dbm.RocksDBBackend {
		opts.Logger.Info("Flushing RocksDB memtable to SST files...")
		if err := flushRocksDB(targetDB); err != nil {
			return stats, fmt.Errorf("failed to flush RocksDB: %w", err)
		}
		opts.Logger.Info("Flush completed")
	}

	// Close databases before verification to release locks
	// This prevents "resource temporarily unavailable" errors
	if err := targetDB.Close(); err != nil {
		opts.Logger.Error("Warning: failed to close target database", "error", err)
	}
	targetDBClosed = true

	if err := sourceDB.Close(); err != nil {
		opts.Logger.Error("Warning: failed to close source database", "error", err)
	}
	sourceDBClosed = true

	stats.EndTime = time.Now()
	opts.Logger.Info("Migration completed",
		"total_keys", stats.TotalKeys.Load(),
		"processed_keys", stats.ProcessedKeys.Load(),
		"errors", stats.ErrorCount.Load(),
		"duration", stats.Duration(),
	)

	// Verification step if requested
	if opts.Verify {
		opts.Logger.Info("Starting verification...")
		if err := verifyMigration(sourceDataDir, tempTargetDir, opts); err != nil {
			return stats, fmt.Errorf("verification failed: %w", err)
		}
		opts.Logger.Info("Verification completed successfully")
	}

	opts.Logger.Info("Migration process completed",
		"temp_location", tempTargetDir,
		"target_location", finalTargetDir,
		"note", "Please backup your source database and manually rename the temp directory to replace the original",
	)

	return stats, nil
}

// countKeys counts the total number of keys in the database
func countKeys(db dbm.DB) (int64, error) {
	itr, err := db.Iterator(nil, nil)
	if err != nil {
		return 0, err
	}
	defer itr.Close()

	var count int64
	for ; itr.Valid(); itr.Next() {
		count++
	}
	return count, itr.Error()
}

// countKeysWithHeightFilter counts keys using bounded iterators for the specified height range
func countKeysWithHeightFilter(db dbm.DB, dbName string, heightRange HeightRange) (int64, error) {
	var iterators []dbm.Iterator
	var err error

	// Get bounded iterators based on database type
	switch dbName {
	case DBNameBlockstore:
		iterators, err = getBlockstoreIterators(db, heightRange)
	case DBNameTxIndex:
		itr, err := getTxIndexIterator(db, heightRange)
		if err != nil {
			return 0, err
		}
		iterators = []dbm.Iterator{itr}
	default:
		// Fall back to full counting for unsupported databases
		return countKeys(db)
	}

	if err != nil {
		return 0, err
	}

	// Ensure all iterators are closed
	defer func() {
		for _, itr := range iterators {
			itr.Close()
		}
	}()

	// Count keys from each iterator, applying height filter
	var count int64
	for _, itr := range iterators {
		for ; itr.Valid(); itr.Next() {
			key := itr.Key()
			// Apply shouldIncludeKey filter to handle discrete heights and metadata
			if !shouldIncludeKey(key, dbName, heightRange) {
				continue
			}
			count++
		}
		if err := itr.Error(); err != nil {
			return count, err
		}
	}

	return count, nil
}

// migrateData performs the actual data migration without height filtering
func migrateData(sourceDB, targetDB dbm.DB, opts MigrateOptions, stats *MigrationStats) error {
	itr, err := sourceDB.Iterator(nil, nil)
	if err != nil {
		return err
	}
	defer itr.Close()

	return migrateWithIterator(itr, targetDB, opts, stats)
}

// migrateDataWithHeightFilter performs data migration using bounded iterators for height filtering
func migrateDataWithHeightFilter(sourceDB, targetDB dbm.DB, opts MigrateOptions, stats *MigrationStats) error {
	var iterators []dbm.Iterator
	var err error

	// Get bounded iterators based on database type
	switch opts.DBName {
	case DBNameBlockstore:
		iterators, err = getBlockstoreIterators(sourceDB, opts.HeightRange)
	case DBNameTxIndex:
		itr, err := getTxIndexIterator(sourceDB, opts.HeightRange)
		if err != nil {
			return err
		}
		iterators = []dbm.Iterator{itr}
	default:
		// Fall back to full migration for unsupported databases
		return migrateData(sourceDB, targetDB, opts, stats)
	}

	if err != nil {
		return fmt.Errorf("failed to create height-filtered iterators: %w", err)
	}

	// Ensure all iterators are closed
	defer func() {
		for _, itr := range iterators {
			itr.Close()
		}
	}()

	// Migrate data from each iterator
	for _, itr := range iterators {
		if err := migrateWithIterator(itr, targetDB, opts, stats); err != nil {
			return err
		}
	}

	opts.Logger.Info("Height-filtered migration completed",
		"height_range", opts.HeightRange.String(),
		"migrated_keys", stats.ProcessedKeys.Load(),
	)

	return nil
}

// migrateWithIterator migrates data from a single iterator
func migrateWithIterator(itr dbm.Iterator, targetDB dbm.DB, opts MigrateOptions, stats *MigrationStats) error {
	batch := targetDB.NewBatch()
	defer batch.Close()

	batchCount := 0
	lastProgressReport := time.Now()

	for ; itr.Valid(); itr.Next() {
		key := itr.Key()

		// Apply shouldIncludeKey filter for all height-filtered migrations
		// This handles discrete heights, metadata keys, and ensures we only migrate requested data
		if !shouldIncludeKey(key, opts.DBName, opts.HeightRange) {
			continue
		}
		value := itr.Value()

		// Make copies since the iterator might reuse the slices
		keyCopy := make([]byte, len(key))
		valueCopy := make([]byte, len(value))
		copy(keyCopy, key)
		copy(valueCopy, value)

		if err := batch.Set(keyCopy, valueCopy); err != nil {
			opts.Logger.Error("Failed to add key to batch", "error", err)
			stats.ErrorCount.Add(1)
			continue
		}

		batchCount++
		stats.ProcessedKeys.Add(1)

		// Write batch when it reaches the configured size
		if batchCount >= opts.BatchSize {
			if err := batch.Write(); err != nil {
				return fmt.Errorf("failed to write batch: %w", err)
			}
			batch.Close()
			batch = targetDB.NewBatch()
			batchCount = 0
		}

		// Report progress every second
		if time.Since(lastProgressReport) >= time.Second {
			opts.Logger.Info("Migration progress",
				"progress", fmt.Sprintf("%.2f%%", stats.Progress()),
				"processed", stats.ProcessedKeys.Load(),
				"total", stats.TotalKeys.Load(),
				"errors", stats.ErrorCount.Load(),
			)
			lastProgressReport = time.Now()
		}
	}

	// Write any remaining items in the batch
	if batchCount > 0 {
		if err := batch.Write(); err != nil {
			return fmt.Errorf("failed to write final batch: %w", err)
		}
	}

	return itr.Error()
}

// openDBWithRetry attempts to open a database with exponential backoff retry logic.
// This handles OS-level file lock delays that can occur after database closure.
func openDBWithRetry(dbName string, backend dbm.BackendType, dir string, maxRetries int, initialDelay time.Duration, logger log.Logger) (dbm.DB, error) {
	var db dbm.DB
	var err error
	delay := initialDelay

	for attempt := 0; attempt < maxRetries; attempt++ {
		db, err = dbm.NewDB(dbName, backend, dir)
		if err == nil {
			return db, nil
		}

		if attempt < maxRetries-1 {
			logger.Info("Failed to open database, retrying...",
				"attempt", attempt+1,
				"max_retries", maxRetries,
				"delay", delay,
				"error", err,
			)
			time.Sleep(delay)
			delay *= 2 // Exponential backoff
		}
	}

	return nil, fmt.Errorf("failed to open database after %d attempts: %w", maxRetries, err)
}

// openRocksDBWithRetry attempts to open a RocksDB database with exponential backoff retry logic.
func openRocksDBWithRetry(dir string, maxRetries int, initialDelay time.Duration, logger log.Logger) (dbm.DB, error) {
	var db dbm.DB
	var err error
	delay := initialDelay

	for attempt := 0; attempt < maxRetries; attempt++ {
		db, err = openRocksDBForRead(dir)
		if err == nil {
			return db, nil
		}

		if attempt < maxRetries-1 {
			logger.Info("Failed to open RocksDB, retrying...",
				"attempt", attempt+1,
				"max_retries", maxRetries,
				"delay", delay,
				"error", err,
			)
			time.Sleep(delay)
			delay *= 2 // Exponential backoff
		}
	}

	return nil, fmt.Errorf("failed to open RocksDB after %d attempts: %w", maxRetries, err)
}

// verifyMigration compares source and target databases to ensure data integrity
func verifyMigration(sourceDir, targetDir string, opts MigrateOptions) error {
	// Determine database name from the directory path
	// Extract the database name from sourceDir (e.g., "blockstore" from "/path/to/blockstore.db")
	dbName := opts.DBName
	if dbName == "" {
		dbName = "application"
	}

	// Reopen databases for verification with retry logic to handle OS-level file lock delays
	// that can occur after database closure. Use exponential backoff: 50ms, 100ms, 200ms, 400ms, 800ms
	const maxRetries = 5
	const initialDelay = 50 * time.Millisecond

	sourceDB, err := openDBWithRetry(dbName, opts.SourceBackend, sourceDir, maxRetries, initialDelay, opts.Logger)
	if err != nil {
		return fmt.Errorf("failed to open source database for verification: %w", err)
	}
	defer sourceDB.Close()

	var targetDB dbm.DB
	if opts.TargetBackend == dbm.RocksDBBackend {
		targetDB, err = openRocksDBWithRetry(targetDir, maxRetries, initialDelay, opts.Logger)
	} else {
		targetDB, err = openDBWithRetry(dbName+".migrate-temp", opts.TargetBackend, filepath.Dir(targetDir), maxRetries, initialDelay, opts.Logger)
	}
	if err != nil {
		return fmt.Errorf("failed to open target database for verification: %w", err)
	}
	defer targetDB.Close()

	// Check if we need height-filtered verification
	useHeightFilter := !opts.HeightRange.IsEmpty() && supportsHeightFiltering(dbName)

	if useHeightFilter {
		opts.Logger.Info("Using height-filtered verification", "height_range", opts.HeightRange.String())
	}

	var verifiedKeys int64
	var mismatchCount int64
	lastProgressReport := time.Now()

	// Phase 1: Verify all keys that should be in target exist and match
	if useHeightFilter {
		// Use filtered iterators for height-based verification
		var sourceIterators []dbm.Iterator
		switch dbName {
		case DBNameBlockstore:
			sourceIterators, err = getBlockstoreIterators(sourceDB, opts.HeightRange)
		case DBNameTxIndex:
			itr, err := getTxIndexIterator(sourceDB, opts.HeightRange)
			if err != nil {
				return fmt.Errorf("failed to get tx_index iterator: %w", err)
			}
			sourceIterators = []dbm.Iterator{itr}
		default:
			return fmt.Errorf("height filtering not supported for database: %s", dbName)
		}
		if err != nil {
			return fmt.Errorf("failed to get filtered iterators: %w", err)
		}
		defer func() {
			for _, itr := range sourceIterators {
				itr.Close()
			}
		}()

		// Verify using filtered iterators
		for _, sourceItr := range sourceIterators {
			for ; sourceItr.Valid(); sourceItr.Next() {
				key := sourceItr.Key()

				// Apply shouldIncludeKey filter to handle discrete heights and metadata
				if !shouldIncludeKey(key, dbName, opts.HeightRange) {
					continue
				}

				sourceValue := sourceItr.Value()

				targetValue, err := targetDB.Get(key)
				if err != nil {
					opts.Logger.Error("Failed to get key from target database", "key", fmt.Sprintf("%x", key), "error", err)
					mismatchCount++
					continue
				}

				if targetValue == nil {
					opts.Logger.Error("Key missing in target database", "key", fmt.Sprintf("%x", key))
					mismatchCount++
					continue
				}

				// Use bytes.Equal for efficient comparison
				if !bytes.Equal(sourceValue, targetValue) {
					opts.Logger.Error("Value mismatch",
						"key", fmt.Sprintf("%x", key),
						"source_len", len(sourceValue),
						"target_len", len(targetValue),
					)
					mismatchCount++
				}

				verifiedKeys++

				// Report progress every second
				if time.Since(lastProgressReport) >= time.Second {
					opts.Logger.Info("Verification progress",
						"verified", verifiedKeys,
						"mismatches", mismatchCount,
					)
					lastProgressReport = time.Now()
				}
			}
			if err := sourceItr.Error(); err != nil {
				return err
			}
		}
	} else {
		// Full database verification (no height filtering)
		sourceItr, err := sourceDB.Iterator(nil, nil)
		if err != nil {
			return err
		}
		defer sourceItr.Close()

		for ; sourceItr.Valid(); sourceItr.Next() {
			key := sourceItr.Key()
			sourceValue := sourceItr.Value()

			targetValue, err := targetDB.Get(key)
			if err != nil {
				opts.Logger.Error("Failed to get key from target database", "key", fmt.Sprintf("%x", key), "error", err)
				mismatchCount++
				continue
			}

			if targetValue == nil {
				opts.Logger.Error("Key missing in target database", "key", fmt.Sprintf("%x", key))
				mismatchCount++
				continue
			}

			// Use bytes.Equal for efficient comparison
			if !bytes.Equal(sourceValue, targetValue) {
				opts.Logger.Error("Value mismatch",
					"key", fmt.Sprintf("%x", key),
					"source_len", len(sourceValue),
					"target_len", len(targetValue),
				)
				mismatchCount++
			}

			verifiedKeys++

			// Report progress every second
			if time.Since(lastProgressReport) >= time.Second {
				opts.Logger.Info("Verification progress",
					"verified", verifiedKeys,
					"mismatches", mismatchCount,
				)
				lastProgressReport = time.Now()
			}
		}

		if err := sourceItr.Error(); err != nil {
			return err
		}
	}

	// Phase 2: Verify target doesn't have extra keys (iterate target, check against source)
	opts.Logger.Info("Starting second verification phase (checking for extra keys in target)...")
	targetItr, err := targetDB.Iterator(nil, nil)
	if err != nil {
		return fmt.Errorf("failed to create target iterator: %w", err)
	}
	defer targetItr.Close()

	var targetKeys int64
	lastProgressReport = time.Now()

	for ; targetItr.Valid(); targetItr.Next() {
		key := targetItr.Key()

		// If using height filter, skip keys that shouldn't have been migrated
		if useHeightFilter {
			if !shouldIncludeKey(key, dbName, opts.HeightRange) {
				continue
			}
		}

		targetKeys++

		// Check if this key exists in source
		sourceValue, err := sourceDB.Get(key)
		if err != nil {
			opts.Logger.Error("Failed to get key from source database during reverse verification",
				"key", fmt.Sprintf("%x", key),
				"error", err,
			)
			mismatchCount++
			continue
		}

		// If key doesn't exist in source (Get returns nil for non-existent keys)
		if sourceValue == nil {
			opts.Logger.Error("Extra key found in target that doesn't exist in source",
				"key", fmt.Sprintf("%x", key),
			)
			mismatchCount++
		}

		// Report progress every second
		if time.Since(lastProgressReport) >= time.Second {
			opts.Logger.Info("Reverse verification progress",
				"target_keys_checked", targetKeys,
				"mismatches", mismatchCount,
			)
			lastProgressReport = time.Now()
		}
	}

	if err := targetItr.Error(); err != nil {
		return fmt.Errorf("error during target iteration: %w", err)
	}

	// Compare key counts
	if targetKeys != verifiedKeys {
		opts.Logger.Error("Key count mismatch",
			"source_keys", verifiedKeys,
			"target_keys", targetKeys,
			"difference", targetKeys-verifiedKeys,
		)
		mismatchCount++
	}

	if mismatchCount > 0 {
		return fmt.Errorf("verification failed: %d mismatches found", mismatchCount)
	}

	opts.Logger.Info("Verification summary",
		"verified_keys", verifiedKeys,
		"target_keys", targetKeys,
		"mismatches", mismatchCount,
	)

	return nil
}

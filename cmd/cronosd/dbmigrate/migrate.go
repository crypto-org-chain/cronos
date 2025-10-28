package dbmigrate

import (
	"fmt"
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

	opts.Logger.Info("Starting database migration",
		"source_backend", opts.SourceBackend,
		"target_backend", opts.TargetBackend,
		"source_home", opts.SourceHome,
		"target_home", opts.TargetHome,
	)

	// Open source database in read-only mode
	sourceDataDir := filepath.Join(opts.SourceHome, "data")
	sourceDB, err := dbm.NewDB("application", opts.SourceBackend, sourceDataDir)
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
	// We'll create a temporary directory first
	tempTargetDir := filepath.Join(targetDataDir, "application.db.migrate-temp")
	finalTargetDir := filepath.Join(targetDataDir, "application.db")

	var targetDB dbm.DB
	if opts.TargetBackend == dbm.RocksDBBackend {
		targetDB, err = openRocksDBForMigration(tempTargetDir, opts.RocksDBOptions)
	} else {
		targetDB, err = dbm.NewDB("application.migrate-temp", opts.TargetBackend, targetDataDir)
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
	totalKeys, err := countKeys(sourceDB)
	if err != nil {
		return stats, fmt.Errorf("failed to count keys: %w", err)
	}
	stats.TotalKeys.Store(totalKeys)
	opts.Logger.Info("Total keys to migrate", "count", totalKeys)

	// Perform the migration
	if err := migrateData(sourceDB, targetDB, opts, stats); err != nil {
		return stats, fmt.Errorf("migration failed: %w", err)
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

// migrateData performs the actual data migration
func migrateData(sourceDB, targetDB dbm.DB, opts MigrateOptions, stats *MigrationStats) error {
	itr, err := sourceDB.Iterator(nil, nil)
	if err != nil {
		return err
	}
	defer itr.Close()

	batch := targetDB.NewBatch()
	defer batch.Close()

	batchCount := 0
	lastProgressReport := time.Now()

	for ; itr.Valid(); itr.Next() {
		key := itr.Key()
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

// verifyMigration compares source and target databases to ensure data integrity
func verifyMigration(sourceDir, targetDir string, opts MigrateOptions) error {
	// Reopen databases for verification
	sourceDB, err := dbm.NewDB("application", opts.SourceBackend, sourceDir)
	if err != nil {
		return fmt.Errorf("failed to open source database for verification: %w", err)
	}
	defer sourceDB.Close()

	var targetDB dbm.DB
	if opts.TargetBackend == dbm.RocksDBBackend {
		targetDB, err = openRocksDBForRead(targetDir)
	} else {
		targetDB, err = dbm.NewDB("application.migrate-temp", opts.TargetBackend, filepath.Dir(targetDir))
	}
	if err != nil {
		return fmt.Errorf("failed to open target database for verification: %w", err)
	}
	defer targetDB.Close()

	// Iterate through source and compare with target
	sourceItr, err := sourceDB.Iterator(nil, nil)
	if err != nil {
		return err
	}
	defer sourceItr.Close()

	var verifiedKeys int64
	var mismatchCount int64
	lastProgressReport := time.Now()

	for ; sourceItr.Valid(); sourceItr.Next() {
		key := sourceItr.Key()
		sourceValue := sourceItr.Value()

		targetValue, err := targetDB.Get(key)
		if err != nil {
			opts.Logger.Error("Failed to get key from target database", "key", fmt.Sprintf("%x", key), "error", err)
			mismatchCount++
			continue
		}

		if len(targetValue) != len(sourceValue) {
			opts.Logger.Error("Value length mismatch",
				"key", fmt.Sprintf("%x", key),
				"source_len", len(sourceValue),
				"target_len", len(targetValue),
			)
			mismatchCount++
			continue
		}

		// Compare byte by byte
		match := true
		for i := range sourceValue {
			if sourceValue[i] != targetValue[i] {
				match = false
				break
			}
		}

		if !match {
			opts.Logger.Error("Value mismatch", "key", fmt.Sprintf("%x", key))
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

	if mismatchCount > 0 {
		return fmt.Errorf("verification failed: %d mismatches found", mismatchCount)
	}

	opts.Logger.Info("Verification summary",
		"verified_keys", verifiedKeys,
		"mismatches", mismatchCount,
	)

	return nil
}

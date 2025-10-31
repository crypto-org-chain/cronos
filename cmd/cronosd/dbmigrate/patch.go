package dbmigrate

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tmstore "github.com/cometbft/cometbft/proto/tendermint/store"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/gogoproto/proto"

	"cosmossdk.io/log"
)

// PatchOptions contains options for patching databases
type PatchOptions struct {
	SourceHome     string          // Source home directory
	TargetPath     string          // Target database path (exact path to patch)
	SourceBackend  dbm.BackendType // Source backend type
	TargetBackend  dbm.BackendType // Target backend type
	BatchSize      int             // Batch size for writing
	Logger         log.Logger      // Logger
	RocksDBOptions interface{}     // RocksDB specific options
	DBName         string          // Database name (blockstore, tx_index, etc.)
	HeightRange    HeightRange     // Height range/specific heights to patch
}

// PatchDatabase patches specific heights from source to target database
func PatchDatabase(opts PatchOptions) (*MigrationStats, error) {
	if opts.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	if opts.HeightRange.IsEmpty() {
		return nil, fmt.Errorf("height range is required for patching")
	}

	if !supportsHeightFiltering(opts.DBName) {
		return nil, fmt.Errorf("database %s does not support height-based patching (only blockstore and tx_index supported)", opts.DBName)
	}

	logger := opts.Logger
	stats := &MigrationStats{
		StartTime: time.Now(),
	}

	// Construct source database path
	sourceDBPath := filepath.Join(opts.SourceHome, "data", opts.DBName+".db")

	// Validate source exists
	if _, err := os.Stat(sourceDBPath); os.IsNotExist(err) {
		return stats, fmt.Errorf("source database does not exist: %s", sourceDBPath)
	}

	// Validate target exists
	if _, err := os.Stat(opts.TargetPath); os.IsNotExist(err) {
		return stats, fmt.Errorf("target database does not exist: %s (use migrate-db to create new databases)", opts.TargetPath)
	}

	logger.Info("Opening databases for patching",
		"source_db", sourceDBPath,
		"source_backend", opts.SourceBackend,
		"target_db", opts.TargetPath,
		"target_backend", opts.TargetBackend,
		"height_range", opts.HeightRange.String(),
	)

	// Open source database (read-only)
	sourceDir := filepath.Dir(sourceDBPath)
	sourceName := filepath.Base(sourceDBPath)
	if len(sourceName) > 3 && sourceName[len(sourceName)-3:] == ".db" {
		sourceName = sourceName[:len(sourceName)-3]
	}

	sourceDB, err := dbm.NewDB(sourceName, opts.SourceBackend, sourceDir)
	if err != nil {
		return stats, fmt.Errorf("failed to open source database: %w", err)
	}
	defer sourceDB.Close()

	// Open target database (read-write for patching)
	var targetDB dbm.DB
	if opts.TargetBackend == dbm.RocksDBBackend {
		targetDB, err = openRocksDBForMigration(opts.TargetPath, opts.RocksDBOptions)
	} else {
		targetDir := filepath.Dir(opts.TargetPath)
		targetName := filepath.Base(opts.TargetPath)
		if len(targetName) > 3 && targetName[len(targetName)-3:] == ".db" {
			targetName = targetName[:len(targetName)-3]
		}
		targetDB, err = dbm.NewDB(targetName, opts.TargetBackend, targetDir)
	}
	if err != nil {
		return stats, fmt.Errorf("failed to open target database: %w", err)
	}
	defer targetDB.Close()

	// Count keys to patch
	totalKeys, err := countKeysForPatch(sourceDB, opts.DBName, opts.HeightRange, logger)
	if err != nil {
		return stats, fmt.Errorf("failed to count keys: %w", err)
	}
	stats.TotalKeys.Store(totalKeys)

	if totalKeys == 0 {
		logger.Info("No keys found in source database for specified heights",
			"database", opts.DBName,
			"height_range", opts.HeightRange.String(),
		)
		return stats, nil
	}

	logger.Info("Starting database patch",
		"database", opts.DBName,
		"total_keys", totalKeys,
		"height_range", opts.HeightRange.String(),
		"batch_size", opts.BatchSize,
	)

	// Perform the patch operation
	if err := patchDataWithHeightFilter(sourceDB, targetDB, opts, stats); err != nil {
		return stats, fmt.Errorf("failed to patch data: %w", err)
	}

	// Flush RocksDB if needed
	if opts.TargetBackend == dbm.RocksDBBackend {
		if err := flushRocksDB(targetDB); err != nil {
			logger.Info("Failed to flush RocksDB", "error", err)
		}
	}

	stats.EndTime = time.Now()
	return stats, nil
}

// countKeysForPatch counts the number of keys to patch based on height range
func countKeysForPatch(db dbm.DB, dbName string, heightRange HeightRange, logger log.Logger) (int64, error) {
	var totalCount int64

	// If we have specific heights, we need to filter while counting
	needsFiltering := heightRange.HasSpecificHeights()

	switch dbName {
	case "blockstore":
		// For blockstore, count keys from all prefixes
		iterators, err := getBlockstoreIterators(db, heightRange)
		if err != nil {
			return 0, fmt.Errorf("failed to get blockstore iterators: %w", err)
		}

		keysSeen := 0
		for iterIdx, it := range iterators {
			defer it.Close()
			logger.Debug("Counting keys from blockstore iterator", "iterator_index", iterIdx)
			for ; it.Valid(); it.Next() {
				keysSeen++
				// Log first few keys to understand the format
				if keysSeen <= 5 {
					height, hasHeight := extractHeightFromBlockstoreKey(it.Key())
					logger.Debug("Blockstore key found",
						"key_prefix", string(it.Key()[:min(10, len(it.Key()))]),
						"key_hex", fmt.Sprintf("%x", it.Key()[:min(20, len(it.Key()))]),
						"has_height", hasHeight,
						"height", height,
						"in_range", !needsFiltering || (hasHeight && heightRange.IsWithinRange(height)))
				}
				if needsFiltering {
					// Extract height and check if it's in our specific list
					height, hasHeight := extractHeightFromBlockstoreKey(it.Key())
					if hasHeight && !heightRange.IsWithinRange(height) {
						continue
					}
				}
				totalCount++
			}
		}
		logger.Debug("Total keys seen in blockstore", "total_seen", keysSeen, "total_counted", totalCount)

	case "tx_index":
		// For tx_index
		it, err := getTxIndexIterator(db, heightRange)
		if err != nil {
			return 0, fmt.Errorf("failed to get tx_index iterator: %w", err)
		}
		defer it.Close()

		for ; it.Valid(); it.Next() {
			if needsFiltering {
				// Extract height and check if it's in our specific list
				height, hasHeight := extractHeightFromTxIndexKey(it.Key())
				if hasHeight && !heightRange.IsWithinRange(height) {
					continue
				}
			}
			totalCount++
		}

	default:
		return 0, fmt.Errorf("unsupported database for height filtering: %s", dbName)
	}

	return totalCount, nil
}

// patchDataWithHeightFilter patches data using height-filtered iterators
func patchDataWithHeightFilter(sourceDB, targetDB dbm.DB, opts PatchOptions, stats *MigrationStats) error {
	switch opts.DBName {
	case "blockstore":
		return patchBlockstoreData(sourceDB, targetDB, opts, stats)
	case "tx_index":
		return patchTxIndexData(sourceDB, targetDB, opts, stats)
	default:
		return fmt.Errorf("unsupported database for height filtering: %s", opts.DBName)
	}
}

// patchBlockstoreData patches blockstore data
func patchBlockstoreData(sourceDB, targetDB dbm.DB, opts PatchOptions, stats *MigrationStats) error {
	// Get bounded iterators for all blockstore prefixes
	iterators, err := getBlockstoreIterators(sourceDB, opts.HeightRange)
	if err != nil {
		return fmt.Errorf("failed to get blockstore iterators: %w", err)
	}

	opts.Logger.Info("Patching blockstore data",
		"height_range", opts.HeightRange.String(),
		"iterator_count", len(iterators),
	)

	// Process each iterator
	for idx, it := range iterators {
		opts.Logger.Debug("Processing blockstore iterator", "index", idx)
		if err := patchWithIterator(it, targetDB, opts, stats); err != nil {
			return fmt.Errorf("failed to patch with iterator %d: %w", idx, err)
		}
	}

	return nil
}

// patchTxIndexData patches tx_index data
func patchTxIndexData(sourceDB, targetDB dbm.DB, opts PatchOptions, stats *MigrationStats) error {
	// Get bounded iterator for tx_index
	it, err := getTxIndexIterator(sourceDB, opts.HeightRange)
	if err != nil {
		return fmt.Errorf("failed to get tx_index iterator: %w", err)
	}

	opts.Logger.Info("Patching tx_index data",
		"height_range", opts.HeightRange.String(),
	)

	if err := patchWithIterator(it, targetDB, opts, stats); err != nil {
		return fmt.Errorf("failed to patch tx_index data: %w", err)
	}

	return nil
}

// patchWithIterator patches data from an iterator to target database
func patchWithIterator(it dbm.Iterator, targetDB dbm.DB, opts PatchOptions, stats *MigrationStats) error {
	defer it.Close()

	logger := opts.Logger
	batch := targetDB.NewBatch()
	defer batch.Close()

	batchCount := 0
	processedCount := int64(0)
	lastLogTime := time.Now()
	const logInterval = 5 * time.Second

	for ; it.Valid(); it.Next() {
		key := it.Key()
		value := it.Value()

		// Additional filtering for specific heights (if needed)
		if opts.HeightRange.HasSpecificHeights() {
			// Extract height from key
			var height int64
			var hasHeight bool

			switch opts.DBName {
			case "blockstore":
				height, hasHeight = extractHeightFromBlockstoreKey(key)
			case "tx_index":
				height, hasHeight = extractHeightFromTxIndexKey(key)
			default:
				return fmt.Errorf("unsupported database: %s", opts.DBName)
			}

			if !hasHeight {
				// Skip keys that don't have heights
				continue
			}

			// Check if this height is in our specific list
			if !opts.HeightRange.IsWithinRange(height) {
				continue
			}
		}

		// Copy key-value to batch
		if err := batch.Set(key, value); err != nil {
			stats.ErrorCount.Add(1)
			logger.Error("Failed to set key in batch", "error", err)
			continue
		}

		// Debug log for each key patched
		logger.Debug("Patched key to target database",
			"key_size", len(key),
			"value_size", len(value),
			"batch_count", batchCount,
		)

		batchCount++
		processedCount++

		// Write batch when it reaches the batch size
		if batchCount >= opts.BatchSize {
			logger.Debug("Writing batch to target database",
				"batch_size", batchCount,
				"total_processed", stats.ProcessedKeys.Load()+int64(batchCount),
			)

			if err := batch.Write(); err != nil {
				return fmt.Errorf("failed to write batch: %w", err)
			}

			stats.ProcessedKeys.Add(int64(batchCount))
			batchCount = 0

			// Close and create new batch
			batch.Close()
			batch = targetDB.NewBatch()
		}

		// Periodic logging
		if time.Since(lastLogTime) >= logInterval {
			progress := float64(stats.ProcessedKeys.Load()) / float64(stats.TotalKeys.Load()) * 100
			logger.Info("Patching progress",
				"processed", stats.ProcessedKeys.Load(),
				"total", stats.TotalKeys.Load(),
				"progress", fmt.Sprintf("%.2f%%", progress),
				"errors", stats.ErrorCount.Load(),
			)
			lastLogTime = time.Now()
		}
	}

	// Write remaining batch
	if batchCount > 0 {
		if err := batch.Write(); err != nil {
			return fmt.Errorf("failed to write final batch: %w", err)
		}
		stats.ProcessedKeys.Add(int64(batchCount))
	}

	if err := it.Error(); err != nil {
		return fmt.Errorf("iterator error: %w", err)
	}

	return nil
}

// UpdateBlockStoreHeight updates the block store height metadata in the target database
// This ensures the blockstore knows about the new blocks
func UpdateBlockStoreHeight(targetPath string, backend dbm.BackendType, newHeight int64, rocksDBOpts interface{}) error {
	// Open database
	var db dbm.DB
	var err error
	if backend == dbm.RocksDBBackend {
		db, err = openRocksDBForMigration(targetPath, rocksDBOpts)
	} else {
		targetDir := filepath.Dir(targetPath)
		targetName := filepath.Base(targetPath)
		if len(targetName) > 3 && targetName[len(targetName)-3:] == ".db" {
			targetName = targetName[:len(targetName)-3]
		}
		db, err = dbm.NewDB(targetName, backend, targetDir)
	}
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Read current height
	heightBytes, err := db.Get([]byte("BS:H"))
	if err != nil && err.Error() != "key not found" {
		return fmt.Errorf("failed to read current height: %w", err)
	}

	var currentHeight int64
	if heightBytes != nil {
		var blockStoreState tmstore.BlockStoreState
		if err := proto.Unmarshal(heightBytes, &blockStoreState); err != nil {
			return fmt.Errorf("failed to unmarshal block store state: %w", err)
		}
		currentHeight = blockStoreState.Height
	}

	// Update if new height is higher
	if newHeight > currentHeight {
		blockStoreState := tmstore.BlockStoreState{
			Base:   1, // Assuming base is 1, adjust if needed
			Height: newHeight,
		}

		heightBytes, err := proto.Marshal(&blockStoreState)
		if err != nil {
			return fmt.Errorf("failed to marshal block store state: %w", err)
		}

		if err := db.Set([]byte("BS:H"), heightBytes); err != nil {
			return fmt.Errorf("failed to update height: %w", err)
		}

		// Flush if RocksDB
		if backend == dbm.RocksDBBackend {
			if err := flushRocksDB(db); err != nil {
				return fmt.Errorf("failed to flush: %w", err)
			}
		}
	}

	return nil
}

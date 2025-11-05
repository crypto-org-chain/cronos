package dbmigrate

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tmstore "github.com/cometbft/cometbft/proto/tendermint/store"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/gogoproto/proto"

	"cosmossdk.io/log"
)

// ConflictResolution represents how to handle key conflicts
type ConflictResolution int

const (
	// ConflictAsk prompts user for each conflict
	ConflictAsk ConflictResolution = iota
	// ConflictSkip skips conflicting keys
	ConflictSkip
	// ConflictReplace replaces conflicting keys
	ConflictReplace
	// ConflictReplaceAll replaces all conflicting keys without asking
	ConflictReplaceAll
)

// PatchOptions contains options for patching databases
type PatchOptions struct {
	SourceHome         string             // Source home directory
	TargetPath         string             // Target database path (exact path to patch)
	SourceBackend      dbm.BackendType    // Source backend type
	TargetBackend      dbm.BackendType    // Target backend type
	BatchSize          int                // Batch size for writing
	Logger             log.Logger         // Logger
	RocksDBOptions     interface{}        // RocksDB specific options
	DBName             string             // Database name (blockstore, tx_index, etc.)
	HeightRange        HeightRange        // Height range/specific heights to patch
	ConflictStrategy   ConflictResolution // How to handle key conflicts
	SkipConflictChecks bool               // Skip checking for conflicts (faster, overwrites all)
	DryRun             bool               // If true, simulate operation without writing
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

	if opts.DryRun {
		logger.Info("DRY RUN MODE - No changes will be made")
	}

	logger.Info("Opening databases for patching",
		"source_db", sourceDBPath,
		"source_backend", opts.SourceBackend,
		"target_db", opts.TargetPath,
		"target_backend", opts.TargetBackend,
		"height_range", opts.HeightRange.String(),
		"dry_run", opts.DryRun,
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
		if err := patchWithIterator(it, sourceDB, targetDB, opts, stats); err != nil {
			return fmt.Errorf("failed to patch with iterator %d: %w", idx, err)
		}
	}

	return nil
}

// patchTxIndexData patches tx_index data with special handling for txhash keys
// tx_index has two key types:
//   - tx.height/<height>/<index> - indexed by height (value is the txhash)
//   - <txhash> - direct lookup by hash (value is tx result data)
//
// This function handles both in two passes:
//  1. Patch tx.height keys and collect txhashes from values
//  2. Patch the corresponding txhash keys
func patchTxIndexData(sourceDB, targetDB dbm.DB, opts PatchOptions, stats *MigrationStats) error {
	logger := opts.Logger

	// Get bounded iterator for tx_index (only iterates over tx.height/<height>/ keys)
	it, err := getTxIndexIterator(sourceDB, opts.HeightRange)
	if err != nil {
		return fmt.Errorf("failed to get tx_index iterator: %w", err)
	}
	defer it.Close()

	logger.Info("Patching tx_index data",
		"height_range", opts.HeightRange.String(),
	)

	// Step 1: Iterate through tx.height keys and collect txhashes
	txhashes := make([][]byte, 0, 1000) // Pre-allocate for performance
	batch := targetDB.NewBatch()
	defer batch.Close()

	batchCount := 0
	processedCount := int64(0)
	skippedCount := int64(0)
	currentStrategy := opts.ConflictStrategy

	for it.Valid() {
		key := it.Key()
		value := it.Value()

		// Additional filtering for specific heights (if needed)
		if opts.HeightRange.HasSpecificHeights() {
			height, hasHeight := extractHeightFromTxIndexKey(key)
			if !hasHeight {
				it.Next()
				continue
			}
			if !opts.HeightRange.IsWithinRange(height) {
				it.Next()
				continue
			}
		}

		// Check for key conflicts
		shouldWrite := true
		if !opts.SkipConflictChecks {
			existingValue, err := targetDB.Get(key)
			if err != nil {
				stats.ErrorCount.Add(1)
				logger.Error("Failed to check existing key", "error", err)
				it.Next()
				continue
			}

			if existingValue != nil {
				switch currentStrategy {
				case ConflictAsk:
					decision, newStrategy, err := promptKeyConflict(key, existingValue, value, opts.DBName, opts.HeightRange)
					if err != nil {
						return fmt.Errorf("failed to get user input: %w", err)
					}
					if newStrategy != ConflictAsk {
						currentStrategy = newStrategy
						logger.Info("Conflict resolution strategy updated", "strategy", formatStrategy(newStrategy))
					}
					shouldWrite = decision
					if !decision {
						skippedCount++
					}

				case ConflictSkip:
					shouldWrite = false
					skippedCount++
					logger.Debug("Skipping existing key", "key", formatKeyPrefix(key, 80))

				case ConflictReplace, ConflictReplaceAll:
					shouldWrite = true
					logger.Debug("Replacing existing key", "key", formatKeyPrefix(key, 80))
				}
			}
		}

		if shouldWrite {
			// Patch the tx.height key
			if opts.DryRun {
				logger.Debug("[DRY RUN] Would patch tx.height key",
					"key", formatKeyPrefix(key, 80),
					"value_preview", formatValue(value, 100),
				)
			} else {
				if err := batch.Set(key, value); err != nil {
					stats.ErrorCount.Add(1)
					logger.Error("Failed to set key in batch", "error", err)
					it.Next()
					continue
				}
				logger.Debug("Patched tx.height key", "key", formatKeyPrefix(key, 80))
			}

			batchCount++
			processedCount++

			// Collect txhash for later patching (value IS the txhash)
			if len(value) > 0 {
				// Make a copy of the value since iterator reuses memory
				txhashCopy := make([]byte, len(value))
				copy(txhashCopy, value)
				txhashes = append(txhashes, txhashCopy)
			}

			// Write batch when full
			if batchCount >= opts.BatchSize {
				if !opts.DryRun {
					if err := batch.Write(); err != nil {
						return fmt.Errorf("failed to write batch: %w", err)
					}
					logger.Debug("Wrote batch", "batch_size", batchCount)
					batch.Close()
					batch = targetDB.NewBatch()
				}
				stats.ProcessedKeys.Add(int64(batchCount))
				batchCount = 0
			}
		}

		it.Next()
	}

	// Write remaining batch
	if batchCount > 0 {
		if !opts.DryRun {
			if err := batch.Write(); err != nil {
				return fmt.Errorf("failed to write final batch: %w", err)
			}
			logger.Debug("Wrote final batch", "batch_size", batchCount)
		}
		stats.ProcessedKeys.Add(int64(batchCount))
	}

	if err := it.Error(); err != nil {
		return fmt.Errorf("iterator error: %w", err)
	}

	logger.Info("Patched tx.height keys",
		"processed", processedCount,
		"skipped", skippedCount,
		"txhashes_collected", len(txhashes),
	)

	// Step 2: Patch txhash keys
	if len(txhashes) > 0 {
		logger.Info("Patching txhash lookup keys", "count", len(txhashes))
		if err := patchTxHashKeys(sourceDB, targetDB, txhashes, opts, stats, currentStrategy); err != nil {
			return fmt.Errorf("failed to patch txhash keys: %w", err)
		}
	}

	return nil
}

// patchTxHashKeys patches txhash lookup keys from collected txhashes
func patchTxHashKeys(sourceDB, targetDB dbm.DB, txhashes [][]byte, opts PatchOptions, stats *MigrationStats, currentStrategy ConflictResolution) error {
	logger := opts.Logger
	batch := targetDB.NewBatch()
	defer batch.Close()

	batchCount := 0
	processedCount := int64(0)
	skippedCount := int64(0)

	for _, txhash := range txhashes {
		// Read txhash value from source
		txhashValue, err := sourceDB.Get(txhash)
		if err != nil {
			stats.ErrorCount.Add(1)
			logger.Error("Failed to read txhash from source", "error", err, "txhash", formatKeyPrefix(txhash, 80))
			continue
		}
		if txhashValue == nil {
			logger.Debug("Txhash key not found in source", "txhash", formatKeyPrefix(txhash, 80))
			continue
		}

		// Check for conflicts
		shouldWrite := true
		if !opts.SkipConflictChecks {
			existingValue, err := targetDB.Get(txhash)
			if err != nil {
				stats.ErrorCount.Add(1)
				logger.Error("Failed to check existing txhash", "error", err)
				continue
			}

			if existingValue != nil {
				switch currentStrategy {
				case ConflictSkip:
					shouldWrite = false
					skippedCount++
					logger.Debug("Skipping existing txhash", "txhash", formatKeyPrefix(txhash, 80))

				case ConflictReplace, ConflictReplaceAll:
					shouldWrite = true
					logger.Debug("Replacing existing txhash", "txhash", formatKeyPrefix(txhash, 80))

				case ConflictAsk:
					// Use replace strategy for txhash keys to avoid double-prompting
					shouldWrite = true
					logger.Debug("Patching txhash (using current strategy)", "txhash", formatKeyPrefix(txhash, 80))
				}
			}
		}

		if shouldWrite {
			if opts.DryRun {
				logger.Debug("[DRY RUN] Would patch txhash key",
					"txhash", formatKeyPrefix(txhash, 80),
					"value_preview", formatValue(txhashValue, 100),
				)
			} else {
				if err := batch.Set(txhash, txhashValue); err != nil {
					stats.ErrorCount.Add(1)
					logger.Error("Failed to set txhash in batch", "error", err)
					continue
				}
				logger.Debug("Patched txhash key", "txhash", formatKeyPrefix(txhash, 80))
			}

			batchCount++
			processedCount++

			// Write batch when full
			if batchCount >= opts.BatchSize {
				if !opts.DryRun {
					if err := batch.Write(); err != nil {
						return fmt.Errorf("failed to write txhash batch: %w", err)
					}
					logger.Debug("Wrote txhash batch", "batch_size", batchCount)
					batch.Close()
					batch = targetDB.NewBatch()
				}
				stats.ProcessedKeys.Add(int64(batchCount))
				batchCount = 0
			}
		}
	}

	// Write remaining batch
	if batchCount > 0 {
		if !opts.DryRun {
			if err := batch.Write(); err != nil {
				return fmt.Errorf("failed to write final txhash batch: %w", err)
			}
			logger.Debug("Wrote final txhash batch", "batch_size", batchCount)
		}
		stats.ProcessedKeys.Add(int64(batchCount))
	}

	logger.Info("Patched txhash keys",
		"processed", processedCount,
		"skipped", skippedCount,
	)

	return nil
}

// patchWithIterator patches data from an iterator to target database
func patchWithIterator(it dbm.Iterator, sourceDB, targetDB dbm.DB, opts PatchOptions, stats *MigrationStats) error {
	defer it.Close()

	logger := opts.Logger
	batch := targetDB.NewBatch()
	defer batch.Close()

	batchCount := 0
	processedCount := int64(0)
	skippedCount := int64(0)
	lastLogTime := time.Now()
	const logInterval = 5 * time.Second

	// Track current conflict resolution strategy (may change during execution)
	currentStrategy := opts.ConflictStrategy

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

		// Check for key conflicts if not skipping checks
		shouldWrite := true
		if !opts.SkipConflictChecks {
			existingValue, err := targetDB.Get(key)
			if err != nil {
				stats.ErrorCount.Add(1)
				logger.Error("Failed to check existing key", "error", err)
				continue
			}

			/// log the existing value and key
			logger.Debug("Existing key",
				"key", formatKeyPrefix(key, 80),
				"existing_value_preview", formatValue(existingValue, 100),
			)

			// Key exists in target database (Get returns nil if key doesn't exist)
			if existingValue != nil {
				// Handle conflict based on strategy
				switch currentStrategy {
				case ConflictAsk:
					// Prompt user for decision
					decision, newStrategy, err := promptKeyConflict(key, existingValue, value, opts.DBName, opts.HeightRange)
					if err != nil {
						return fmt.Errorf("failed to get user input: %w", err)
					}

					// If user chose "replace all", update strategy
					if newStrategy != ConflictAsk {
						currentStrategy = newStrategy
						logger.Info("Conflict resolution strategy updated", "strategy", formatStrategy(newStrategy))
					}

					shouldWrite = decision
					if !decision {
						skippedCount++
					}

				case ConflictSkip:
					shouldWrite = false
					skippedCount++
					logger.Debug("Skipping existing key",
						"key", formatKeyPrefix(key, 80),
						"existing_value_preview", formatValue(existingValue, 100),
					)

				case ConflictReplace, ConflictReplaceAll:
					shouldWrite = true
					logger.Debug("Replacing existing key",
						"key", formatKeyPrefix(key, 80),
						"old_value_preview", formatValue(existingValue, 100),
						"new_value_preview", formatValue(value, 100),
					)
				}
			}
		}

		if !shouldWrite {
			continue
		}

		// In dry-run mode, just count what would be written
		if opts.DryRun {
			// Debug log for what would be patched
			logger.Debug("[DRY RUN] Would patch key",
				"key", formatKeyPrefix(key, 80),
				"key_size", len(key),
				"value_preview", formatValue(value, 100),
				"value_size", len(value),
			)
		} else {
			// Copy key-value to batch (actual write)
			if err := batch.Set(key, value); err != nil {
				stats.ErrorCount.Add(1)
				logger.Error("Failed to set key in batch", "error", err)
				continue
			}

			// Debug log for each key patched
			logger.Debug("Patched key to target database",
				"key", formatKeyPrefix(key, 80),
				"key_size", len(key),
				"value_preview", formatValue(value, 100),
				"value_size", len(value),
				"batch_count", batchCount,
			)
		}

		batchCount++
		processedCount++

		// Write batch when it reaches the batch size (skip in dry-run)
		if batchCount >= opts.BatchSize {
			if opts.DryRun {
				logger.Debug("[DRY RUN] Would write batch",
					"batch_size", batchCount,
					"total_processed", stats.ProcessedKeys.Load()+int64(batchCount),
				)
			} else {
				logger.Debug("Writing batch to target database",
					"batch_size", batchCount,
					"total_processed", stats.ProcessedKeys.Load()+int64(batchCount),
				)

				if err := batch.Write(); err != nil {
					return fmt.Errorf("failed to write batch: %w", err)
				}

				// Close and create new batch
				batch.Close()
				batch = targetDB.NewBatch()
			}

			stats.ProcessedKeys.Add(int64(batchCount))
			batchCount = 0
		}

		// Periodic logging
		if time.Since(lastLogTime) >= logInterval {
			progress := float64(stats.ProcessedKeys.Load()) / float64(stats.TotalKeys.Load()) * 100
			logger.Info("Patching progress",
				"processed", stats.ProcessedKeys.Load(),
				"skipped", skippedCount,
				"total", stats.TotalKeys.Load(),
				"progress", fmt.Sprintf("%.2f%%", progress),
				"errors", stats.ErrorCount.Load(),
			)
			lastLogTime = time.Now()
		}
	}

	// Write remaining batch (skip in dry-run)
	if batchCount > 0 {
		if opts.DryRun {
			logger.Debug("[DRY RUN] Would write final batch", "batch_size", batchCount)
		} else {
			if err := batch.Write(); err != nil {
				return fmt.Errorf("failed to write final batch: %w", err)
			}
		}
		stats.ProcessedKeys.Add(int64(batchCount))
	}

	if skippedCount > 0 {
		logger.Info("Skipped conflicting keys", "count", skippedCount)
	}

	if opts.DryRun {
		logger.Info("[DRY RUN] Simulation complete - no changes were made")
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

// promptKeyConflict prompts the user to decide what to do with a conflicting key
// Returns: (shouldWrite bool, newStrategy ConflictResolution, error)
func promptKeyConflict(key, existingValue, newValue []byte, dbName string, heightRange HeightRange) (bool, ConflictResolution, error) {
	// Extract height if possible for display
	var heightStr string
	switch dbName {
	case "blockstore":
		if height, ok := extractHeightFromBlockstoreKey(key); ok {
			heightStr = fmt.Sprintf(" (height: %d)", height)
		}
	case "tx_index":
		if height, ok := extractHeightFromTxIndexKey(key); ok {
			heightStr = fmt.Sprintf(" (height: %d)", height)
		}
	}

	// Display key information
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("KEY CONFLICT DETECTED")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Database:      %s\n", dbName)
	fmt.Printf("Key:           %s%s\n", formatKeyPrefix(key, 40), heightStr)
	fmt.Printf("Existing size: %d bytes\n", len(existingValue))
	fmt.Printf("New size:      %d bytes\n", len(newValue))
	fmt.Println(strings.Repeat("-", 80))

	// Prompt for action
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Action? [(r)eplace, (s)kip, (R)eplace all, (S)kip all]: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return false, ConflictAsk, fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(input)
		inputLower := strings.ToLower(input)

		switch {
		case input == "R":
			return true, ConflictReplaceAll, nil
		case input == "S":
			return false, ConflictSkip, nil
		case inputLower == "r" || inputLower == "replace":
			return true, ConflictAsk, nil
		case inputLower == "s" || inputLower == "skip":
			return false, ConflictAsk, nil
		default:
			fmt.Println("Invalid input. Please enter r, s, R, or S.")
		}
	}
}

// formatKeyPrefix formats a key for display, truncating if necessary
func formatKeyPrefix(key []byte, maxLen int) string {
	if len(key) <= maxLen {
		return string(key)
	}
	return string(key[:maxLen]) + "..."
}

// formatValue formats a value for display
// If the value appears to be binary data, it shows a hex preview
// Otherwise, it shows the string representation
func formatValue(value []byte, maxLen int) string {
	if len(value) == 0 {
		return "<empty>"
	}

	// Check if value is mostly printable ASCII (heuristic for text vs binary)
	printableCount := 0
	for _, b := range value {
		if b >= 32 && b <= 126 || b == 9 || b == 10 || b == 13 {
			printableCount++
		}
	}

	// If more than 80% is printable, treat as text
	if float64(printableCount)/float64(len(value)) > 0.8 {
		if len(value) <= maxLen {
			return string(value)
		}
		return string(value[:maxLen]) + fmt.Sprintf("... (%d more bytes)", len(value)-maxLen)
	}

	// Otherwise, show as hex
	hexPreview := fmt.Sprintf("%x", value)
	if len(hexPreview) <= maxLen {
		return "0x" + hexPreview
	}
	return "0x" + hexPreview[:maxLen] + fmt.Sprintf("... (%d total bytes)", len(value))
}

// formatStrategy returns a human-readable string for a conflict resolution strategy
func formatStrategy(strategy ConflictResolution) string {
	switch strategy {
	case ConflictAsk:
		return "ask"
	case ConflictSkip:
		return "skip all"
	case ConflictReplace:
		return "replace"
	case ConflictReplaceAll:
		return "replace all"
	default:
		return "unknown"
	}
}

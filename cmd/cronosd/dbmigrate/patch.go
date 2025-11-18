package dbmigrate

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/gogoproto/proto"

	"cosmossdk.io/log"
)

const (
	DbExtension = ".db"
)

// EthTxInfo stores information needed to search for event-indexed keys in source DB
type EthTxInfo struct {
	Height  int64 // Block height
	TxIndex int64 // Transaction index within block
}

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

// validatePatchOptions validates the patch options
func validatePatchOptions(opts PatchOptions) error {
	if opts.Logger == nil {
		return fmt.Errorf("logger is required")
	}
	if opts.HeightRange.IsEmpty() {
		return fmt.Errorf("height range is required for patching")
	}
	if !supportsHeightFiltering(opts.DBName) {
		return fmt.Errorf("database %s does not support height-based patching (only blockstore and tx_index supported)", opts.DBName)
	}

	// Construct and validate source database path
	sourceDBPath := filepath.Join(opts.SourceHome, "data", opts.DBName+DbExtension)
	if _, err := os.Stat(sourceDBPath); os.IsNotExist(err) {
		return fmt.Errorf("source database does not exist: %s", sourceDBPath)
	}

	// Validate target exists
	if _, err := os.Stat(opts.TargetPath); os.IsNotExist(err) {
		return fmt.Errorf("target database does not exist: %s (use db migrate to create new databases)", opts.TargetPath)
	}

	return nil
}

// openSourceDatabase opens the source database for reading
func openSourceDatabase(opts PatchOptions) (dbm.DB, string, error) {
	sourceDBPath := filepath.Join(opts.SourceHome, "data", opts.DBName+DbExtension)
	sourceDir := filepath.Dir(sourceDBPath)
	sourceName := filepath.Base(sourceDBPath)
	if len(sourceName) > len(DbExtension) && sourceName[len(sourceName)-len(DbExtension):] == DbExtension {
		sourceName = sourceName[:len(sourceName)-len(DbExtension)]
	}

	sourceDB, err := dbm.NewDB(sourceName, opts.SourceBackend, sourceDir)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open source database: %w", err)
	}
	return sourceDB, sourceDBPath, nil
}

// openTargetDatabase opens the target database for patching
func openTargetDatabase(opts PatchOptions) (dbm.DB, error) {
	var targetDB dbm.DB
	var err error

	if opts.TargetBackend == dbm.RocksDBBackend {
		targetDB, err = openRocksDBForMigration(opts.TargetPath, opts.RocksDBOptions)
	} else {
		targetDir := filepath.Dir(opts.TargetPath)
		targetName := filepath.Base(opts.TargetPath)
		targetName = strings.TrimSuffix(targetName, DbExtension)
		targetDB, err = dbm.NewDB(targetName, opts.TargetBackend, targetDir)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open target database: %w", err)
	}
	return targetDB, nil
}

// PatchDatabase patches specific heights from source to target database
func PatchDatabase(opts PatchOptions) (*MigrationStats, error) {
	// Validate options
	if err := validatePatchOptions(opts); err != nil {
		return nil, err
	}

	logger := opts.Logger
	stats := &MigrationStats{
		StartTime: time.Now(),
	}

	// Log dry-run mode if enabled
	if opts.DryRun {
		logger.Info("DRY RUN MODE - No changes will be made")
		if opts.DBName == DBNameBlockstore {
			logger.Info("Note: Blockstore patching will also discover and patch corresponding BH: (block header by hash) keys")
		}
	}

	// Open source database
	sourceDB, sourceDBPath, err := openSourceDatabase(opts)
	if err != nil {
		return stats, err
	}
	defer sourceDB.Close()

	// Open target database
	targetDB, err := openTargetDatabase(opts)
	if err != nil {
		return stats, err
	}
	defer targetDB.Close()

	logger.Info("Opening databases for patching",
		"source_db", sourceDBPath,
		"source_backend", opts.SourceBackend,
		"target_db", opts.TargetPath,
		"target_backend", opts.TargetBackend,
		"height_range", opts.HeightRange.String(),
		"dry_run", opts.DryRun,
	)

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
	case DBNameBlockstore:
		// For blockstore, count keys from all prefixes
		iterators, err := getBlockstoreIterators(db, heightRange)
		if err != nil {
			return 0, fmt.Errorf("failed to get blockstore iterators: %w", err)
		}

		keysSeen := 0
		for iterIdx, it := range iterators {
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
			it.Close()
		}
		logger.Debug("Total keys seen in blockstore", "total_seen", keysSeen, "total_counted", totalCount)

	case DBNameTxIndex:
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
	case DBNameBlockstore:
		return patchBlockstoreData(sourceDB, targetDB, opts, stats)
	case DBNameTxIndex:
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

// extractHeightAndTxIndexFromKey extracts height and txIndex from a tx.height key
// Returns (height, txIndex, success)
func extractHeightAndTxIndexFromKey(key []byte, logger log.Logger) (int64, int64, bool) {
	keyStr := string(key)
	if !bytes.HasPrefix(key, []byte("tx.height/")) {
		return 0, 0, false
	}

	// Format: "tx.height/<height>/<height>/<txindex>$es$0" or "tx.height/<height>/<height>/<txindex>"
	parts := strings.Split(keyStr[len("tx.height/"):], "/")
	if len(parts) < 3 {
		return 0, 0, false
	}

	// parts[0] = height (first occurrence)
	// parts[1] = height (second occurrence, same value)
	// parts[2] = txindex$es$0 OR just txindex
	var height, txIndex int64
	_, err := fmt.Sscanf(parts[0], "%d", &height)
	if err != nil {
		logger.Debug("Failed to parse height from tx.height key", "key", keyStr, "error", err)
		return 0, 0, false
	}

	// Extract txIndex - handle both with and without "$es$" suffix
	txIndexStr := parts[2]
	if strings.Contains(txIndexStr, "$es$") {
		// Key has "$es$<eventseq>" suffix
		txIndexStr = strings.Split(txIndexStr, "$es$")[0]
	}
	_, err = fmt.Sscanf(txIndexStr, "%d", &txIndex)
	if err != nil {
		logger.Debug("Failed to parse txIndex from tx.height key", "key", keyStr, "error", err)
		return 0, 0, false
	}

	return height, txIndex, true
}

// checkTxHeightKeyConflict checks for key conflicts and returns whether to write
// Returns (shouldWrite, newStrategy, skipped)
func checkTxHeightKeyConflict(key, value []byte, targetDB dbm.DB, currentStrategy ConflictResolution, opts PatchOptions, logger log.Logger) (bool, ConflictResolution, bool) {
	if opts.SkipConflictChecks {
		return true, currentStrategy, false
	}

	existingValue, err := targetDB.Get(key)
	if err != nil {
		logger.Error("Failed to check existing key", "error", err)
		return false, currentStrategy, false
	}

	// No conflict if key doesn't exist
	if existingValue == nil {
		return true, currentStrategy, false
	}

	// Handle conflict based on strategy
	switch currentStrategy {
	case ConflictAsk:
		decision, newStrategy, err := promptKeyConflict(key, existingValue, value, opts.DBName, opts.HeightRange)
		if err != nil {
			logger.Error("Failed to get user input", "error", err)
			return false, currentStrategy, false
		}
		if newStrategy != ConflictAsk {
			logger.Info("Conflict resolution strategy updated", "strategy", formatStrategy(newStrategy))
		}
		return decision, newStrategy, !decision

	case ConflictSkip:
		logger.Debug("Skipping existing key", "key", formatKeyPrefix(key, 80))
		return false, currentStrategy, true

	case ConflictReplace, ConflictReplaceAll:
		logger.Debug("Replacing existing key", "key", formatKeyPrefix(key, 80))
		return true, currentStrategy, false
	}

	return true, currentStrategy, false
}

// patchTxHeightKeyAndCollect patches a tx.height key and collects txhash info
// Returns true if batch should be written, false if error occurred
func patchTxHeightKeyAndCollect(key, value []byte, sourceDB dbm.DB, batch dbm.Batch, txhashes *[][]byte, ethTxInfos map[string]EthTxInfo, opts PatchOptions, stats *MigrationStats, logger log.Logger) bool {
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
			return false
		}
		logger.Debug("Patched tx.height key", "key", formatKeyPrefix(key, 80))
	}

	// Collect CometBFT txhash for later patching (value is the CometBFT txhash)
	if len(value) > 0 {
		// Make a copy of the value since iterator reuses memory
		txhashCopy := make([]byte, len(value))
		copy(txhashCopy, value)
		*txhashes = append(*txhashes, txhashCopy)

		// Extract height and txIndex from the key
		height, txIndex, ok := extractHeightAndTxIndexFromKey(key, logger)
		if ok {
			// Try to collect Ethereum txhash for event-indexed keys
			collectEthereumTxInfo(sourceDB, txhashCopy, height, txIndex, ethTxInfos, logger)
		}
	}

	return true
}

// collectEthereumTxInfo tries to extract Ethereum txhash from a transaction result
// and stores it in ethTxInfos map if found
func collectEthereumTxInfo(sourceDB dbm.DB, txhash []byte, height, txIndex int64, ethTxInfos map[string]EthTxInfo, logger log.Logger) {
	// Read the transaction result from source database
	txResultValue, err := sourceDB.Get(txhash)
	if err != nil || txResultValue == nil {
		return
	}

	// Extract ethereum txhash from events
	ethTxHash, err := extractEthereumTxHash(txResultValue)
	if err != nil {
		logger.Debug("Failed to extract ethereum txhash", "error", err, "cometbft_txhash", formatKeyPrefix(txhash, 80))
		return
	}

	if ethTxHash != "" {
		// Store the info for later Ethereum event key patching
		ethTxInfos[ethTxHash] = EthTxInfo{
			Height:  height,
			TxIndex: txIndex,
		}
		logger.Debug("Collected ethereum txhash",
			"eth_txhash", ethTxHash,
			"cometbft_txhash", formatKeyPrefix(txhash, 80),
			"height", height,
			"tx_index", txIndex,
		)
	}
}

// patchTxHeightKeys patches tx.height keys and collects txhashes and ethereum tx info
// Returns (txhashes, ethTxInfos, currentStrategy, error)
func patchTxHeightKeys(it dbm.Iterator, sourceDB, targetDB dbm.DB, opts PatchOptions, stats *MigrationStats) ([][]byte, map[string]EthTxInfo, ConflictResolution, error) {
	logger := opts.Logger
	txhashes := make([][]byte, 0, 1000)      // Pre-allocate for performance
	ethTxInfos := make(map[string]EthTxInfo) // eth_txhash (hex) -> EthTxInfo
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
		shouldWrite, newStrategy, skipped := checkTxHeightKeyConflict(key, value, targetDB, currentStrategy, opts, logger)
		if newStrategy != currentStrategy {
			currentStrategy = newStrategy
		}
		if skipped {
			skippedCount++
		}
		if !shouldWrite {
			it.Next()
			continue
		}

		// Patch the key and collect txhash info
		if !patchTxHeightKeyAndCollect(key, value, sourceDB, batch, &txhashes, ethTxInfos, opts, stats, logger) {
			it.Next()
			continue
		}

		batchCount++
		processedCount++

		// Write batch when full
		if batchCount >= opts.BatchSize {
			if !opts.DryRun {
				if err := batch.Write(); err != nil {
					return nil, nil, currentStrategy, fmt.Errorf("failed to write batch: %w", err)
				}
				logger.Debug("Wrote batch", "batch_size", batchCount)
				batch.Close()
				batch = targetDB.NewBatch()
			}
			stats.ProcessedKeys.Add(int64(batchCount))
			batchCount = 0
		}

		it.Next()
	}

	// Write remaining batch
	if batchCount > 0 {
		if !opts.DryRun {
			if err := batch.Write(); err != nil {
				return nil, nil, currentStrategy, fmt.Errorf("failed to write final batch: %w", err)
			}
			logger.Debug("Wrote final batch", "batch_size", batchCount)
		}
		stats.ProcessedKeys.Add(int64(batchCount))
	}

	if err := it.Error(); err != nil {
		return nil, nil, currentStrategy, fmt.Errorf("iterator error: %w", err)
	}

	logger.Info("Patched tx.height keys",
		"processed", processedCount,
		"skipped", skippedCount,
		"txhashes_collected", len(txhashes),
		"ethereum_txhashes_collected", len(ethTxInfos),
	)

	return txhashes, ethTxInfos, currentStrategy, nil
}

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

	// Step 1: Patch tx.height keys and collect CometBFT txhashes and Ethereum tx info
	txhashes, ethTxInfos, currentStrategy, err := patchTxHeightKeys(it, sourceDB, targetDB, opts, stats)
	if err != nil {
		return err
	}

	// Step 2: Patch CometBFT txhash keys
	if len(txhashes) > 0 {
		logger.Info("Patching CometBFT txhash lookup keys", "count", len(txhashes))
		if err := patchTxHashKeys(sourceDB, targetDB, txhashes, opts, stats, currentStrategy); err != nil {
			return fmt.Errorf("failed to patch txhash keys: %w", err)
		}
	}

	// Step 3: Patch Ethereum event-indexed keys from source database
	if len(ethTxInfos) > 0 {
		logger.Info("Patching Ethereum event-indexed keys from source database", "count", len(ethTxInfos))
		if err := patchEthereumEventKeysFromSource(sourceDB, targetDB, ethTxInfos, opts, stats, currentStrategy); err != nil {
			return fmt.Errorf("failed to patch ethereum event keys: %w", err)
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

// extractEthereumTxHash extracts the Ethereum transaction hash from transaction result events
// Returns the eth txhash (with 0x prefix) if found, empty string otherwise
func extractEthereumTxHash(txResultValue []byte) (string, error) {
	// Decode the transaction result
	var txResult abci.TxResult
	if err := proto.Unmarshal(txResultValue, &txResult); err != nil {
		return "", fmt.Errorf("failed to unmarshal tx result: %w", err)
	}

	// Look for ethereum_tx event with eth_hash attribute
	for _, event := range txResult.Result.Events {
		if event.Type == "ethereum_tx" {
			for _, attr := range event.Attributes {
				if attr.Key == "ethereumTxHash" {
					// The value is the Ethereum txhash (with or without 0x prefix)
					ethHash := attr.Value
					// Ensure 0x prefix is present
					if len(ethHash) >= 2 && ethHash[:2] != "0x" {
						ethHash = "0x" + ethHash
					}
					// Validate it's a valid hex hash (should be 66 characters: 0x + 64 hex chars)
					if len(ethHash) != 66 {
						return "", fmt.Errorf("invalid ethereum txhash length: %d", len(ethHash))
					}
					// Decode to verify it's valid hex (skip 0x prefix)
					if _, err := hex.DecodeString(ethHash[2:]); err != nil {
						return "", fmt.Errorf("invalid ethereum txhash hex: %w", err)
					}
					return ethHash, nil
				}
			}
		}
	}

	// No ethereum_tx event found (this is normal for non-EVM transactions)
	return "", nil
}

// incrementBytes increments a byte slice by 1 to create an exclusive upper bound for iterators
// Returns a new byte slice that is the input + 1
func incrementBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}

	// Create a copy to avoid modifying the original
	incremented := make([]byte, len(b))
	copy(incremented, b)

	// Increment from the last byte, carrying over if necessary
	for i := len(incremented) - 1; i >= 0; i-- {
		if incremented[i] < 0xFF {
			incremented[i]++
			return incremented
		}
		// If byte is 0xFF, set to 0x00 and continue to carry
		incremented[i] = 0x00
	}

	// If all bytes were 0xFF, return nil to signal no exclusive upper bound
	return nil
}

// patchEthereumEventKeysFromSource patches ethereum event-indexed keys by searching source DB
// Key format: "ethereum_tx.ethereumTxHash/0x<eth_txhash>/<height>/<txindex>$es$<eventseq>"
//
//	or "ethereum_tx.ethereumTxHash/0x<eth_txhash>/<height>/<txindex>" (without $es$ suffix)
//
// Value: CometBFT tx hash (allows lookup by Ethereum txhash)
func patchEthereumEventKeysFromSource(sourceDB, targetDB dbm.DB, ethTxInfos map[string]EthTxInfo, opts PatchOptions, stats *MigrationStats, currentStrategy ConflictResolution) error {
	logger := opts.Logger
	batch := targetDB.NewBatch()
	defer batch.Close()

	batchCount := 0
	processedCount := int64(0)
	skippedCount := int64(0)

	// For each Ethereum transaction, create a specific prefix and iterate
	for ethTxHash, info := range ethTxInfos {
		// Create specific prefix for this transaction to minimize iteration range
		// Format: ethereum_tx.ethereumTxHash/0x<eth_txhash>/<height>/<txindex>
		// This will match both keys with and without "$es$<eventseq>" suffix
		// Note: ethTxHash already includes the 0x prefix
		prefix := fmt.Sprintf("ethereum_tx.ethereumTxHash/%s/%d/%d", ethTxHash, info.Height, info.TxIndex)
		prefixBytes := []byte(prefix)

		// Create end boundary by incrementing the prefix (exclusive upper bound)
		endBytes := incrementBytes(prefixBytes)

		// Create bounded iterator with [start, end)
		it, err := sourceDB.Iterator(prefixBytes, endBytes)
		if err != nil {
			logger.Error("Failed to create iterator for ethereum event keys", "error", err, "eth_txhash", ethTxHash)
			stats.ErrorCount.Add(1)
			continue
		}

		eventKeysFound := 0
		for it.Valid() {
			key := it.Key()
			value := it.Value()

			// Stop if we're past the prefix
			if !bytes.HasPrefix(key, prefixBytes) {
				break
			}

			eventKeysFound++
			keyStr := string(key)

			logger.Debug("Found ethereum event key in source",
				"event_key", keyStr,
				"eth_txhash", ethTxHash,
				"height", info.Height,
				"tx_index", info.TxIndex,
			)

			// Check for conflicts
			shouldWrite := true
			if !opts.SkipConflictChecks {
				existingValue, err := targetDB.Get(key)
				if err != nil {
					stats.ErrorCount.Add(1)
					logger.Error("Failed to check existing ethereum event key", "error", err)
					it.Next()
					continue
				}

				if existingValue != nil {
					switch currentStrategy {
					case ConflictSkip:
						shouldWrite = false
						skippedCount++
						logger.Debug("Skipping existing ethereum event key",
							"event_key", keyStr,
						)

					case ConflictReplace, ConflictReplaceAll:
						shouldWrite = true
						logger.Debug("Replacing existing ethereum event key",
							"event_key", keyStr,
						)

					case ConflictAsk:
						// Use replace strategy for event keys to avoid excessive prompting
						shouldWrite = true
						logger.Debug("Patching ethereum event key (using current strategy)",
							"event_key", keyStr,
						)
					}
				}
			}

			if shouldWrite {
				// Make a copy of the value since iterator reuses memory
				valueCopy := make([]byte, len(value))
				copy(valueCopy, value)

				if opts.DryRun {
					logger.Debug("[DRY RUN] Would patch ethereum event key",
						"event_key", keyStr,
						"value_preview", formatKeyPrefix(valueCopy, 80),
					)
				} else {
					if err := batch.Set(key, valueCopy); err != nil {
						stats.ErrorCount.Add(1)
						logger.Error("Failed to set ethereum event key in batch", "error", err)
						it.Next()
						continue
					}
					logger.Debug("Patched ethereum event key",
						"event_key", keyStr,
						"value_preview", formatKeyPrefix(valueCopy, 80),
					)
				}

				batchCount++
				processedCount++

				// Write batch when full
				if batchCount >= opts.BatchSize {
					if !opts.DryRun {
						if err := batch.Write(); err != nil {
							it.Close()
							return fmt.Errorf("failed to write ethereum event batch: %w", err)
						}
						logger.Debug("Wrote ethereum event batch", "batch_size", batchCount)
						batch.Close()
						batch = targetDB.NewBatch()
					}
					stats.ProcessedKeys.Add(int64(batchCount))
					batchCount = 0
				}
			}

			it.Next()
		}

		if err := it.Error(); err != nil {
			it.Close()
			return fmt.Errorf("iterator error for eth_txhash %s: %w", ethTxHash, err)
		}

		it.Close()

		if eventKeysFound > 0 {
			logger.Debug("Processed event keys for transaction",
				"eth_txhash", ethTxHash,
				"event_keys_found", eventKeysFound,
			)
		}
	}

	// Write remaining batch
	if batchCount > 0 {
		if !opts.DryRun {
			if err := batch.Write(); err != nil {
				return fmt.Errorf("failed to write final ethereum event batch: %w", err)
			}
			logger.Debug("Wrote final ethereum event batch", "batch_size", batchCount)
		}
		stats.ProcessedKeys.Add(int64(batchCount))
	}

	logger.Info("Patched ethereum event keys from source database",
		"processed", processedCount,
		"skipped", skippedCount,
	)

	return nil
}

// patchWithIterator patches data from an iterator to target database
// shouldProcessKey checks if a key should be processed based on height filtering
func shouldProcessKey(key []byte, dbName string, heightRange HeightRange) bool {
	if !heightRange.HasSpecificHeights() {
		return true
	}

	// Extract height from key
	var height int64
	var hasHeight bool

	switch dbName {
	case DBNameBlockstore:
		height, hasHeight = extractHeightFromBlockstoreKey(key)
	case DBNameTxIndex:
		height, hasHeight = extractHeightFromTxIndexKey(key)
	default:
		return false
	}

	if !hasHeight {
		return false
	}

	return heightRange.IsWithinRange(height)
}

// handleKeyConflict handles key conflict resolution
// Returns (shouldWrite bool, newStrategy ConflictResolution, skipped bool)
func handleKeyConflict(key, existingValue, newValue []byte, targetDB dbm.DB, currentStrategy ConflictResolution, opts PatchOptions, logger log.Logger) (bool, ConflictResolution, bool) {
	if opts.SkipConflictChecks {
		return true, currentStrategy, false
	}

	// Key doesn't exist, no conflict
	if existingValue == nil {
		return true, currentStrategy, false
	}

	// log the existing value and key
	logger.Debug("Existing key",
		"key", formatKeyPrefix(key, 80),
		"existing_value_preview", formatValue(existingValue, 100),
	)

	// Handle conflict based on strategy
	switch currentStrategy {
	case ConflictAsk:
		decision, newStrategy, err := promptKeyConflict(key, existingValue, newValue, opts.DBName, opts.HeightRange)
		if err != nil {
			logger.Error("Failed to get user input", "error", err)
			return false, currentStrategy, true
		}
		if newStrategy != ConflictAsk {
			logger.Info("Conflict resolution strategy updated", "strategy", formatStrategy(newStrategy))
		}
		return decision, newStrategy, !decision

	case ConflictSkip:
		logger.Debug("Skipping existing key",
			"key", formatKeyPrefix(key, 80),
			"existing_value_preview", formatValue(existingValue, 100),
		)
		return false, currentStrategy, true

	case ConflictReplace, ConflictReplaceAll:
		logger.Debug("Replacing existing key",
			"key", formatKeyPrefix(key, 80),
			"old_value_preview", formatValue(existingValue, 100),
			"new_value_preview", formatValue(newValue, 100),
		)
		return true, currentStrategy, false
	}

	return true, currentStrategy, false
}

// patchSingleKey patches a single key-value pair, including BH: key for blockstore H: keys
func patchSingleKey(key, value []byte, sourceDB dbm.DB, batch dbm.Batch, opts PatchOptions, logger log.Logger) error {
	if opts.DryRun {
		// Debug log for what would be patched
		logger.Debug("[DRY RUN] Would patch key",
			"key", formatKeyPrefix(key, 80),
			"key_size", len(key),
			"value_preview", formatValue(value, 100),
			"value_size", len(value),
		)

		// For blockstore H: keys, check if corresponding BH:<hash> key would be patched
		if opts.DBName == DBNameBlockstore && len(key) > 2 && key[0] == 'H' && key[1] == ':' {
			if blockHash, ok := extractBlockHashFromMetadata(value); ok {
				// Check if BH: key exists in source DB
				bhKey := make([]byte, 3+len(blockHash))
				copy(bhKey[0:3], []byte("BH:"))
				copy(bhKey[3:], blockHash)

				bhValue, err := sourceDB.Get(bhKey)
				if err == nil && bhValue != nil {
					logger.Debug("[DRY RUN] Would patch BH: key",
						"hash", fmt.Sprintf("%x", blockHash),
						"key_size", len(bhKey),
						"value_size", len(bhValue),
					)
				}
			}
		}
		return nil
	}

	// Copy key-value to batch (actual write)
	if err := batch.Set(key, value); err != nil {
		return fmt.Errorf("failed to set key in batch: %w", err)
	}

	// For blockstore H: keys, also patch the corresponding BH:<hash> key
	if opts.DBName == DBNameBlockstore && len(key) > 2 && key[0] == 'H' && key[1] == ':' {
		if blockHash, ok := extractBlockHashFromMetadata(value); ok {
			// Construct BH: key
			bhKey := make([]byte, 3+len(blockHash))
			copy(bhKey[0:3], []byte("BH:"))
			copy(bhKey[3:], blockHash)

			// Try to get the value from source DB
			bhValue, err := sourceDB.Get(bhKey)
			if err == nil && bhValue != nil {
				// Make a copy of the value before adding to batch
				bhValueCopy := make([]byte, len(bhValue))
				copy(bhValueCopy, bhValue)

				if err := batch.Set(bhKey, bhValueCopy); err != nil {
					logger.Debug("Failed to patch BH: key", "error", err, "hash", fmt.Sprintf("%x", blockHash))
				} else {
					logger.Debug("Patched BH: key", "hash", fmt.Sprintf("%x", blockHash))
				}
			}
		}
	}

	// Debug log for each key patched
	logger.Debug("Patched key to target database",
		"key", formatKeyPrefix(key, 80),
		"key_size", len(key),
		"value_preview", formatValue(value, 100),
		"value_size", len(value),
	)

	return nil
}

// writeAndResetBatch writes the batch to the database and creates a new batch
func writeAndResetBatch(batch dbm.Batch, targetDB dbm.DB, batchCount int, opts PatchOptions, logger log.Logger) (dbm.Batch, error) {
	if opts.DryRun {
		logger.Debug("[DRY RUN] Would write batch", "batch_size", batchCount)
		return batch, nil
	}

	logger.Debug("Writing batch to target database", "batch_size", batchCount)
	if err := batch.Write(); err != nil {
		return batch, fmt.Errorf("failed to write batch: %w", err)
	}

	// Close and create new batch
	batch.Close()
	return targetDB.NewBatch(), nil
}

func patchWithIterator(it dbm.Iterator, sourceDB, targetDB dbm.DB, opts PatchOptions, stats *MigrationStats) error {
	defer it.Close()

	logger := opts.Logger
	batch := targetDB.NewBatch()
	defer batch.Close()

	batchCount := 0
	skippedCount := int64(0)
	lastLogTime := time.Now()
	const logInterval = 5 * time.Second

	// Track current conflict resolution strategy (may change during execution)
	currentStrategy := opts.ConflictStrategy

	for ; it.Valid(); it.Next() {
		key := it.Key()
		value := it.Value()

		// Check if we should process this key (height filtering)
		if !shouldProcessKey(key, opts.DBName, opts.HeightRange) {
			continue
		}

		// Check for key conflicts and get resolution
		existingValue, err := targetDB.Get(key)
		if err != nil {
			stats.ErrorCount.Add(1)
			logger.Error("Failed to check existing key", "error", err)
			continue
		}

		shouldWrite, newStrategy, skipped := handleKeyConflict(key, existingValue, value, targetDB, currentStrategy, opts, logger)
		if newStrategy != currentStrategy {
			currentStrategy = newStrategy
		}
		if skipped {
			skippedCount++
		}
		if !shouldWrite {
			continue
		}

		// Patch the key-value pair
		if err := patchSingleKey(key, value, sourceDB, batch, opts, logger); err != nil {
			stats.ErrorCount.Add(1)
			logger.Error("Failed to patch key", "error", err)
			continue
		}

		batchCount++

		// Write batch when it reaches the batch size
		if batchCount >= opts.BatchSize {
			batch, err = writeAndResetBatch(batch, targetDB, batchCount, opts, logger)
			if err != nil {
				return err
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

	// Write remaining batch
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

	// Final logging
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

// promptKeyConflict prompts the user to decide what to do with a conflicting key
// Returns: (shouldWrite bool, newStrategy ConflictResolution, error)
func promptKeyConflict(key, existingValue, newValue []byte, dbName string, heightRange HeightRange) (bool, ConflictResolution, error) {
	// Extract height if possible for display
	var heightStr string
	switch dbName {
	case DBNameBlockstore:
		if height, ok := extractHeightFromBlockstoreKey(key); ok {
			heightStr = fmt.Sprintf(" (height: %d)", height)
		}
	case DBNameTxIndex:
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
// Detects binary data (like txhashes) and formats as hex
func formatKeyPrefix(key []byte, maxLen int) string {
	if len(key) == 0 {
		return "<empty>"
	}

	// Check if key is mostly printable ASCII (heuristic for text vs binary)
	printableCount := 0
	for _, b := range key {
		if (b >= 32 && b <= 126) || b == 9 || b == 10 || b == 13 || b == '/' || b == ':' {
			printableCount++
		}
	}

	// If more than 80% is printable, treat as text (e.g., "tx.height/123/0")
	if float64(printableCount)/float64(len(key)) > 0.8 {
		if len(key) <= maxLen {
			return string(key)
		}
		return string(key[:maxLen]) + "..."
	}

	// Otherwise, format as hex (e.g., txhashes)
	hexStr := fmt.Sprintf("%x", key)
	if len(hexStr) <= maxLen {
		return "0x" + hexStr
	}
	// Truncate hex string if too long
	halfLen := (maxLen - 8) / 2 // Reserve space for "0x" and "..."
	if maxLen <= 8 || halfLen <= 0 {
		// Not enough space for "0x..."; just truncate what we can
		if maxLen <= 2 {
			return "0x"
		}
		// Truncate to maxLen-2 to account for "0x" prefix
		truncLen := maxLen - 2
		if truncLen > len(hexStr) {
			truncLen = len(hexStr)
		}
		return "0x" + hexStr[:truncLen]
	}
	return "0x" + hexStr[:halfLen] + "..." + hexStr[len(hexStr)-halfLen:]
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
		if (b >= 32 && b <= 126) || b == 9 || b == 10 || b == 13 {
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

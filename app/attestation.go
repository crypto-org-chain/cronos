package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cosmossdk.io/log"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	dbm "github.com/cosmos/cosmos-db"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/spf13/cast"

	attestationcollector "github.com/crypto-org-chain/cronos/x/attestation/collector"
)

// setupAttestationFinalityStorage initializes the local finality storage for the attestation module
// This storage is used to track pending attestations and finality status (non-consensus data)
func setupAttestationFinalityStorage(app *App, homePath string, appOpts servertypes.AppOptions, logger log.Logger, db dbm.DB) error {

	dbBackend, err := getDBBackend(db, appOpts)
	if err != nil {
		return err
	}

	finalityDBPath := filepath.Join(homePath, "data", "finality")

	// MemDB doesn't persist - skip existence check
	if dbBackend != dbm.MemDBBackend {
		if err := os.MkdirAll(finalityDBPath, 0755); err != nil {
			return fmt.Errorf("failed to create finality database directory: %w", err)
		}
		if err := cleanupMismatchedDB(finalityDBPath, dbBackend, logger); err != nil {
			logger.Error("Failed to cleanup mismatched database", "error", err)
		}
	}

	cacheSize := cast.ToInt(appOpts.Get("attestation.finality-cache-size"))
	if cacheSize == 0 {
		cacheSize = 10000
	}

	if err := app.AttestationKeeper.InitializeLocalStorage(finalityDBPath, cacheSize, dbBackend); err != nil {
		return fmt.Errorf("failed to initialize local finality storage: %w", err)
	}

	logger.Info("Initialized attestation local finality storage",
		"path", finalityDBPath,
		"backend", dbBackend,
		"cache_size", cacheSize,
	)

	// Initialize Block Data Collector (non-fatal if fails)
	if err := setupBlockDataCollector(app, homePath, dbBackend, logger); err != nil {
		logger.Warn("Failed to setup block data collector", "error", err)
	}

	return nil
}

// getDBBackend determines the database backend type from the db instance or app options
func getDBBackend(db dbm.DB, appOpts servertypes.AppOptions) (dbm.BackendType, error) {
	// Check concrete types first (MemDB and GoLevelDB are always available)
	switch db.(type) {
	case *dbm.MemDB:
		return dbm.MemDBBackend, nil
	case *dbm.GoLevelDB:
		return dbm.GoLevelDBBackend, nil
	}

	// Fall back to config for RocksDB (conditionally compiled)
	dbBackendStr := cast.ToString(appOpts.Get("app-db-backend"))
	if dbBackendStr == "" {
		dbBackendStr = "goleveldb"
	}

	switch dbBackendStr {
	case "goleveldb":
		return dbm.GoLevelDBBackend, nil
	case "rocksdb":
		return dbm.RocksDBBackend, nil
	case "memdb":
		return dbm.MemDBBackend, nil
	default:
		return "", fmt.Errorf("unsupported database backend: %s", dbBackendStr)
	}
}

// cleanupMismatchedDB removes existing database if it doesn't match the configured backend
func cleanupMismatchedDB(dbPath string, backend dbm.BackendType, logger log.Logger) error {
	exists, matches := checkFinalityDatabaseExists(dbPath, backend)
	if !exists || matches {
		return nil
	}

	logger.Warn("Existing finality database has different backend type, removing",
		"path", dbPath,
		"configured_backend", backend,
	)

	return os.RemoveAll(filepath.Join(dbPath, "finality.db"))
}

// setupBlockDataCollector initializes the block data collector for full block attestation
// This collector runs in the background and subscribes to CometBFT events
func setupBlockDataCollector(app *App, homePath string, dbBackend dbm.BackendType, logger log.Logger) error {
	// Create separate database for block data collection
	blockDataDBPath := filepath.Join(homePath, "data", "attestation_blocks")

	// Ensure the directory exists
	if err := os.MkdirAll(blockDataDBPath, 0755); err != nil {
		return fmt.Errorf("failed to create block data database directory: %w", err)
	}

	blockDataDB, err := dbm.NewDB("block_data", dbBackend, blockDataDBPath)
	if err != nil {
		return fmt.Errorf("failed to create block data database: %w", err)
	}

	logger.Info("Created attestation block data storage", "path", blockDataDBPath, "backend", dbBackend)

	// Create block data collector without RPC client (set later)
	collector := attestationcollector.NewBlockDataCollector(
		app.appCodec,
		blockDataDB,
		nil,    // RPC client will be set when starting the collector
		logger, // Pass logger for proper logging
	)

	// Set collector in keeper
	app.AttestationKeeper.SetBlockCollector(collector)

	logger.Info("Initialized attestation block data collector (will start after RPC server is ready)")

	return nil
}

// startBlockDataCollectorWithRetry starts the block data collector with retry logic
// This is called in a goroutine after ABCI handshake to wait for RPC server availability
func (app *App) startBlockDataCollectorWithRetry() {
	maxRetries := 10
	retryDelay := 500 * time.Millisecond

	app.Logger().Debug("Starting block data collector with retry",
		"rpc_address", app.rpcAddress,
		"max_retries", maxRetries)

	for attempt := 1; attempt <= maxRetries; attempt++ {
		app.Logger().Debug("Attempting to start block data collector",
			"attempt", attempt,
			"max_retries", maxRetries)

		err := app.startBlockDataCollectorOnce()
		if err == nil {
			app.Logger().Info("Successfully started attestation block data collector",
				"rpc_address", app.rpcAddress,
				"attempt", attempt)
			return
		}

		app.Logger().Debug("Failed to start block data collector, will retry",
			"attempt", attempt,
			"error", err,
			"retry_delay", retryDelay)

		time.Sleep(retryDelay)
		retryDelay *= 2 // Exponential backoff
	}

	app.Logger().Warn("Failed to start block data collector after all retries",
		"rpc_address", app.rpcAddress,
		"max_retries", maxRetries)
}

// startBlockDataCollectorOnce attempts to start the collector once
func (app *App) startBlockDataCollectorOnce() error {
	if app.AttestationKeeper.BlockCollector == nil {
		return fmt.Errorf("block collector not initialized")
	}

	// Type assert to get concrete collector type
	collector, ok := app.AttestationKeeper.BlockCollector.(*attestationcollector.BlockDataCollector)
	if !ok {
		return fmt.Errorf("block collector is not a BlockDataCollector")
	}

	// Check if already running
	if collector.IsRunning() {
		return nil // Already started
	}

	// Create CometBFT HTTP client
	rpcClient, err := rpchttp.New(app.rpcAddress, "/websocket")
	if err != nil {
		return fmt.Errorf("failed to create CometBFT RPC client: %w", err)
	}

	// Start the RPC client before using it
	if err := rpcClient.Start(); err != nil {
		return fmt.Errorf("failed to start CometBFT RPC client: %w", err)
	}

	// Set the client
	collector.SetClient(rpcClient)

	// Start the collector
	ctx := context.Background()
	if err := collector.Start(ctx); err != nil {
		rpcClient.Stop() // Clean up on failure
		return fmt.Errorf("failed to start block collector: %w", err)
	}

	return nil
}

// checkFinalityDatabaseExists checks if a finality database already exists
// and whether it matches the configured backend type
// Returns: (dbExists bool, backendMatches bool)
func checkFinalityDatabaseExists(dbPath string, configuredBackend dbm.BackendType) (bool, bool) {
	dbDir := filepath.Join(dbPath, "finality.db")

	// Check if directory exists
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		return false, false
	}

	// Check if directory is empty
	entries, err := os.ReadDir(dbDir)
	if err != nil || len(entries) == 0 {
		return false, false
	}

	// Database exists, now check if backend type matches
	// LevelDB and RocksDB both use CURRENT file but have different structures
	currentFile := filepath.Join(dbDir, "CURRENT")
	if _, err := os.Stat(currentFile); err != nil {
		// No CURRENT file means unknown/corrupted database
		return true, false
	}

	// Check for backend-specific indicators
	backendMatches := false

	switch configuredBackend {
	case dbm.GoLevelDBBackend:
		// LevelDB typically has .ldb files
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".ldb" {
				backendMatches = true
				break
			}
		}
		// Also check for .log files (LevelDB write-ahead log)
		if !backendMatches {
			for _, entry := range entries {
				if filepath.Ext(entry.Name()) == ".log" {
					backendMatches = true
					break
				}
			}
		}

	case dbm.RocksDBBackend:
		// RocksDB typically has .sst files
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".sst" {
				backendMatches = true
				break
			}
		}
		// Also check for OPTIONS file (RocksDB specific)
		if !backendMatches {
			for _, entry := range entries {
				if entry.Name() == "OPTIONS" || entry.Name() == "OPTIONS-" {
					backendMatches = true
					break
				}
			}
		}
	}

	return true, backendMatches
}

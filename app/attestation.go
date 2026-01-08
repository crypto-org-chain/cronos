package app

import (
	"fmt"
	"os"
	"path/filepath"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/spf13/cast"
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
			return fmt.Errorf("failed to cleanup mismatched database: %w", err)
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

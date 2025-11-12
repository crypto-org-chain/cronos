package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/crypto-org-chain/cronos/cmd/cronosd/dbmigrate"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/server"
)

const (
	flagPatchSourceBackend = "source-backend"
	flagPatchTargetBackend = "target-backend"
	flagPatchSourceHome    = "source-home"
	flagPatchTargetPath    = "target-path"
	flagPatchDatabase      = "database"
	flagPatchHeight        = "height"
	flagPatchBatchSize     = "batch-size"
	flagPatchDryRun        = "dry-run"
)

// PatchDBCmd returns the legacy patchdb command (for backward compatibility)
func PatchDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "patchdb",
		Short:      "Patch specific block heights from source database into target database",
		Deprecated: "Use 'database patch' or 'db patch' instead",
		Long: `Patch specific block heights from a source database into an existing target database.

This command is designed for:
  - Adding missing blocks to an existing database
  - Backfilling specific heights
  - Patching gaps in block data
  - Copying individual blocks between databases

Unlike migrate-db which creates a new database, patchdb UPDATES an existing target database
by adding or overwriting keys for the specified heights.

Supported databases:
  - blockstore: Block data (headers, commits, evidence)
  - tx_index: Transaction indexing
  - Multiple: blockstore,tx_index (comma-separated for both)

Height specification (--height):
  - Range: --height 10000-20000 (patch heights 10000 to 20000)
  - Single: --height 123456 (patch only height 123456)
  - Multiple: --height 123456,234567,999999 (patch specific heights)

IMPORTANT:
  - The target database MUST already exist
  - Source database is opened in read-only mode
  - Target database will be modified (keys added/updated)
  - Always backup your target database before patching
  - You MUST specify --target-path explicitly (required flag to prevent accidental modification of source database)

Examples:
  # Patch a single missing block
  cronosd patchdb \
    --database blockstore \
    --height 123456 \
    --source-home ~/.cronos-archive \
    --target-path ~/.cronos/data/blockstore.db \
    --source-backend rocksdb \
    --target-backend rocksdb

  # Patch a range of blocks
  cronosd patchdb \
    --database blockstore \
    --height 1000000-1001000 \
    --source-home ~/.cronos-backup \
    --target-path /mnt/data/cronos/blockstore.db \
    --source-backend goleveldb \
    --target-backend rocksdb

  # Patch multiple specific blocks
  cronosd patchdb \
    --database tx_index \
    --height 100000,200000,300000 \
    --source-home ~/.cronos-old \
    --target-path ~/.cronos/data/tx_index.db

  # Patch both blockstore and tx_index at once
  cronosd patchdb \
    --database blockstore,tx_index \
    --height 1000000-1001000 \
    --source-home ~/.cronos-backup \
    --target-path ~/.cronos/data \
    --source-backend goleveldb \
    --target-backend rocksdb

  # Patch from different backend
  cronosd patchdb \
    --database blockstore \
    --height 5000000-5001000 \
    --source-home /backup/cronos \
    --target-path /production/cronos/data/blockstore.db \
    --source-backend goleveldb \
    --target-backend rocksdb
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := server.GetServerContextFromCmd(cmd)
			logger := ctx.Logger

			sourceBackend := ctx.Viper.GetString(flagPatchSourceBackend)
			targetBackend := ctx.Viper.GetString(flagPatchTargetBackend)
			sourceHome := ctx.Viper.GetString(flagPatchSourceHome)
			targetPath := ctx.Viper.GetString(flagPatchTargetPath)
			databases := ctx.Viper.GetString(flagPatchDatabase)
			heightFlag := ctx.Viper.GetString(flagPatchHeight)
			batchSize := ctx.Viper.GetInt(flagPatchBatchSize)
			dryRun := ctx.Viper.GetBool(flagPatchDryRun)

			// Validate required flags
			if sourceHome == "" {
				return fmt.Errorf("--source-home is required")
			}
			if databases == "" {
				return fmt.Errorf("--database is required (blockstore, tx_index, or both comma-separated)")
			}
			if heightFlag == "" {
				return fmt.Errorf("--height is required (specify which heights to patch)")
			}
			if targetPath == "" {
				return fmt.Errorf("--target-path is required: you must explicitly specify the target database path to prevent accidental modification of the source database")
			}

			// Parse database names (comma-separated)
			dbNames := strings.Split(databases, ",")
			var validDBNames []string
			for _, db := range dbNames {
				db = strings.TrimSpace(db)
				if db == "" {
					continue
				}
				// Validate database
				if db != "blockstore" && db != "tx_index" {
					return fmt.Errorf("invalid database: %s (must be: blockstore or tx_index)", db)
				}
				validDBNames = append(validDBNames, db)
			}

			if len(validDBNames) == 0 {
				return fmt.Errorf("no valid databases specified")
			}

			// Parse backend types
			sourceBackendType, err := parseBackendType(sourceBackend)
			if err != nil {
				return fmt.Errorf("invalid source backend: %w", err)
			}

			targetBackendType, err := parseBackendType(targetBackend)
			if err != nil {
				return fmt.Errorf("invalid target backend: %w", err)
			}

			// Parse height specification
			heightRange, err := dbmigrate.ParseHeightFlag(heightFlag)
			if err != nil {
				return fmt.Errorf("invalid height specification: %w", err)
			}

			// Validate height range
			if err := heightRange.Validate(); err != nil {
				return fmt.Errorf("invalid height specification: %w", err)
			}

			if heightRange.IsEmpty() {
				return fmt.Errorf("height specification is required (cannot patch all heights)")
			}

			logger.Info("Database patch configuration",
				"databases", strings.Join(validDBNames, ", "),
				"source_home", sourceHome,
				"source_backend", sourceBackend,
				"target_backend", targetBackend,
				"height", heightRange.String(),
				"batch_size", batchSize,
			)

			// Prepare RocksDB options if target is RocksDB
			var rocksDBOpts interface{}
			if targetBackendType == dbm.RocksDBBackend {
				rocksDBOpts = dbmigrate.PrepareRocksDBOptions()
			}

			// Track aggregate statistics
			var totalKeysPatched int64
			var totalErrors int64
			var totalDuration time.Duration

			// Patch each database
			for _, dbName := range validDBNames {
				// Determine target path
				var dbTargetPath string
				// For single DB: targetPath is the full DB path (e.g., ~/.cronos/data/blockstore.db)
				// For multiple DBs: targetPath is the data directory (e.g., ~/.cronos/data)
				if len(validDBNames) == 1 {
					dbTargetPath = targetPath
				} else {
					// For multiple databases, treat targetPath as data directory
					dbTargetPath = filepath.Join(targetPath, dbName+".db")
				}

				cleanTargetPath := filepath.Clean(dbTargetPath)
				if filepath.Ext(cleanTargetPath) != ".db" {
					return fmt.Errorf("--target-path must reference a *.db directory (got %q)", dbTargetPath)
				}

				logger.Info("Patching database",
					"database", dbName,
					"target_path", dbTargetPath,
				)

				// Perform the patch operation
				opts := dbmigrate.PatchOptions{
					SourceHome:         sourceHome,
					TargetPath:         dbTargetPath,
					SourceBackend:      sourceBackendType,
					TargetBackend:      targetBackendType,
					BatchSize:          batchSize,
					Logger:             logger,
					RocksDBOptions:     rocksDBOpts,
					DBName:             dbName,
					HeightRange:        heightRange,
					ConflictStrategy:   dbmigrate.ConflictAsk, // Ask user for each conflict
					SkipConflictChecks: false,                 // Enable conflict checking
					DryRun:             dryRun,                // Dry run mode
				}

				stats, err := dbmigrate.PatchDatabase(opts)
				if err != nil {
					if stats != nil {
						logger.Error("Patch failed",
							"database", dbName,
							"error", err,
							"processed_keys", stats.ProcessedKeys.Load(),
							"duration", stats.Duration(),
						)
					} else {
						logger.Error("Patch failed",
							"database", dbName,
							"error", err,
						)
					}
					return fmt.Errorf("failed to patch %s: %w", dbName, err)
				}

				logger.Info("Database patch completed",
					"database", dbName,
					"total_keys", stats.TotalKeys.Load(),
					"processed_keys", stats.ProcessedKeys.Load(),
					"errors", stats.ErrorCount.Load(),
					"duration", stats.Duration(),
				)

				// Accumulate statistics
				totalKeysPatched += stats.ProcessedKeys.Load()
				totalErrors += stats.ErrorCount.Load()
				totalDuration += stats.Duration()
			}

			// Print summary
			fmt.Println("\n" + strings.Repeat("=", 80))
			if dryRun {
				fmt.Println("DATABASE PATCH DRY RUN COMPLETED")
			} else {
				fmt.Println("DATABASE PATCH COMPLETED SUCCESSFULLY")
			}
			fmt.Println(strings.Repeat("=", 80))
			if dryRun {
				fmt.Println("Mode:           DRY RUN (no changes made)")
			}
			fmt.Printf("Databases:      %s\n", strings.Join(validDBNames, ", "))
			fmt.Printf("Height:         %s\n", heightRange.String())
			if dryRun {
				fmt.Printf("Keys Found:     %d\n", totalKeysPatched)
			} else {
				fmt.Printf("Keys Patched:   %d\n", totalKeysPatched)
			}
			fmt.Printf("Errors:         %d\n", totalErrors)
			fmt.Printf("Total Duration: %s\n", totalDuration)
			if dryRun {
				fmt.Println("\nThis was a dry run. No changes were made to the target database(s).")
			} else {
				fmt.Println("\nThe target database(s) have been updated with the specified heights.")
			}
			fmt.Println(strings.Repeat("=", 80))

			return nil
		},
	}

	cmd.Flags().StringP(flagPatchSourceBackend, "s", "goleveldb", "Source database backend type (goleveldb, rocksdb)")
	cmd.Flags().StringP(flagPatchTargetBackend, "t", "rocksdb", "Target database backend type (goleveldb, rocksdb)")
	cmd.Flags().StringP(flagPatchSourceHome, "f", "", "Source home directory (required)")
	cmd.Flags().StringP(flagPatchTargetPath, "p", "", "Target path: for single DB (e.g., ~/.cronos/data/blockstore.db), for multiple DBs (e.g., ~/.cronos/data) (required)")
	cmd.Flags().StringP(flagPatchDatabase, "d", "", "Database(s) to patch: blockstore, tx_index, or both comma-separated (e.g., blockstore,tx_index) (required)")
	cmd.Flags().StringP(flagPatchHeight, "H", "", "Height specification: range (10000-20000), single (123456), or multiple (123456,234567) (required)")
	cmd.Flags().IntP(flagPatchBatchSize, "b", dbmigrate.DefaultBatchSize, "Number of key-value pairs to process in a batch")
	cmd.Flags().BoolP(flagPatchDryRun, "n", false, "Dry run mode: simulate the operation without making any changes")

	// Mark required flags
	_ = cmd.MarkFlagRequired(flagPatchSourceHome)
	_ = cmd.MarkFlagRequired(flagPatchTargetPath)
	_ = cmd.MarkFlagRequired(flagPatchDatabase)
	_ = cmd.MarkFlagRequired(flagPatchHeight)

	return cmd
}

// PatchCmd returns the patch subcommand (for database command group)
func PatchCmd() *cobra.Command {
	cmd := PatchDBCmd()
	cmd.Use = "patch"
	cmd.Deprecated = ""
	return cmd
}

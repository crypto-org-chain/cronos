package cmd

import (
	"fmt"
	"strings"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/crypto-org-chain/cronos/v2/cmd/cronosd/dbmigrate"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
)

const (
	flagSourceBackend = "source-backend"
	flagTargetBackend = "target-backend"
	flagTargetHome    = "target-home"
	flagBatchSize     = "batch-size"
	flagVerify        = "verify"
	flagDBType        = "db-type"
)

// Database type constants
const (
	DBTypeApp      = "app"
	DBTypeCometBFT = "cometbft"
	DBTypeAll      = "all"
)

// MigrateDBCmd returns a command to migrate database from one backend to another
func MigrateDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate-db",
		Short: "Migrate databases from one backend to another (e.g., leveldb to rocksdb)",
		Long: `Migrate databases from one backend to another.

This command migrates databases from a source backend to a target backend.
It can migrate the application database, CometBFT databases, or both.

The migration process:
1. Opens the source database(s) in read-only mode
2. Creates new temporary target database(s)
3. Copies all key-value pairs in batches
4. Optionally verifies the migration
5. Creates the target database(s) in a temporary location

Database types:
  - app: Application database only (application.db)
  - cometbft: CometBFT databases only (blockstore.db, state.db, tx_index.db, evidence.db)
  - all: Both application and CometBFT databases

IMPORTANT: 
- Always backup your databases before migration
- The source databases are opened in read-only mode and are not modified
- The target databases are created with a .migrate-temp suffix
- After successful migration, you need to manually replace the original databases
- Stop your node before running this command

Examples:
  # Migrate application database only
  cronosd migrate-db --source-backend goleveldb --target-backend rocksdb --db-type app --home ~/.cronos

  # Migrate CometBFT databases only
  cronosd migrate-db --source-backend goleveldb --target-backend rocksdb --db-type cometbft --home ~/.cronos

  # Migrate all databases
  cronosd migrate-db --source-backend goleveldb --target-backend rocksdb --db-type all --home ~/.cronos

  # Migrate with verification
  cronosd migrate-db --source-backend goleveldb --target-backend rocksdb --db-type all --verify --home ~/.cronos
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := server.GetServerContextFromCmd(cmd)
			logger := ctx.Logger

			homeDir := ctx.Viper.GetString(flags.FlagHome)
			sourceBackend := ctx.Viper.GetString(flagSourceBackend)
			targetBackend := ctx.Viper.GetString(flagTargetBackend)
			targetHome := ctx.Viper.GetString(flagTargetHome)
			batchSize := ctx.Viper.GetInt(flagBatchSize)
			verify := ctx.Viper.GetBool(flagVerify)
			dbType := ctx.Viper.GetString(flagDBType)

			// Parse backend types
			sourceBackendType, err := parseBackendType(sourceBackend)
			if err != nil {
				return fmt.Errorf("invalid source backend: %w", err)
			}

			targetBackendType, err := parseBackendType(targetBackend)
			if err != nil {
				return fmt.Errorf("invalid target backend: %w", err)
			}

			if sourceBackendType == targetBackendType {
				return fmt.Errorf("source and target backends must be different")
			}

			if targetHome == "" {
				targetHome = homeDir
			}

			// Validate db-type
			if dbType != DBTypeApp && dbType != DBTypeCometBFT && dbType != DBTypeAll {
				return fmt.Errorf("invalid db-type: %s (must be: app, cometbft, or all)", dbType)
			}

			logger.Info("Database migration configuration",
				"source_home", homeDir,
				"target_home", targetHome,
				"source_backend", sourceBackend,
				"target_backend", targetBackend,
				"db_type", dbType,
				"batch_size", batchSize,
				"verify", verify,
			)

			// Prepare RocksDB options if target is RocksDB
			var rocksDBOpts interface{}
			if targetBackendType == dbm.RocksDBBackend {
				// Use the same RocksDB options as the application (implemented in build-tagged files)
				rocksDBOpts = prepareRocksDBOptions()
			}

			// Determine which databases to migrate
			var dbNames []string
			switch dbType {
			case DBTypeApp:
				dbNames = []string{"application"}
			case DBTypeCometBFT:
				dbNames = []string{"blockstore", "state", "tx_index", "evidence"}
			case DBTypeAll:
				dbNames = []string{"application", "blockstore", "state", "tx_index", "evidence"}
			}

			// Migrate each database
			var totalStats dbmigrate.MigrationStats
			for _, dbName := range dbNames {
				logger.Info("Starting migration", "database", dbName)

				opts := dbmigrate.MigrateOptions{
					SourceHome:     homeDir,
					TargetHome:     targetHome,
					SourceBackend:  sourceBackendType,
					TargetBackend:  targetBackendType,
					BatchSize:      batchSize,
					Logger:         logger,
					RocksDBOptions: rocksDBOpts,
					Verify:         verify,
					DBName:         dbName,
				}

				stats, err := dbmigrate.Migrate(opts)
				if err != nil {
					logger.Error("Migration failed",
						"database", dbName,
						"error", err,
						"processed_keys", stats.ProcessedKeys.Load(),
						"total_keys", stats.TotalKeys.Load(),
						"duration", stats.Duration(),
					)
					return fmt.Errorf("failed to migrate %s: %w", dbName, err)
				}

				logger.Info("Database migration completed",
					"database", dbName,
					"total_keys", stats.TotalKeys.Load(),
					"processed_keys", stats.ProcessedKeys.Load(),
					"errors", stats.ErrorCount.Load(),
					"duration", stats.Duration(),
				)

				// Accumulate stats
				totalStats.TotalKeys.Add(stats.TotalKeys.Load())
				totalStats.ProcessedKeys.Add(stats.ProcessedKeys.Load())
				totalStats.ErrorCount.Add(stats.ErrorCount.Load())
			}

			fmt.Println("\n" + strings.Repeat("=", 80))
			fmt.Println("ALL MIGRATIONS COMPLETED SUCCESSFULLY")
			fmt.Println(strings.Repeat("=", 80))
			fmt.Printf("Database Type:  %s\n", dbType)
			fmt.Printf("Total Keys:     %d\n", totalStats.TotalKeys.Load())
			fmt.Printf("Processed Keys: %d\n", totalStats.ProcessedKeys.Load())
			fmt.Printf("Errors:         %d\n", totalStats.ErrorCount.Load())
			fmt.Println("\nIMPORTANT NEXT STEPS:")
			fmt.Println("1. Backup your original databases")
			fmt.Println("2. Verify the migration was successful")
			fmt.Println("3. Migrated databases are located at:")
			for _, dbName := range dbNames {
				fmt.Printf("   %s/data/%s.db.migrate-temp\n", targetHome, dbName)
			}
			fmt.Println("4. Replace the original databases with the migrated ones")
			fmt.Println("5. Update your config.toml to use the new backend type")
			fmt.Println(strings.Repeat("=", 80))

			return nil
		},
	}

	cmd.Flags().String(flagSourceBackend, "goleveldb", "Source database backend type (goleveldb, rocksdb)")
	cmd.Flags().String(flagTargetBackend, "rocksdb", "Target database backend type (goleveldb, rocksdb)")
	cmd.Flags().String(flagTargetHome, "", "Target home directory (default: same as --home)")
	cmd.Flags().Int(flagBatchSize, dbmigrate.DefaultBatchSize, "Number of key-value pairs to process in a batch")
	cmd.Flags().Bool(flagVerify, true, "Verify migration by comparing source and target databases")
	cmd.Flags().String(flagDBType, DBTypeApp, "Database type to migrate: app (application.db only), cometbft (CometBFT databases only), all (both)")

	return cmd
}

// parseBackendType parses a backend type string into dbm.BackendType
func parseBackendType(backend string) (dbm.BackendType, error) {
	switch backend {
	case "goleveldb", "leveldb":
		return dbm.GoLevelDBBackend, nil
	case "rocksdb":
		return dbm.RocksDBBackend, nil
	case "pebbledb", "pebble":
		return dbm.PebbleDBBackend, nil
	case "memdb", "mem":
		return dbm.MemDBBackend, nil
	default:
		return "", fmt.Errorf("unsupported backend type: %s (supported: goleveldb, rocksdb, pebbledb, memdb)", backend)
	}
}

package cmd

import (
	"fmt"
	"strings"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/crypto-org-chain/cronos/cmd/cronosd/dbmigrate"
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
	flagDatabases     = "databases"
)

type DbType string

// Database type constants
const (
	App      DbType = "app"
	CometBFT DbType = "cometbft"
	All      DbType = "all"
)

type BackendType string

// Backend type constants
const (
	GoLevelDB BackendType = "goleveldb"
	LevelDB   BackendType = "leveldb" // Alias for goleveldb
	RocksDB   BackendType = "rocksdb"
)

// Valid database names
var validDatabaseNames = map[string]bool{
	"application": true,
	"blockstore":  true,
	"state":       true,
	"tx_index":    true,
	"evidence":    true,
}

// MigrateDBCmd returns the migrate command
func MigrateDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
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

Database types (--db-type):
  - app: Application database only (application.db)
  - cometbft: CometBFT databases only (blockstore.db, state.db, tx_index.db, evidence.db)
  - all: Both application and CometBFT databases

Specific databases (--databases):
You can also specify individual databases as a comma-separated list:
  - application: Chain state
  - blockstore: Block data
  - state: Latest state
  - tx_index: Transaction indexing
  - evidence: Misbehavior evidence

NOTE: This command performs FULL database migration (all keys).
For selective height-based patching, use 'database patch' or 'db patch' instead.

IMPORTANT: 
- Always backup your databases before migration
- The source databases are opened in read-only mode and are not modified
- The target databases are created with a .migrate-temp.db suffix (e.g., application.migrate-temp.db)
- After successful migration, you need to manually replace the original databases
- Stop your node before running this command

Examples:
  # Migrate application database only (using --db-type)
  cronosd db migrate --source-backend goleveldb --target-backend rocksdb --db-type app --home ~/.cronos

  # Migrate CometBFT databases only (using --db-type)
  cronosd db migrate --source-backend goleveldb --target-backend rocksdb --db-type cometbft --home ~/.cronos

  # Migrate all databases (using --db-type)
  cronosd db migrate --source-backend goleveldb --target-backend rocksdb --db-type all --home ~/.cronos

  # Migrate specific databases (using --databases)
  cronosd db migrate --source-backend goleveldb --target-backend rocksdb --databases blockstore,tx_index --home ~/.cronos

  # Migrate multiple specific databases
  cronosd db migrate --source-backend goleveldb --target-backend rocksdb --databases application,blockstore,state --home ~/.cronos

  # Migrate with verification
  cronosd db migrate --source-backend goleveldb --target-backend rocksdb --db-type all --verify --home ~/.cronos
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
			databases := ctx.Viper.GetString(flagDatabases)

			// Parse backend types
			sourceBackendType, err := parseBackendType(BackendType(sourceBackend))
			if err != nil {
				return fmt.Errorf("invalid source backend: %w", err)
			}

			targetBackendType, err := parseBackendType(BackendType(targetBackend))
			if err != nil {
				return fmt.Errorf("invalid target backend: %w", err)
			}

			if sourceBackendType == targetBackendType {
				return fmt.Errorf("source and target backends must be different")
			}

			if targetHome == "" {
				targetHome = homeDir
				logger.Info("Target home not specified, using source home directory", "target_home", targetHome)
			}

			// Determine which databases to migrate
			var dbNames []string

			// If --databases flag is provided, use it (takes precedence over --db-type)
			if databases != "" {
				var err error
				dbNames, err = parseDatabaseNames(databases)
				if err != nil {
					return err
				}
			} else {
				// Fall back to --db-type flag
				var err error
				dbNames, err = getDBNamesFromType(DbType(dbType))
				if err != nil {
					return err
				}
			}

			logger.Info("Database migration configuration",
				"source_home", homeDir,
				"target_home", targetHome,
				"source_backend", sourceBackend,
				"target_backend", targetBackend,
				"databases", dbNames,
				"batch_size", batchSize,
				"verify", verify,
			)

			// Prepare RocksDB options if target is RocksDB
			var rocksDBOpts interface{}
			if targetBackendType == dbm.RocksDBBackend {
				// Use the same RocksDB options as the application (implemented in build-tagged files)
				rocksDBOpts = dbmigrate.PrepareRocksDBOptions()
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
					if stats != nil {
						logger.Error("Migration failed",
							"database", dbName,
							"error", err,
							"processed_keys", stats.ProcessedKeys.Load(),
							"total_keys", stats.TotalKeys.Load(),
							"duration", stats.Duration(),
						)
					} else {
						logger.Error("Migration failed",
							"database", dbName,
							"error", err,
						)
					}
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

			logger.Info(strings.Repeat("=", 80))
			logger.Info("ALL MIGRATIONS COMPLETED SUCCESSFULLY")
			logger.Info(strings.Repeat("=", 80))

			if databases != "" {
				logger.Info("Migration summary",
					"databases", strings.Join(dbNames, ", "),
					"total_keys", totalStats.TotalKeys.Load(),
					"processed_keys", totalStats.ProcessedKeys.Load(),
					"errors", totalStats.ErrorCount.Load(),
				)
			} else {
				logger.Info("Migration summary",
					"database_type", dbType,
					"total_keys", totalStats.TotalKeys.Load(),
					"processed_keys", totalStats.ProcessedKeys.Load(),
					"errors", totalStats.ErrorCount.Load(),
				)
			}

			logger.Info("IMPORTANT NEXT STEPS:")
			logger.Info("1. Backup your original databases")
			logger.Info("2. Verify the migration was successful")
			logger.Info("3. Migrated databases are located at:")
			for _, dbName := range dbNames {
				logger.Info("   Migrated database location", "path", fmt.Sprintf("%s/data/%s.migrate-temp.db", targetHome, dbName))
			}
			logger.Info("4. Replace the original databases with the migrated ones")
			logger.Info("5. Update your config.toml to use the new backend type")
			logger.Info(strings.Repeat("=", 80))

			return nil
		},
	}

	cmd.Flags().StringP(flagSourceBackend, "s", string(GoLevelDB), "Source database backend type (goleveldb, rocksdb)")
	cmd.Flags().StringP(flagTargetBackend, "t", string(RocksDB), "Target database backend type (goleveldb, rocksdb)")
	cmd.Flags().StringP(flagTargetHome, "o", "", "Target home directory (default: same as --home)")
	cmd.Flags().IntP(flagBatchSize, "b", dbmigrate.DefaultBatchSize, "Number of key-value pairs to process in a batch")
	cmd.Flags().BoolP(flagVerify, "v", true, "Verify migration by comparing source and target databases")
	cmd.Flags().StringP(flagDBType, "y", string(App), "Database type to migrate: app (application.db only), cometbft (CometBFT databases only), all (both)")
	cmd.Flags().StringP(flagDatabases, "d", "", "Comma-separated list of specific databases to migrate (e.g., 'blockstore,tx_index'). Valid names: application, blockstore, state, tx_index, evidence. If specified, this flag takes precedence over --db-type")

	return cmd
}

// MigrateCmd returns the migrate subcommand (for database command group)
func MigrateCmd() *cobra.Command {
	cmd := MigrateDBCmd()
	cmd.Use = "migrate"
	cmd.Deprecated = ""
	return cmd
}

// parseBackendType parses a backend type into dbm.BackendType
func parseBackendType(backend BackendType) (dbm.BackendType, error) {
	switch backend {
	case GoLevelDB, LevelDB:
		return dbm.GoLevelDBBackend, nil
	case RocksDB:
		return dbm.RocksDBBackend, nil
	default:
		return "", fmt.Errorf("unsupported backend type: %s (supported: goleveldb, leveldb, rocksdb)", backend)
	}
}

// parseDatabaseNames parses a comma-separated list of database names and validates them
func parseDatabaseNames(databases string) ([]string, error) {
	if databases == "" {
		return nil, fmt.Errorf("no databases specified")
	}

	dbList := strings.Split(databases, ",")
	var dbNames []string
	for _, dbName := range dbList {
		dbName = strings.TrimSpace(dbName)
		if dbName == "" {
			continue
		}
		if !validDatabaseNames[dbName] {
			return nil, fmt.Errorf("invalid database name: %s (valid names: application, blockstore, state, tx_index, evidence)", dbName)
		}
		dbNames = append(dbNames, dbName)
	}
	if len(dbNames) == 0 {
		return nil, fmt.Errorf("no valid databases specified in --databases flag")
	}
	return dbNames, nil
}

// getDBNamesFromType returns the list of database names for a given db-type
func getDBNamesFromType(dbType DbType) ([]string, error) {
	switch dbType {
	case App:
		return []string{"application"}, nil
	case CometBFT:
		return []string{"blockstore", "state", "tx_index", "evidence"}, nil
	case All:
		return []string{"application", "blockstore", "state", "tx_index", "evidence"}, nil
	default:
		return nil, fmt.Errorf("invalid db-type: %s (must be: app, cometbft, or all)", dbType)
	}
}

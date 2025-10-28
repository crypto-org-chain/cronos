package cmd

import (
	"fmt"
	"strings"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/crypto-org-chain/cronos/v2/cmd/cronosd/dbmigrate"
	"github.com/crypto-org-chain/cronos/v2/cmd/cronosd/opendb"
	"github.com/linxGnu/grocksdb"
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
)

// MigrateDBCmd returns a command to migrate database from one backend to another
func MigrateDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate-db",
		Short: "Migrate database from one backend to another (e.g., leveldb to rocksdb)",
		Long: `Migrate database from one backend to another.

This command migrates the application database from a source backend to a target backend.
It is useful for migrating from leveldb to rocksdb or vice versa.

The migration process:
1. Opens the source database in read-only mode
2. Creates a new temporary target database
3. Copies all key-value pairs in batches
4. Optionally verifies the migration
5. Creates the target database in a temporary location

IMPORTANT: 
- Always backup your database before migration
- The source database is opened in read-only mode and is not modified
- The target database is created with a .migrate-temp suffix
- After successful migration, you need to manually replace the original database
- Stop your node before running this command

Examples:
  # Migrate from leveldb to rocksdb
  cronosd migrate-db --source-backend goleveldb --target-backend rocksdb --home ~/.cronos

  # Migrate with verification
  cronosd migrate-db --source-backend goleveldb --target-backend rocksdb --verify --home ~/.cronos

  # Migrate to a different location
  cronosd migrate-db --source-backend goleveldb --target-backend rocksdb --target-home /new/path --home ~/.cronos
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

			logger.Info("Database migration configuration",
				"source_home", homeDir,
				"target_home", targetHome,
				"source_backend", sourceBackend,
				"target_backend", targetBackend,
				"batch_size", batchSize,
				"verify", verify,
			)

			// Prepare RocksDB options if target is RocksDB
			var rocksDBOpts *grocksdb.Options
			if targetBackendType == dbm.RocksDBBackend {
				// Use the same RocksDB options as the application
				rocksDBOpts = opendb.NewRocksdbOptions(nil, false)
			}

			// Perform migration
			opts := dbmigrate.MigrateOptions{
				SourceHome:     homeDir,
				TargetHome:     targetHome,
				SourceBackend:  sourceBackendType,
				TargetBackend:  targetBackendType,
				BatchSize:      batchSize,
				Logger:         logger,
				RocksDBOptions: rocksDBOpts,
				Verify:         verify,
			}

			stats, err := dbmigrate.Migrate(opts)
			if err != nil {
				logger.Error("Migration failed",
					"error", err,
					"processed_keys", stats.ProcessedKeys.Load(),
					"total_keys", stats.TotalKeys.Load(),
					"duration", stats.Duration(),
				)
				return err
			}

			logger.Info("Migration completed successfully",
				"total_keys", stats.TotalKeys.Load(),
				"processed_keys", stats.ProcessedKeys.Load(),
				"errors", stats.ErrorCount.Load(),
				"duration", stats.Duration(),
			)

			fmt.Println("\n" + strings.Repeat("=", 80))
			fmt.Println("MIGRATION COMPLETED SUCCESSFULLY")
			fmt.Println(strings.Repeat("=", 80))
			fmt.Printf("Total Keys:     %d\n", stats.TotalKeys.Load())
			fmt.Printf("Processed Keys: %d\n", stats.ProcessedKeys.Load())
			fmt.Printf("Errors:         %d\n", stats.ErrorCount.Load())
			fmt.Printf("Duration:       %s\n", stats.Duration())
			fmt.Println("\nIMPORTANT NEXT STEPS:")
			fmt.Println("1. Backup your original database")
			fmt.Println("2. Verify the migration was successful")
			fmt.Printf("3. The migrated database is located at: %s/data/application.db.migrate-temp\n", targetHome)
			fmt.Printf("4. Replace the original database: %s/data/application.db\n", targetHome)
			fmt.Println("5. Update your app.toml to use the new backend type")
			fmt.Println(strings.Repeat("=", 80))

			return nil
		},
	}

	cmd.Flags().String(flagSourceBackend, "goleveldb", "Source database backend type (goleveldb, rocksdb)")
	cmd.Flags().String(flagTargetBackend, "rocksdb", "Target database backend type (goleveldb, rocksdb)")
	cmd.Flags().String(flagTargetHome, "", "Target home directory (default: same as --home)")
	cmd.Flags().Int(flagBatchSize, dbmigrate.DefaultBatchSize, "Number of key-value pairs to process in a batch")
	cmd.Flags().Bool(flagVerify, true, "Verify migration by comparing source and target databases")

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

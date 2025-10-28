# Database Migration Tool

This package provides a CLI tool for migrating Cronos application databases between different backend types (e.g., LevelDB to RocksDB).

## Features

- **Multiple Backend Support**: Migrate between LevelDB, RocksDB, PebbleDB, and MemDB
- **Batch Processing**: Configurable batch size for optimal performance
- **Progress Tracking**: Real-time progress reporting with statistics
- **Data Verification**: Optional post-migration verification to ensure data integrity
- **Configurable RocksDB Options**: Use project-specific RocksDB configurations
- **Safe Migration**: Creates migrated database in a temporary location to avoid data loss

## Usage

### Basic Migration

Migrate from LevelDB to RocksDB:

```bash
cronosd migrate-db \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --home ~/.cronos
```

### Migration with Verification

Enable verification to ensure data integrity:

```bash
cronosd migrate-db \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --verify \
  --home ~/.cronos
```

### Migration to Different Location

Migrate to a different directory:

```bash
cronosd migrate-db \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --target-home /mnt/new-storage \
  --home ~/.cronos
```

### Custom Batch Size

Adjust batch size for performance tuning:

```bash
cronosd migrate-db \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --batch-size 50000 \
  --home ~/.cronos
```

## Command-Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--source-backend` | Source database backend type (goleveldb, rocksdb, pebbledb, memdb) | goleveldb |
| `--target-backend` | Target database backend type (goleveldb, rocksdb, pebbledb, memdb) | rocksdb |
| `--target-home` | Target home directory (if different from source) | Same as --home |
| `--batch-size` | Number of key-value pairs to process in each batch | 10000 |
| `--verify` | Verify migration by comparing source and target databases | true |
| `--home` | Node home directory | ~/.cronos |

## Migration Process

The migration tool follows these steps:

1. **Opens Source Database** - Opens the source database in read-only mode
2. **Creates Target Database** - Creates a new database with `.migrate-temp` suffix
3. **Counts Keys** - Counts total keys for progress reporting
4. **Migrates Data** - Copies all key-value pairs in batches
5. **Verifies Data** (optional) - Compares source and target to ensure integrity
6. **Reports Statistics** - Displays migration statistics and next steps

## Important Notes

### Before Migration

1. **Backup Your Data** - Always backup your database before migration
2. **Stop Your Node** - Ensure the node is not running during migration
3. **Check Disk Space** - Ensure sufficient disk space for the new database
4. **Verify Requirements** - For RocksDB migration, ensure RocksDB is compiled (build with `-tags rocksdb`)

### After Migration

The migrated database is created with a temporary suffix to prevent accidental overwrites:

```
Original:  ~/.cronos/data/application.db
Migrated:  ~/.cronos/data/application.db.migrate-temp
```

**Manual Steps Required:**

1. Verify the migration was successful
2. Backup the original database
3. Replace the original database with the migrated one:
   ```bash
   cd ~/.cronos/data
   mv application.db application.db.backup
   mv application.db.migrate-temp application.db
   ```
4. Update `app.toml` to use the new backend type
5. Restart your node

## Examples

### Example 1: Basic LevelDB to RocksDB Migration

```bash
# Stop the node
systemctl stop cronosd

# Backup the database
cp -r ~/.cronos/data/application.db ~/.cronos/data/application.db.backup-$(date +%Y%m%d)

# Run migration
cronosd migrate-db \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --verify \
  --home ~/.cronos

# Replace the database
cd ~/.cronos/data
mv application.db application.db.old
mv application.db.migrate-temp application.db

# Update app.toml
# Change: app-db-backend = "rocksdb"

# Restart the node
systemctl start cronosd
```

### Example 2: Migration with Custom Batch Size

For slower disks or limited memory, reduce batch size:

```bash
cronosd migrate-db \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --batch-size 1000 \
  --verify \
  --home ~/.cronos
```

### Example 3: Large Database Migration

For very large databases, disable verification for faster migration:

```bash
cronosd migrate-db \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --batch-size 50000 \
  --verify=false \
  --home ~/.cronos
```

## Performance Considerations

### Batch Size

- **Small Batch (1000-5000)**: Better for limited memory, slower overall
- **Medium Batch (10000-20000)**: Balanced performance (default: 10000)
- **Large Batch (50000+)**: Faster migration, requires more memory

### Verification

- **Enabled**: Ensures data integrity but doubles migration time
- **Disabled**: Faster migration but no automatic verification
- **Recommendation**: Enable for production systems, disable for testing

### Disk I/O

- Migration speed is primarily limited by disk I/O
- SSDs provide significantly better performance than HDDs
- Consider migration during low-traffic periods

## Troubleshooting

### Migration Fails with "file not found"

Ensure the source database exists and the path is correct:

```bash
ls -la ~/.cronos/data/application.db
```

### RocksDB Build Error

RocksDB requires native libraries. Build with RocksDB support:

```bash
make build-rocksdb
```

### Verification Fails

If verification fails, check:
1. Source database wasn't modified during migration
2. Sufficient disk space for target database
3. No I/O errors in logs

### Out of Memory

Reduce batch size:

```bash
cronosd migrate-db --batch-size 1000 ...
```

## Testing

Run tests:

```bash
# Unit tests (no RocksDB required)
go test -v ./cmd/cronosd/dbmigrate/... -short

# All tests including RocksDB
go test -v -tags rocksdb ./cmd/cronosd/dbmigrate/...

# Large database tests
go test -v ./cmd/cronosd/dbmigrate/...
```

## Architecture

### Package Structure

```
cmd/cronosd/dbmigrate/
├── migrate.go              # Core migration logic
├── migrate_rocksdb.go      # RocksDB-specific functions (with build tag)
├── migrate_no_rocksdb.go   # RocksDB stubs (without build tag)
├── migrate_basic_test.go   # Tests without RocksDB
├── migrate_test.go         # Tests with RocksDB (build tag)
├── migrate_rocksdb_test.go # RocksDB-specific tests (build tag)
└── README.md              # This file
```

### Build Tags

The package uses build tags to conditionally compile RocksDB support:

- **Without RocksDB**: Basic functionality, LevelDB migrations
- **With RocksDB** (`-tags rocksdb`): Full RocksDB support

## API

### MigrateOptions

```go
type MigrateOptions struct {
    SourceHome     string              // Source home directory
    TargetHome     string              // Target home directory
    SourceBackend  dbm.BackendType     // Source database backend
    TargetBackend  dbm.BackendType     // Target database backend
    BatchSize      int                 // Batch size for processing
    Logger         log.Logger          // Logger for progress reporting
    RocksDBOptions interface{}         // RocksDB options (if applicable)
    Verify         bool                // Enable post-migration verification
}
```

### MigrationStats

```go
type MigrationStats struct {
    TotalKeys     atomic.Int64  // Total number of keys
    ProcessedKeys atomic.Int64  // Number of keys processed
    ErrorCount    atomic.Int64  // Number of errors encountered
    StartTime     time.Time     // Migration start time
    EndTime       time.Time     // Migration end time
}
```

## Contributing

When adding new features:

1. Maintain backward compatibility
2. Add tests for new functionality
3. Update documentation
4. Follow the existing code style
5. Use build tags appropriately for optional dependencies

## License

This tool is part of the Cronos project and follows the same license.


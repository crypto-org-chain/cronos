# Database Migration Tools

This package provides CLI tools for managing Cronos databases under the `database` (or `db`) command group:

- **`database migrate`** (or `db migrate`): Full database migration between backends
- **`database patch`** (or `db patch`): Patch specific block heights into existing databases

> **Alias**: You can use `cronosd database` or `cronosd db` interchangeably.  
> **Short Flags**: All flags have short alternatives (e.g., `-s`, `-t`, `-d`, `-H`)

## database migrate: Full Database Migration

The `database migrate` command is used for migrating entire databases between different backend types (e.g., LevelDB to RocksDB).

### Features

- **Multiple Database Support**: Migrate application and/or CometBFT databases
- **Multiple Backend Support**: Migrate between LevelDB, RocksDB, PebbleDB, and MemDB
- **Batch Processing**: Configurable batch size for optimal performance
- **Progress Tracking**: Real-time progress reporting with statistics
- **Data Verification**: Optional post-migration verification to ensure data integrity
- **Configurable RocksDB Options**: Use project-specific RocksDB configurations
- **Safe Migration**: Creates migrated databases in temporary locations to avoid data loss

---

## database patch: Patch Specific Heights

The `database patch` command is used for patching specific block heights from a source database into an existing target database. Unlike `database migrate`, it **updates an existing database** rather than creating a new one.

### Key Differences

| Feature | `database migrate` | `database patch` |
|---------|-------------------|------------------|
| **Purpose** | Full database migration | Patch specific heights |
| **Target** | Creates new database | Updates existing database |
| **Height Filter** | Optional | Required |
| **Supported DBs** | All databases | blockstore, tx_index only |
| **Use Case** | Moving entire database | Adding/fixing specific blocks |
| **Key Format** | All backends | String-encoded heights (CometBFT) |

### Use Cases

- **Adding missing blocks** to an existing database
- **Backfilling specific heights** from an archive node
- **Fixing corrupted blocks** by patching from backup
- **Selective data recovery** without full resync

### Quick Example

```bash
# Patch a single missing block (with short flags)
cronosd database patch \
  -d blockstore \
  -H 123456 \
  -f ~/.cronos-archive \
  -p ~/.cronos/data/blockstore.db

# Patch a range of blocks
cronosd db patch \
  -d blockstore \
  -H 1000000-2000000 \
  -f ~/backup/cronos \
  -p ~/.cronos/data/blockstore.db

# Patch both blockstore and tx_index at once
cronosd db patch \
  -d blockstore,tx_index \
  -H 1000000-2000000 \
  -f ~/backup/cronos \
  -p ~/.cronos/data

# Patch specific heights
cronosd database patch \
  --database tx_index \
  --height 100000,200000,300000 \
  --source-home ~/.cronos-old \
  --target-path ~/.cronos/data/tx_index.db
```

For detailed documentation, see **[PATCHDB.md](PATCHDB.md)**.

---

## Supported Databases

### Application Database
- **application.db** - Chain state (accounts, contracts, balances, etc.)

### CometBFT Databases
- **blockstore.db** - Block data (headers, commits, evidence)
- **state.db** - Latest state (validator sets, consensus params)
- **tx_index.db** - Transaction indexing for lookups
- **evidence.db** - Misbehavior evidence

Use the `--db-type` flag to select which databases to migrate:
- `app` (default): Application database only
- `cometbft`: CometBFT databases only
- `all`: Both application and CometBFT databases

## Usage

### Basic Migration

#### Migrate Application Database Only
```bash
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --db-type app \
  --home ~/.cronos
```

#### Migrate CometBFT Databases Only
```bash
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --db-type cometbft \
  --home ~/.cronos
```

#### Migrate All Databases
```bash
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --db-type all \
  --home ~/.cronos
```

### Migration with Verification

Enable verification to ensure data integrity:

```bash
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --db-type all \
  --verify \
  --home ~/.cronos
```

### Migration to Different Location

Migrate to a different directory:

```bash
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --target-home /mnt/new-storage \
  --home ~/.cronos
```

### Custom Batch Size

Adjust batch size for performance tuning:

```bash
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --batch-size 50000 \
  --home ~/.cronos
```

### Migrate Specific Databases

Migrate only specific databases using the `--databases` flag:

```bash
# Migrate only blockstore and tx_index databases
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --databases blockstore,tx_index \
  --home ~/.cronos

# Migrate application and state databases
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --databases application,state \
  --home ~/.cronos
```

### Migrate Specific Height Range

For `blockstore.db` and `tx_index.db`, you can specify a height range to migrate only specific blocks:

```bash
# Migrate blockstore for heights 1000000 to 2000000
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --databases blockstore \
  --start-height 1000000 \
  --end-height 2000000 \
  --home ~/.cronos

# Migrate tx_index for heights from 5000000 onwards
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --databases tx_index \
  --start-height 5000000 \
  --home ~/.cronos

# Migrate blockstore up to height 1000000
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --databases blockstore \
  --end-height 1000000 \
  --home ~/.cronos

# Migrate both blockstore and tx_index with same height range
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --databases blockstore,tx_index \
  --start-height 1000000 \
  --end-height 2000000 \
  --home ~/.cronos
```

**Note**: Height range filtering only applies to `blockstore.db` and `tx_index.db`. Other databases will ignore these flags and migrate all data.

## Command-Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--source-backend` | Source database backend type (goleveldb, rocksdb, pebbledb, memdb) | goleveldb |
| `--target-backend` | Target database backend type (goleveldb, rocksdb, pebbledb, memdb) | rocksdb |
| `--db-type` | Database type to migrate (app, cometbft, all) | app |
| `--databases` | Comma-separated list of specific databases (e.g., 'blockstore,tx_index'). Valid: application, blockstore, state, tx_index, evidence. Takes precedence over --db-type | (empty) |
| `--start-height` | Start height for migration (inclusive, 0 for from beginning). Only applies to blockstore and tx_index | 0 |
| `--end-height` | End height for migration (inclusive, 0 for to end). Only applies to blockstore and tx_index | 0 |
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

The migrated databases are created with a temporary suffix to prevent accidental overwrites:

```
Application Database:
  Original:  ~/.cronos/data/application.db
  Migrated:  ~/.cronos/data/application.db.migrate-temp

CometBFT Databases:
  Original:  ~/.cronos/data/blockstore.db
  Migrated:  ~/.cronos/data/blockstore.db.migrate-temp
  (same pattern for state, tx_index, evidence)
```

**Manual Steps Required:**

1. Verify the migration was successful
2. Replace the original databases with the migrated ones

   **Option A: Using the swap script (recommended):**
   ```bash
   # Preview changes
   ./cmd/cronosd/dbmigrate/swap-migrated-db.sh \
     --home ~/.cronos \
     --db-type all \
     --dry-run
   
   # Perform swap with automatic backup
   ./cmd/cronosd/dbmigrate/swap-migrated-db.sh \
     --home ~/.cronos \
     --db-type all
   ```
   
   **Option B: Manual replacement:**
   ```bash
   cd ~/.cronos/data
   
   # For application database
   mv application.db application.db.backup
   mv application.db.migrate-temp application.db
   
   # For CometBFT databases (if migrated)
   for db in blockstore state tx_index evidence; do
     if [ -d "${db}.db.migrate-temp" ]; then
       mv ${db}.db ${db}.db.backup
       mv ${db}.db.migrate-temp ${db}.db
     fi
   done
   ```

3. Update configuration files:
   - `app.toml`: Set `app-db-backend` to new backend type
   - `config.toml`: Set `db_backend` to new backend type (if CometBFT databases were migrated)
4. Restart your node

## Examples

### Example 1: Basic LevelDB to RocksDB Migration

```bash
# Stop the node
systemctl stop cronosd

# Backup the database
cp -r ~/.cronos/data/application.db ~/.cronos/data/application.db.backup-$(date +%Y%m%d)

# Run migration
cronosd database migrate \
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

### Example 2: Migrate All Databases (with Swap Script)

For a complete migration of all node databases using the automated swap script:

```bash
# Stop the node
systemctl stop cronosd

# Run migration
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --db-type all \
  --verify \
  --home ~/.cronos

# Use the swap script to replace databases (includes automatic backup)
./cmd/cronosd/dbmigrate/swap-migrated-db.sh \
  --home ~/.cronos \
  --db-type all

# Update config files
# Edit app.toml: app-db-backend = "rocksdb"
# Edit config.toml: db_backend = "rocksdb"

# Restart the node
systemctl start cronosd
```

### Example 2b: Migrate All Databases (Manual Method)

For a complete migration with manual database replacement:

```bash
# Stop the node
systemctl stop cronosd

# Backup all databases
cd ~/.cronos/data
for db in application blockstore state tx_index evidence; do
  if [ -d "${db}.db" ]; then
    cp -r ${db}.db ${db}.db.backup-$(date +%Y%m%d)
  fi
done

# Run migration
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --db-type all \
  --verify \
  --home ~/.cronos

# Replace the databases
cd ~/.cronos/data
mkdir -p backups
for db in application blockstore state tx_index evidence; do
  if [ -d "${db}.db" ]; then
    mv ${db}.db backups/
    mv ${db}.db.migrate-temp ${db}.db
  fi
done

# Update config files
# Edit app.toml: app-db-backend = "rocksdb"
# Edit config.toml: db_backend = "rocksdb"

# Restart the node
systemctl start cronosd
```

### Example 3: Migration with Custom Batch Size

For slower disks or limited memory, reduce batch size:

```bash
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --db-type all \
  --batch-size 1000 \
  --verify \
  --home ~/.cronos
```

### Example 4: Migrate Specific Databases

Migrate only the databases you need:

```bash
# Migrate only transaction indexing and block storage
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --databases tx_index,blockstore \
  --verify \
  --home ~/.cronos

# Manually replace the databases
cd ~/.cronos/data
mv tx_index.db tx_index.db.backup
mv tx_index.db.migrate-temp tx_index.db
mv blockstore.db blockstore.db.backup
mv blockstore.db.migrate-temp blockstore.db

# Update config.toml: db_backend = "rocksdb"
```

### Example 5: Migrate Specific Height Range

Migrate only specific heights from blockstore and tx_index:

```bash
# Stop the node
systemctl stop cronosd

# Backup databases
cp -r ~/.cronos/data/blockstore.db ~/.cronos/data/blockstore.db.backup-$(date +%Y%m%d)
cp -r ~/.cronos/data/tx_index.db ~/.cronos/data/tx_index.db.backup-$(date +%Y%m%d)

# Migrate heights 1000000 to 2000000
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --databases blockstore,tx_index \
  --start-height 1000000 \
  --end-height 2000000 \
  --verify \
  --home ~/.cronos

# The migrated data will be in:
# ~/.cronos/data/blockstore.db.migrate-temp (only heights 1000000-2000000)
# ~/.cronos/data/tx_index.db.migrate-temp (only heights 1000000-2000000)
```

**Use Cases for Height Range Migration:**
- Pruning old blocks: Migrate only recent heights
- Testing: Migrate a subset of data for testing
- Archival: Separate old and new data into different storage backends
- Partial migration: Migrate data incrementally

### Example 6: Large Database Migration

For very large databases, disable verification for faster migration:

```bash
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --db-type all \
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
cronosd database migrate --batch-size 1000 ...
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
    DBName         string              // Database name (application, blockstore, state, tx_index, evidence)
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

## Height Filtering Feature

### Overview

Both `database migrate` and `database patch` support height-based filtering for `blockstore` and `tx_index` databases. This allows you to:

- Migrate or patch only specific block heights
- Efficiently process ranges without scanning entire database
- Handle single blocks or multiple specific heights

### Height Specification Format

The `--height` flag supports three formats:

1. **Range**: `1000000-2000000` - Continuous range (inclusive)
2. **Single**: `123456` - One specific height
3. **Multiple**: `100000,200000,300000` - Comma-separated heights

### Bounded Iterator Optimization

Height filtering uses **bounded database iterators** for maximum efficiency:

#### Traditional Approach (Inefficient)
```
Open iterator for entire database
For each key:
  Extract height
  If height in range:
    Process key
  Else:
    Skip key
```
- Reads ALL keys from disk
- Filters at application level
- Slow for large databases with small ranges

#### Bounded Iterator Approach (Efficient)
```
Calculate start_key for start_height
Calculate end_key for end_height
Open iterator with bounds [start_key, end_key)
For each key:
  Process key (all keys are in range)
```
- Only reads relevant keys from disk
- Database-level filtering
- Performance scales with range size, not total DB size

### Performance Comparison

Example: Patching heights 1M-1.1M from a 5M block database

| Approach | Keys Read | Disk I/O | Time |
|----------|-----------|----------|------|
| **Full Scan + Filter** | 5,000,000 | All blocks | ~2 hours |
| **Bounded Iterator** | 100,000 | Only range | ~3 minutes |
| **Improvement** | **50x fewer** | **98% less** | **40x faster** |

### CometBFT Key Formats

#### Blockstore Keys

**Cronos CometBFT uses STRING-ENCODED heights in blockstore keys:**

```
H:<height>         - Block metadata (height as string)
P:<height>:<part>  - Block parts (height as string, part as number)
C:<height>         - Commit at height (height as string)
SC:<height>        - Seen commit (height as string)
BH:<hash>          - Block header by hash (no height)
BS:H               - Block store height (metadata, no height encoding)
```

Example keys for height 38307809:
```
H:38307809         # Block metadata
P:38307809:0       # Block parts (part 0)
C:38307809         # Commit
SC:38307809        # Seen commit
BH:0362b5c81d...   # Block header by hash
```

> **Important**: Unlike standard CometBFT, Cronos uses **ASCII string-encoded heights**, not binary encoding.

#### TX Index Keys

Transaction index has two types of keys:

**1. Height-indexed keys:**
```
tx.height/<height>/<index>
```
- **Key format**: Height and sequential index
- **Value**: The transaction hash (txhash)

**2. Direct hash lookup keys:**
```
<txhash>
```
- **Key format**: The transaction hash itself
- **Value**: Transaction result data (protobuf-encoded)

**Important**: When patching by height, both key types are automatically patched using a two-pass approach:

**Pass 1: Height-indexed keys**
- Iterator reads `tx.height/<height>/<index>` keys within the height range
- Patches these keys to target database
- Collects txhashes from the values

**Pass 2: Txhash lookup keys**
- For each collected txhash, reads the `<txhash>` key from source
- Patches the txhash keys to target database

This ensures txhash keys (which are outside the iterator's range) are properly patched.

Example:
```
# Pass 1: Height-indexed key (from iterator)
tx.height/0001000000/0  → value: 0xABCD1234... (txhash)

# Pass 2: Direct lookup key (read individually)
0xABCD1234...  → value: <tx result data>
```

### Implementation Details

#### Blockstore Bounded Iterators

Creates separate iterators for each prefix type using string-encoded heights:

```go
// H: prefix - block metadata
startKey := []byte(fmt.Sprintf("H:%d", startHeight))    // e.g., "H:1000000"
endKey := []byte(fmt.Sprintf("H:%d", endHeight+1))      // e.g., "H:1000001"
iterator1 := db.Iterator(startKey, endKey)

// P: prefix - block parts
startKey := []byte(fmt.Sprintf("P:%d", startHeight))    // e.g., "P:1000000"
endKey := []byte(fmt.Sprintf("P:%d", endHeight+1))      // e.g., "P:1000001"
iterator2 := db.Iterator(startKey, endKey)

// ... similar for C: and SC: prefixes
```

> **Note**: Heights are encoded as ASCII strings, not binary. This is a Cronos-specific format.

**Note**: Metadata keys like `BS:H` are NOT included when using height filtering (they don't have height encoding).

#### TX Index Bounded Iterator

Single iterator with height range:

```go
startKey := []byte(fmt.Sprintf("tx.height/%010d/", startHeight))
endKey := []byte(fmt.Sprintf("tx.height/%010d/", endHeight+1))
iterator := db.Iterator(startKey, endKey)
```

#### Specific Heights Handling

For specific heights (e.g., `100,200,300`):

1. **Create encompassing range iterator**: From min(100) to max(300)
2. **Filter at application level**: Check if extracted height is in list
3. **Still efficient**: Only reads 100-300 range, not entire database

```go
// Create iterator for overall range
minHeight := 100
maxHeight := 300
iterator := db.Iterator(makeKey(minHeight), makeKey(maxHeight+1))

// Filter to specific heights
for ; iterator.Valid(); iterator.Next() {
    height := extractHeight(iterator.Key())
    if height == 100 || height == 200 || height == 300 {
        process(iterator.Key(), iterator.Value())
    }
}
```

---

## database patch Command (Detailed Documentation)

### Overview

The `database patch` command patches specific block heights from a source database into an **existing** target database.

**Key characteristics**:
- Target database MUST already exist
- Height specification is REQUIRED
- Only supports `blockstore` and `tx_index`
- Updates existing database (overwrites existing keys)

### When to Use patchdb vs migrate-db

| Scenario | Command | Reason |
|----------|---------|--------|
| **Changing database backend** | migrate-db | Creates new database with all data |
| **Missing a few blocks** | patchdb | Surgical fix, efficient for small ranges |
| **Corrupted block data** | patchdb | Replace specific bad blocks |
| **Entire database migration** | migrate-db | Handles all databases, includes verification |
| **Backfilling specific heights** | patchdb | Efficient for non-continuous heights |
| **Migrating application.db** | migrate-db | patchdb only supports blockstore/tx_index |
| **Target doesn't exist** | migrate-db | Creates new database |
| **Target exists, need additions** | patchdb | Updates existing database |

### Command Line Reference

#### Required Flags

```bash
--database <name>          # blockstore, tx_index, or blockstore,tx_index
--height <specification>   # Range, single, or multiple heights
--source-home <path>       # Source node home directory
```

#### Optional Flags

```bash
--target-path <path>       # For single DB: exact path (e.g., ~/.cronos/data/blockstore.db)
                           # For multiple DBs: data directory (e.g., ~/.cronos/data)
                           # Default: source home data directory
--source-backend <type>    # Default: goleveldb
--target-backend <type>    # Default: rocksdb
--batch-size <number>      # Default: 10000
--dry-run                  # Simulate patching without making changes
--log_level <level>        # Log level: info, debug, etc. (default: info)
```

#### Debug Logging

When using `--log_level debug`, the patch command will log detailed information about each key-value pair being patched:

```bash
# Enable debug logging to see detailed patching information
cronosd database patch \
  --database blockstore \
  --height 5000000 \
  --source-home ~/.cronos-archive \
  --target-path ~/.cronos/data/blockstore.db \
  --log_level debug
```

**Debug Output Includes**:
- **Key**: The full database key (up to 80 characters)
- **Key Size**: Size in bytes of the key
- **Value Preview**: Preview of the value (up to 100 bytes)
  - Text values: Displayed as-is
  - Binary values: Displayed as hex (e.g., `0x1a2b3c...`)
- **Value Size**: Total size in bytes of the value
- **Batch Information**: Current batch count and progress

**Example Debug Output**:
```
DBG Patched key to target database key=C:5000000 key_size=9 value_preview=0x0a8f01... value_size=143 batch_count=1
DBG Patched key to target database key=P:5000000:0 key_size=13 value_preview=0x0a4d0a... value_size=77 batch_count=2
DBG Writing batch to target database batch_size=2
```

### Detailed Examples

#### Example 1: Single Missing Block

**Scenario**: Your node is missing block 5,000,000 due to a network issue.

```bash
# 1. Stop the node
sudo systemctl stop cronosd

# 2. Backup
cp -r ~/.cronos/data/blockstore.db ~/.cronos/data/blockstore.db.backup

# 3. Patch the block
cronosd database patch \
  --database blockstore \
  --height 5000000 \
  --source-home /mnt/archive-node \
  --target-path ~/.cronos/data/blockstore.db \
  --source-backend rocksdb \
  --target-backend rocksdb

# 4. Restart
sudo systemctl start cronosd
```

#### Example 2: Range of Missing Blocks

**Scenario**: Network partition caused missing blocks 1,000,000 to 1,001,000.

```bash
cronosd database patch \
  --database blockstore \
  --height 1000000-1001000 \
  --source-home ~/backup/cronos \
  --target-path ~/.cronos/data/blockstore.db
```

#### Example 3: Multiple Checkpoint Heights

**Scenario**: Pruned node needs specific governance proposal heights.

```bash
cronosd database patch \
  --database blockstore \
  --height 1000000,2000000,3000000,4000000,5000000 \
  --source-home /archive/cronos \
  --target-path ~/.cronos/data/blockstore.db
```

#### Example 4: Cross-Backend Patching

**Scenario**: Patch from goleveldb backup to rocksdb production.

```bash
cronosd database patch \
  --database blockstore \
  --height 4500000-4600000 \
  --source-home /backup/cronos-goleveldb \
  --target-path /production/cronos/data/blockstore.db \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --batch-size 5000
```

#### Example 5: TX Index Patching

**Scenario**: Rebuild transaction index for specific heights.

```bash
cronosd database patch \
  --database tx_index \
  --height 3000000-3100000 \
  --source-home ~/.cronos-archive \
  --target-path ~/.cronos/data/tx_index.db
```

#### Example 6: Patch Both Databases at Once

**Scenario**: Missing blocks in both blockstore and tx_index (most efficient).

```bash
cronosd database patch \
  --database blockstore,tx_index \
  --height 5000000-5000100 \
  --source-home ~/.cronos-archive \
  --target-path ~/.cronos/data \
  --source-backend rocksdb \
  --target-backend rocksdb
```

**Note**: When patching multiple databases, `--target-path` should be the data directory. The command will automatically append the database name (e.g., `blockstore.db`, `tx_index.db`).

### Safety and Best Practices

#### Always Backup First

```bash
# Timestamp your backups
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

# Backup the target database
cp -r ~/.cronos/data/blockstore.db \
      ~/.cronos/data/blockstore.db.backup-$TIMESTAMP

# Verify backup
du -sh ~/.cronos/data/blockstore.db*
```

#### Stop the Node

Never patch while the node is running:

```bash
# Stop the node
sudo systemctl stop cronosd

# Verify it's stopped
ps aux | grep cronosd

# Wait for graceful shutdown
sleep 5
```

#### Verify Source Has the Data

Before patching, verify the source has the heights you need:

```bash
# For RocksDB
ldb --db=/source/blockstore.db scan --from=H: --max_keys=10

# For LevelDB
# Use leveldb tools or open the database programmatically
```

#### Monitor Progress

The `database patch` command logs progress every 5 seconds:

```
INFO  Patching progress  processed=5000 total=10000 progress=50.00% errors=0
INFO  Patching progress  processed=10000 total=10000 progress=100.00% errors=0
INFO  Database patch completed
```

#### Verify After Patching

```bash
# Start the node
sudo systemctl start cronosd

# Check node status
cronosd status

# Verify block heights
cronosd query block <height>

# Check logs for errors
journalctl -u cronosd -f
```

### Error Handling

#### Common Errors and Solutions

**1. "target database does not exist"**

```
Error: target database does not exist: /path/to/blockstore.db
```

**Solution**: Create the target database first or use `database migrate` to initialize it:

```bash
# Option 1: Use migrate-db to create empty database
cronosd database migrate --db-type cometbft --home ~/.cronos

# Option 2: Copy from another node
cp -r /other-node/data/blockstore.db ~/.cronos/data/
```

**2. "height range is required for patching"**

```
Error: height range is required for patching
```

**Solution**: Always specify `--height` flag:

```bash
cronosd database patch --height 123456 ...
```

**3. "database X does not support height-based patching"**

```
Error: database application does not support height-based patching
```

**Solution**: Use `database migrate` for non-height-encoded databases:

```bash
# For application, state, evidence databases
cronosd database migrate --db-type app ...
```

**4. "No keys found in source database for specified heights"**

```
WARN  No keys found in source database for specified heights
```

**Possible causes**:
- Source database doesn't have those heights (pruned)
- Wrong database name specified
- Incorrect source-home path

**Solution**: Verify source database content and paths.

**5. "Failed to open source database"**

```
Error: failed to open source database: <details>
```

**Solutions**:
- Check source-home path is correct
- Verify database backend type matches
- Ensure database isn't corrupted
- Check file permissions

### Performance Tuning

#### Batch Size

Adjust `--batch-size` based on your system:

| System | Recommended Batch Size | Reasoning |
|--------|------------------------|-----------|
| **HDD** | 5,000 | Slower I/O, smaller batches |
| **SSD** | 10,000 (default) | Good balance |
| **NVMe** | 20,000 | Fast I/O, larger batches |
| **Low Memory** | 1,000 | Reduce memory usage |

```bash
# For fast NVMe
cronosd database patch --batch-size 20000 ...

# For slow HDD
cronosd database patch --batch-size 5000 ...
```

#### Monitoring Performance

```bash
# Watch disk I/O during patching
iostat -x 1

# Watch memory usage
watch -n1 free -h

# Check database size
du -sh ~/.cronos/data/blockstore.db
```

### Advanced Usage

#### Patching Multiple Databases

**Option 1: Patch both at once (recommended)**

```bash
# Patch both databases in a single command
cronosd database patch \
  --database blockstore,tx_index \
  --height 1000000-2000000 \
  --source-home ~/archive \
  --target-path ~/.cronos/data
```

**Benefits**:
- Single command execution
- Consistent height range across databases
- Aggregated statistics
- Faster overall (no command overhead between runs)

**Option 2: Patch separately**

```bash
# Patch blockstore
cronosd database patch \
  --database blockstore \
  --height 1000000-2000000 \
  --source-home ~/archive \
  --target-path ~/.cronos/data/blockstore.db

# Patch tx_index for same range
cronosd database patch \
  --database tx_index \
  --height 1000000-2000000 \
  --source-home ~/archive \
  --target-path ~/.cronos/data/tx_index.db
```

**Use when**: You need different height ranges for each database.

#### Updating Block Store Height Metadata

After patching blockstore, you may need to update the height metadata:

```go
import "github.com/crypto-org-chain/cronos/v2/cmd/cronosd/dbmigrate"

// Update blockstore height to include patched blocks
err := dbmigrate.UpdateBlockStoreHeight(
    "~/.cronos/data/blockstore.db",
    dbm.RocksDBBackend,
    5000000, // new max height
    nil,     // rocksdb options
)
```

This ensures CometBFT knows about the new blocks.

### Implementation Architecture

#### Core Components

```
cmd/cronosd/cmd/patch_db.go
  └─> PatchDBCmd()                    # CLI command definition
       └─> dbmigrate.PatchDatabase()  # Core patching logic

cmd/cronosd/dbmigrate/patch.go
  ├─> PatchDatabase()                 # Main entry point
  ├─> patchDataWithHeightFilter()    # Router for database types
  ├─> patchBlockstoreData()          # Blockstore-specific patching
  ├─> patchTxIndexData()             # TX index-specific patching
  └─> patchWithIterator()            # Generic iterator processing

cmd/cronosd/dbmigrate/height_filter.go
  ├─> ParseHeightFlag()              # Parse height specification
  ├─> getBlockstoreIterators()       # Get bounded iterators
  ├─> getTxIndexIterator()           # Get bounded iterator
  └─> extractHeightFrom*Key()        # Extract height from keys
```

#### Data Flow

```
1. Parse CLI flags
2. Validate inputs (target exists, height specified, etc.)
3. Open source database (read-only)
4. Open target database (read-write)
5. Count keys to patch (using bounded iterators)
6. For each bounded iterator:
   a. Read key-value pairs
   b. Filter if specific heights
   c. Write to target in batches
   d. Log progress
7. Flush if RocksDB
8. Close databases
9. Report statistics
```

#### Memory Usage

- **Batch Size**: Default 10,000 keys
- **Per Key**: ~1KB average (blockstore), ~500B (tx_index)
- **Memory per Batch**: ~10MB (blockstore), ~5MB (tx_index)
- **Iterator State**: Minimal overhead
- **Total**: Usually < 50MB

### Limitations

#### 1. No Metadata Keys

When using bounded iterators, metadata keys (like `BS:H` in blockstore) are **not included**.

**Workaround**: Use `UpdateBlockStoreHeight()` function after patching.

#### 2. Application-Level Filtering for Specific Heights

Specific heights use encompassing range iterator + application filter.

**Impact**: Less efficient than continuous ranges, but still much better than full scan.

#### 3. No Cross-Version Support

Patching between different Cronos versions may fail if database formats differ.

**Mitigation**: Use matching versions for source and target nodes.

#### 4. No Rollback on Failure

If patching fails midway, there's no automatic rollback.

**Mitigation**: Always backup before patching. Can re-run patchdb to complete.

#### 5. Limited Database Support

Only `blockstore` and `tx_index` supported.

**Reason**: These are the only databases with height-encoded keys. Use `database migrate` for others.

### FAQ

**Q: Can I patch while the node is running?**

A: No, always stop the node first to avoid database corruption.

**Q: What happens if I patch the same heights twice?**

A: The second patch overwrites the first. The latest data wins.

**Q: Can I patch from a newer version to an older version?**

A: Not recommended. Database formats may differ between versions.

**Q: Does patchdb verify the patched data?**

A: No, patchdb doesn't have verification mode. Ensure source data is valid before patching.

**Q: Can I use patchdb for application.db?**

A: No, only blockstore and tx_index are supported. Use `database migrate` for application.db.

**Q: What if my target database doesn't exist yet?**

A: Use `database migrate` to create it first, then use `database patch` to add specific heights.

**Q: How long does patching take?**

A: Depends on the number of heights:
- Single block: seconds
- 100K range: minutes
- 1M range: tens of minutes

**Q: Can I patch from a different backend type?**

A: Yes, use `--source-backend` and `--target-backend` flags to specify different types.

---

## Contributing

When adding new features:

1. Maintain backward compatibility
2. Add tests for new functionality
3. Update documentation
4. Follow the existing code style
5. Use build tags appropriately for optional dependencies

## License

This tool is part of the Cronos project and follows the same license.


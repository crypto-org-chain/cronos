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
- **Multiple Backend Support**: Migrate between LevelDB and RocksDB
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
| **Height Filter** | Not supported | Required |
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


## Command-Line Flags (migrate)

| Flag | Description | Default |
|------|-------------|---------|
| `--source-backend` (`-s`) | Source database backend type (`goleveldb`, `rocksdb`) | goleveldb |
| `--target-backend` (`-t`) | Target database backend type (`goleveldb`, `rocksdb`) | rocksdb |
| `--db-type` (`-y`) | Database type to migrate (`app`, `cometbft`, `all`) | app |
| `--databases` (`-d`) | Comma-separated list of specific databases (e.g., `blockstore,tx_index`). Valid: `application`, `blockstore`, `state`, `tx_index`, `evidence`. Takes precedence over `--db-type` | (empty) |
| `--target-home` (`-o`) | Target home directory (if different from source) | Same as `--home` |
| `--batch-size` (`-b`) | Number of key-value pairs to process in each batch | 10000 |
| `--verify` (`-v`) | Verify migration by comparing source and target databases | true |
| `--home` | Node home directory | ~/.cronos |

**Note:** The `migrate` command performs **full database migration** without height filtering. For selective height-based operations, use the `database patch` command instead.

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

### Example 5: Large Database Migration

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
# From project root with RocksDB support
COSMOS_BUILD_OPTIONS=rocksdb make build

# Or with specific tags
go build -tags rocksdb -o ./cronosd ./cmd/cronosd
```

Note: RocksDB requires native C++ libraries to be installed on your system. On macOS, install via `brew install rocksdb`. On Ubuntu/Debian, install via `apt-get install librocksdb-dev`. For other systems, see the [RocksDB installation guide](https://github.com/facebook/rocksdb/blob/main/INSTALL.md).

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
├── patch.go                # Patching logic for specific heights
├── height_filter.go        # Height-based filtering and iterators
├── migrate_basic_test.go   # Tests without RocksDB
├── migrate_test.go         # General migration tests
├── migrate_dbname_test.go  # Database name-specific tests
├── migrate_rocksdb_test.go # RocksDB-specific tests (build tag)
├── patch_test.go           # Patching tests
├── height_parse_test.go    # Height parsing tests
├── height_filter_test.go   # Height filtering tests
├── swap-migrated-db.sh     # Script to swap databases after migration
├── README.md               # Full documentation
└── QUICKSTART.md           # Quick start guide
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
    HeightRange    HeightRange         // Height range to migrate (only for blockstore and tx_index)
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

**IMPORTANT**: Height-based filtering is **ONLY supported** by the `database patch` command, not `database migrate`.

- **`database migrate`**: Full database migration between backends (processes entire database, no filtering)
- **`database patch`**: Selective patching of specific heights to existing database (supports height filtering)

The `database patch` command supports height-based filtering for `blockstore` and `tx_index` databases, allowing you to:

- Patch only specific block heights to an existing database
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
EC:<height>        - Extended commit (height as string, ABCI 2.0)
BH:<hash>          - Block header by hash (no height, migrated via H: keys)
BS:H               - Block store height (metadata, no height encoding)
```

Example keys for height 38307809:
```
H:38307809         # Block metadata
P:38307809:0       # Block parts (part 0)
C:38307809         # Commit
SC:38307809        # Seen commit
EC:38307809        # Extended commit (ABCI 2.0, if present)
BH:0362b5c81d...   # Block header by hash (auto-migrated with H: keys)
```

> **Important**: Unlike standard CometBFT, Cronos uses **ASCII string-encoded heights**, not binary encoding.

#### TX Index Keys

Transaction index has two types of keys:

**1. Height-indexed keys:**
```
tx.height/<height>/<height>/<txindex>$es$0
tx.height/<height>/<height>/<txindex>  (without $es$ suffix)
```
- **Key format**: Height (twice) and transaction index, optionally with event sequence suffix
- **Value**: The transaction hash (txhash)

**2. Direct hash lookup keys (CometBFT):**
```
<cometbft_txhash>
```
- **Key format**: The CometBFT transaction hash itself
- **Value**: Transaction result data (protobuf-encoded)

**3. Event-indexed keys (Ethereum):**
```
ethereum_tx.ethereumTxHash/0x<eth_txhash>/<height>/<txindex>$es$<eventseq>
ethereum_tx.ethereumTxHash/0x<eth_txhash>/<height>/<txindex>  (without $es$ suffix)
```
- **Key format**: Event key + Ethereum txhash (hex, with 0x) + height + tx index, optionally with event sequence
  - `ethereum_tx.ethereumTxHash`: Event attribute key
  - `0x<eth_txhash>`: Ethereum txhash (hex, with 0x prefix)
  - `<height>`: Block height
  - `<txindex>`: Transaction index within block
  - `$es$<eventseq>`: Event sequence separator and number (optional)
- **Value**: CometBFT transaction hash (allows lookup by Ethereum txhash)
- **Purpose**: Enables `eth_getTransactionReceipt` by Ethereum txhash

**Important**: When patching by height, all three key types are automatically patched using a three-pass approach:

**Pass 1: Height-indexed keys**
- Iterator reads `tx.height/<height>/<height>/<txindex>` keys within the height range (with or without `$es$` suffix)
- Patches these keys to target database
- Collects CometBFT txhashes from the values
- **Extracts Ethereum txhashes** from transaction result events

**Pass 2: CometBFT txhash lookup keys**
- For each collected CometBFT txhash, reads the `<cometbft_txhash>` key from source
- Patches the txhash keys to target database

**Pass 3: Ethereum event-indexed keys**
- For each transaction from Pass 1, creates a bounded iterator with specific start/end keys
- Start: `ethereum_tx.ethereumTxHash/0x<eth_txhash>/<height>/<txindex>`
- End: `start + 1` (exclusive upper bound)
- Iterates only through event keys for that specific transaction (matches keys with or without `$es$` suffix)
- Patches all matching event keys to target database
- **Critical for `eth_getTransactionReceipt` to work correctly**
- **Performance**: Uses bounded iteration for optimal database range scans

This ensures all tx_index keys (including event-indexed keys) are properly patched.

Example:
```
# Pass 1: Height-indexed key (from iterator)
tx.height/1000000/1000000/0$es$0  → value: <cometbft_txhash>

# Pass 2: CometBFT direct lookup key (read individually)
<cometbft_txhash>  → value: <tx result data with events>

# Pass 3: Ethereum event-indexed key (searched from source DB)
ethereum_tx.ethereumTxHash/0xa1b2c3d4.../1000000/0$es$0  → value: <cometbft_txhash>
```

> **Note**: Pass 3 is only performed for transactions that contain `ethereum_tx` events. Non-EVM transactions (e.g., bank transfers, staking) will not have Ethereum txhashes.

### Implementation Details

#### Blockstore Bounded Iterators

**CRITICAL**: CometBFT uses **string-encoded decimal heights** (e.g., "H:100", "H:20"), which **do NOT sort lexicographically by numeric value**:
- "H:20" > "H:150" (lexically)
- "H:9" > "H:10000" (lexically)

**Solution**: We use **prefix-only iterators** with **Go-level numeric filtering** (Strategy B):

```go
// H: prefix - create prefix-only iterator
start := []byte("H:")
end := []byte("I:")  // Next prefix in ASCII
iterator1 := db.Iterator(start, end)

// For each key from iterator:
//   1. Extract height numerically from key (e.g., parse "H:12345" -> 12345)
//   2. Check if height is within range using shouldIncludeKey()
//   3. Only process keys that pass the numeric filter

// ... similar for P:, C:, SC:, and EC: prefixes
```

This strategy trades some iteration efficiency for correctness, scanning all keys with each prefix but filtering at the application level.

> **Note**: Metadata keys like `BS:H` are NOT included when using height filtering (they don't have height encoding).

**BH: Key Patching**: Block header by hash (`BH:<hash>`) keys don't contain height information. During **patching** (not full migration), when an `H:<height>` key is patched, the block hash is extracted from its value and used to look up and patch the corresponding `BH:<hash>` key automatically. For full migrations, BH: keys are included in the complete database scan.

#### TX Index Bounded Iterator

**CRITICAL**: tx_index keys use format `tx.height/{height}/{hash}` where height is a **decimal string** (not zero-padded). Like blockstore, decimal strings **do NOT sort lexicographically by numeric value**:
- "tx.height/20/" > "tx.height/150/" (lexically)
- "tx.height/9/" > "tx.height/10000/" (lexically)

**Solution**: We use a **prefix-only iterator** with **Go-level numeric filtering** (Strategy B):

```go
// Create prefix-only iterator for tx.height namespace
start := []byte("tx.height/")
end := []byte("tx.height/~")  // '~' is ASCII 126, after all digits
iterator := db.Iterator(start, end)

// For each key from iterator:
//   1. Extract height numerically from key (e.g., parse "tx.height/12345/..." -> 12345)
//   2. Check if height is within range using shouldIncludeKey()
//   3. Only process keys that pass the numeric filter
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

### When to Use patch vs migrate

| Scenario | Command | Reason |
|----------|---------|--------|
| **Changing database backend** | migrate | Creates new database with all data |
| **Missing a few blocks** | patch | Surgical fix, efficient for small ranges |
| **Corrupted block data** | patch | Replace specific bad blocks |
| **Entire database migration** | migrate | Handles all databases, includes verification |
| **Backfilling specific heights** | patch | Efficient for non-continuous heights |
| **Migrating application.db** | migrate | patch only supports blockstore/tx_index |
| **Target doesn't exist** | migrate | Creates new database |
| **Target exists, need additions** | patch | Updates existing database |

## Command-Line Flags (patch)

### Required Flags

| Flag | Description |
|------|-------------|
| `--database` (`-d`) | Database name: `blockstore`, `tx_index`, or `blockstore,tx_index` |
| `--height` (`-H`) | Height specification: range (`1000-2000`), single (`123456`), or multiple (`100,200,300`) |
| `--source-home` (`-f`) | Source node home directory |

### Optional Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--target-path` (`-p`) | For single DB: exact path (e.g., `~/.cronos/data/blockstore.db`)<br>For multiple DBs: data directory (e.g., `~/.cronos/data`) | Source home data directory |
| `--source-backend` (`-s`) | Source database backend type (`goleveldb`, `rocksdb`) | goleveldb |
| `--target-backend` (`-t`) | Target database backend type (`goleveldb`, `rocksdb`) | rocksdb |
| `--batch-size` (`-b`) | Number of key-value pairs to process in each batch | 10000 |
| `--dry-run` | Simulate patching without making changes | false |
| `--log_level` | Log level (`info`, `debug`, etc.) | info |

**Dry-Run Mode**: When using `--dry-run`, the patch command will:
- Simulate the entire patching process without writing any data
- Log all keys that would be patched (with `--log_level debug`)
- For blockstore patches, also discover and report BH: (block header by hash) keys that would be patched
- Report the total number of operations that would be performed

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
  - Text keys: Displayed as-is (e.g., `tx.height/123/0`)
  - Binary keys: Displayed as hex (e.g., `0x1a2b3c...` for txhashes)
- **Key Size**: Size in bytes of the key
- **Value Preview**: Preview of the value (up to 100 bytes)
  - Text values: Displayed as-is
  - Binary values: Displayed as hex (e.g., `0x0a8f01...`)
- **Value Size**: Total size in bytes of the value
- **Batch Information**: Current batch count and progress

**Example Debug Output**:

For blockstore keys (text):
```
DBG Patched key to target database key=C:5000000 key_size=9 value_preview=0x0a8f01... value_size=143 batch_count=1
DBG Patched key to target database key=P:5000000:0 key_size=13 value_preview=0x0a4d0a... value_size=77 batch_count=2
```

For tx_index keys:
```
# Pass 1: Height-indexed keys
DBG Patched tx.height key key=tx.height/5000000/5000000/0$es$0
DBG Collected ethereum txhash eth_txhash=0xa1b2c3d4... height=5000000 tx_index=0

# Pass 2: CometBFT txhash keys (binary)
DBG Patched txhash key txhash=0x1a2b3c4d5e6f7890abcdef1234567890abcdef1234567890abcdef1234567890

# Pass 3: Ethereum event-indexed keys (searched from source DB)
DBG Found ethereum event key in source event_key=ethereum_tx.ethereumTxHash/0xa1b2c3d4.../5000000/0$es$0
DBG Patched ethereum event key event_key=ethereum_tx.ethereumTxHash/0xa1b2c3d4.../5000000/0$es$0
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
# Option 1: Use db migrate to create empty database
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


### Limitations

#### 1. Application-Level Filtering for Specific Heights

Specific heights use encompassing range iterator + application filter.

**Impact**: Less efficient than continuous ranges, but still much better than full scan.

#### 2. No Cross-Version Support

Patching between different Cronos versions may fail if database formats differ.

**Mitigation**: Use matching versions for source and target nodes.

#### 3. No Rollback on Failure

If patching fails midway, there's no automatic rollback.

**Mitigation**: Always backup before patching. Can re-run db patch to complete.

#### 4. Limited Database Support

Only `blockstore` and `tx_index` supported.

**Reason**: These are the only databases with height-encoded keys. Use `database migrate` for others.




## License

This tool is part of the Cronos project and follows the same license.


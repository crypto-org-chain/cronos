# Database Tools - Quick Start Guide

This guide covers two commands under the `database` (or `db`) command group:
- **`database migrate`**: Full database migration between backends
- **`database patch`**: Patch specific block heights into existing databases

> **Command Aliases**: You can use `cronosd database` or `cronosd db` interchangeably.

---

## Part 1: database migrate (Full Migration)

### Overview

The `database migrate` command supports migrating:
- **Application database** (`application.db`) - Your chain state
- **CometBFT databases** (`blockstore.db`, `state.db`, `tx_index.db`, `evidence.db`) - Consensus data

### Database Selection

**Option 1: Use `--db-type` (or `-y`) flag** (migrate predefined groups):
- `app` (default): Application database only
- `cometbft`: CometBFT databases only  
- `all`: Both application and CometBFT databases

**Option 2: Use `--databases` (or `-d`) flag** (migrate specific databases):
- Comma-separated list of database names
- Valid names: `application`, `blockstore`, `state`, `tx_index`, `evidence`
- Example: `--databases blockstore,tx_index` or `-d blockstore,tx_index`
- Takes precedence over `--db-type` if both are specified

## Prerequisites

- Cronos node stopped
- Database backup created
- Sufficient disk space (at least 2x database size)
- For RocksDB: Build with `make build-rocksdb` or `-tags rocksdb`

## Basic Migration Steps

### 1. Stop Your Node

```bash
# systemd
sudo systemctl stop cronosd

# or manually
pkill cronosd
```

### 2. Backup Your Databases

```bash
# Backup application database
BACKUP_NAME="application.db.backup-$(date +%Y%m%d-%H%M%S)"
cp -r ~/.cronos/data/application.db ~/.cronos/data/$BACKUP_NAME

# If migrating CometBFT databases too
for db in blockstore state tx_index evidence; do
  cp -r ~/.cronos/data/${db}.db ~/.cronos/data/${db}.db.backup-$(date +%Y%m%d-%H%M%S)
done

# Verify backups
du -sh ~/.cronos/data/*.backup-*
```

### 3. Run Migration

#### Application Database Only (Default)
```bash
cronosd database migrate \
  -s goleveldb \
  -t rocksdb \
  -y app \
  --home ~/.cronos
```

#### CometBFT Databases Only
```bash
cronosd database migrate \
  -s goleveldb \
  -t rocksdb \
  -y cometbft \
  --home ~/.cronos
```

#### All Databases (Recommended)
```bash
cronosd database migrate \
  -s goleveldb \
  -t rocksdb \
  -y all \
  --home ~/.cronos
```

#### RocksDB to LevelDB
```bash
cronosd database migrate \
  -s rocksdb \
  -t goleveldb \
  -y all \
  --home ~/.cronos
```

#### Specific Databases Only
```bash
# Migrate only blockstore and tx_index
cronosd database migrate \
  -s goleveldb \
  -t rocksdb \
  -d blockstore,tx_index \
  --home ~/.cronos

# Migrate application and state databases
cronosd database migrate \
  -s goleveldb \
  -t rocksdb \
  -d application,state \
  --home ~/.cronos
```

### 4. Verify Migration Output

#### Single Database Migration
Look for:
```
================================================================================
MIGRATION COMPLETED SUCCESSFULLY
================================================================================
Total Keys:     1234567
Processed Keys: 1234567
Errors:         0
Duration:       5m30s
```

#### Multiple Database Migration (db-type=all)
Look for:
```
4:30PM INF Starting migration database=application
4:30PM INF Migration completed database=application processed_keys=21 total_keys=21
4:30PM INF Starting migration database=blockstore
4:30PM INF Migration completed database=blockstore processed_keys=1523 total_keys=1523
...

================================================================================
ALL MIGRATIONS COMPLETED SUCCESSFULLY
================================================================================
Database Type:  all
Total Keys:     3241
Processed Keys: 3241
Errors:         0
```

### 5. Replace Original Databases

#### Using the Swap Script (Recommended)

The easiest way to replace databases is using the provided script:

```bash
# Preview what will happen (dry run)
./cmd/cronosd/dbmigrate/swap-migrated-db.sh \
  --home ~/.cronos \
  --db-type all \
  --dry-run

# Perform the actual swap
./cmd/cronosd/dbmigrate/swap-migrated-db.sh \
  --home ~/.cronos \
  --db-type all
```

The script will:
- ✅ Create timestamped backups (using fast `mv` operation)
- ✅ Replace originals with migrated databases
- ✅ Show summary with next steps
- ⚡ Faster than copying (no disk space duplication)

**Script Options:**
```bash
--home DIR           # Node home directory (default: ~/.cronos)
--db-type TYPE       # Database type: app, cometbft, all (default: app)
--backup-suffix STR  # Custom backup name (default: backup-YYYYMMDD-HHMMSS)
--dry-run            # Preview without making changes
```

#### Manual Replacement (Alternative)

##### Application Database Only
```bash
cd ~/.cronos/data

# Keep old database as backup
mv application.db application.db.old

# Use migrated database
mv application.db.migrate-temp application.db

# Verify
ls -lh application.db
```

##### All Databases
```bash
cd ~/.cronos/data

# Backup originals
mkdir -p backups
for db in application blockstore state tx_index evidence; do
  if [ -d "${db}.db" ]; then
    mv ${db}.db backups/${db}.db.old
  fi
done

# Replace with migrated databases
for db in application blockstore state tx_index evidence; do
  if [ -d "${db}.db.migrate-temp" ]; then
    mv ${db}.db.migrate-temp ${db}.db
  fi
done

# Verify
ls -lh *.db
```

### 6. Update Configuration

#### Application Database
Edit `~/.cronos/config/app.toml`:

```toml
# Change from:
app-db-backend = "goleveldb"

# To:
app-db-backend = "rocksdb"
```

#### CometBFT Databases
Edit `~/.cronos/config/config.toml`:

```toml
[consensus]
# Change from:
db_backend = "goleveldb"

# To:
db_backend = "rocksdb"
```

### 7. Start Node

```bash
# systemd
sudo systemctl start cronosd

# or manually
cronosd start --home ~/.cronos
```

### 8. Verify Node Health

```bash
# Check node is syncing
cronosd status

# Check logs
tail -f ~/.cronos/logs/cronos.log

# Or systemd logs
journalctl -u cronosd -f
```

## Quick Complete Workflow

For the fastest migration experience:

```bash
# 1. Stop node
systemctl stop cronosd

# 2. Run migration
cronosd database migrate \
  -s goleveldb \
  -t rocksdb \
  -y all \
  --home ~/.cronos

# 3. Swap databases (with automatic backup)
./cmd/cronosd/dbmigrate/swap-migrated-db.sh \
  --home ~/.cronos \
  --db-type all

# 4. Update configs (edit app.toml and config.toml)

# 5. Start node
systemctl start cronosd
```

## Common Options

### Migrate Specific Database Type
```bash
# Application only
cronosd database migrate -y app ...

# CometBFT only
cronosd database migrate -y cometbft ...

# All databases
cronosd database migrate -y all ...
```

### Skip Verification (Faster)
```bash
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --db-type all \
  --verify=false \
  --home ~/.cronos
```

### Custom Batch Size
```bash
# Smaller batches for low memory
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --batch-size 1000 \
  --home ~/.cronos

# Larger batches for high-end systems
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --batch-size 50000 \
  --home ~/.cronos
```

### Migrate to Different Location
```bash
# Useful for moving to faster disk
cronosd database migrate \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --target-home /mnt/nvme/cronos \
  --home ~/.cronos
```

## Troubleshooting

### Migration is Slow

**Solution 1: Increase Batch Size**
```bash
cronosd database migrate --batch-size 50000 ...
```

**Solution 2: Disable Verification**
```bash
cronosd database migrate --verify=false ...
```

### Out of Disk Space

**Check Space:**
```bash
df -h ~/.cronos/data
```

**Free Up Space:**
```bash
# Remove old snapshots
rm -rf ~/.cronos/data/snapshots/*

# Remove old backups
rm -rf ~/.cronos/data/*.old
```

### Migration Failed

**Check Logs:**
The migration tool outputs detailed progress. Look for:
- "Migration failed" error message
- Error counts > 0
- Verification failures

**Recovery:**
```bash
# Remove failed migration
rm -rf ~/.cronos/data/application.db.migrate-temp

# Restore from backup if needed
cp -r ~/.cronos/data/application.db.backup-* ~/.cronos/data/application.db

# Try again with different options
cronosd database migrate --batch-size 1000 --verify=false ...
```

### RocksDB Build Error

**Error:** `fatal error: 'rocksdb/c.h' file not found`

**Solution:** Build with RocksDB support:
```bash
# Install RocksDB dependencies (Ubuntu/Debian)
sudo apt-get install librocksdb-dev

# Or build from source
make build-rocksdb
```

## Performance Tips

### For Large Databases (> 100GB)

1. **Use SSD/NVMe** if possible
2. **Increase batch size**: `--batch-size 50000`
3. **Skip verification initially**: `--verify=false`
4. **Run during low-traffic**: Minimize disk I/O competition
5. **Verify separately**: Check a few keys manually after migration

### For Limited Memory Systems

1. **Decrease batch size**: `--batch-size 1000`
2. **Close other applications**: Free up RAM
3. **Monitor memory**: `watch -n 1 free -h`

### For Network-Attached Storage

1. **Migrate locally first**: Then copy to NAS
2. **Use small batches**: Network latency affects performance
3. **Consider rsync**: For final data transfer

## Verification

### Check Migration Success

```bash
# Count keys in original (LevelDB example)
OLD_KEYS=$(cronosd query-db-keys --backend goleveldb --home ~/.cronos | wc -l)

# Count keys in new database
NEW_KEYS=$(cronosd query-db-keys --backend rocksdb --home ~/.cronos | wc -l)

# Compare
echo "Old: $OLD_KEYS, New: $NEW_KEYS"
```

### Manual Verification

```bash
# Start node with new database
cronosd start --home ~/.cronos

# Check a few accounts
cronosd query bank balances <address>

# Check contract state
cronosd query evm code <contract-address>

# Check latest block
cronosd query block
```

## Rollback

If migration fails or node won't start:

```bash
cd ~/.cronos/data

# Remove new database
rm -rf application.db.migrate-temp application.db

# Restore backup
cp -r application.db.backup-* application.db

# Restore original app.toml settings
# Change app-db-backend back to original value

# Start node
sudo systemctl start cronosd
```

## Estimated Migration Times

### Single Database (Application)
Based on typical disk speeds:

| Database Size | HDD (100MB/s) | SSD (500MB/s) | NVMe (3GB/s) |
|--------------|---------------|---------------|--------------|
| 10 GB        | ~3 minutes    | ~30 seconds   | ~5 seconds   |
| 50 GB        | ~15 minutes   | ~2.5 minutes  | ~25 seconds  |
| 100 GB       | ~30 minutes   | ~5 minutes    | ~50 seconds  |
| 500 GB       | ~2.5 hours    | ~25 minutes   | ~4 minutes   |

*Note: Times include verification. Add 50% time for verification disabled.*

### All Databases (app + cometbft)
Multiply by approximate factor based on your database sizes:
- **Application**: Usually largest (state data)
- **Blockstore**: Medium-large (block history)
- **State**: Small-medium (latest state)
- **TX Index**: Medium-large (transaction lookups)
- **Evidence**: Small (misbehavior evidence)

**Example:** For a typical node with 100GB application.db and 50GB of CometBFT databases combined, expect ~40 minutes on SSD with verification.

## Getting Help

### Enable Verbose Logging

The migration tool already provides detailed logging. For more details:

```bash
# Check migration progress (in another terminal)
watch -n 1 'tail -n 20 ~/.cronos/migration.log'
```

### Report Issues

Include:
1. Migration command used
2. Error message
3. Database size
4. System specs (RAM, disk type)
5. Cronos version

## Success Checklist

- [ ] Node stopped
- [ ] Database backed up
- [ ] Sufficient disk space
- [ ] Migration completed successfully (0 errors)
- [ ] app.toml updated
- [ ] Original database replaced
- [ ] Node started successfully
- [ ] Node syncing normally
- [ ] Queries working correctly

## Next Steps After Migration

1. **Monitor performance**: RocksDB may perform differently
2. **Tune RocksDB**: Adjust options in code if needed
3. **Remove old backup**: After confirming stability
4. **Update documentation**: Note the backend change
5. **Update monitoring**: If tracking database metrics

---

## Part 2: database patch (Patch Specific Heights)

### Overview

The `database patch` command patches specific block heights from a source database into an **existing** target database.

**Use cases**:
- Fix missing blocks
- Repair corrupted blocks
- Backfill specific heights
- Add blocks without full resync

**Key differences from `database migrate`**:
- Target database MUST already exist
- Only patches specified heights (required)
- Only supports `blockstore` and `tx_index`
- Updates existing database (doesn't create new one)
- CometBFT uses **string-encoded heights** in keys (e.g., `C:38307809`)

### Prerequisites

- Both nodes stopped
- **Target database must exist**
- Backup of target database
- Source database with the blocks you need

### Quick Start: Patch Missing Block

#### 1. Stop Nodes

```bash
# Stop both source and target nodes
sudo systemctl stop cronosd
```

#### 2. Backup Target Database

```bash
# Always backup before patching!
BACKUP_NAME="blockstore.db.backup-$(date +%Y%m%d-%H%M%S)"
cp -r ~/.cronos/data/blockstore.db ~/.cronos/data/$BACKUP_NAME
```

#### 3. Patch the Block

**Single block**:
```bash
cronosd database patch \
  -d blockstore \
  -H 123456 \
  -f ~/.cronos-archive \
  -p ~/.cronos/data/blockstore.db
```

**Range of blocks**:
```bash
cronosd database patch \
  -d blockstore \
  -H 1000000-1001000 \
  -f ~/.cronos-archive \
  -p ~/.cronos/data/blockstore.db
```

**Multiple specific blocks**:
```bash
cronosd database patch \
  -d blockstore \
  -H 100000,200000,300000 \
  -f ~/.cronos-archive \
  -p ~/.cronos/data/blockstore.db
```

**Both databases at once** (recommended):
```bash
cronosd database patch \
  -d blockstore,tx_index \
  -H 1000000-1001000 \
  -f ~/.cronos-archive \
  -p ~/.cronos/data
```

**With debug logging** (to see detailed key/value information):
```bash
cronosd database patch \
  -d blockstore \
  -H 123456 \
  -f ~/.cronos-archive \
  -p ~/.cronos/data/blockstore.db \
  --log_level debug
```

> **Note**: Debug logs automatically format binary data (like txhashes) as hex strings (e.g., `0x1a2b3c...`) for readability, while text keys (like `tx.height/123/0`) are displayed as-is.

**Dry run** (preview without making changes):
```bash
cronosd database patch \
  -d blockstore \
  -H 123456 \
  -f ~/.cronos-archive \
  -p ~/.cronos/data/blockstore.db \
  --dry-run
```

#### 4. Verify and Restart

```bash
# Check the logs from database patch output
# Look for: "DATABASE PATCH COMPLETED SUCCESSFULLY"

# Start node
sudo systemctl start cronosd

# Verify node is working
cronosd status
```

### Common Patching Scenarios

#### Scenario 1: Missing Blocks

**Problem**: Node missing blocks 5000000-5000100

**Solution**:
```bash
cronosd database patch \
  -d blockstore \
  -H 5000000-5000100 \
  -f /mnt/archive-node \
  -p ~/.cronos/data/blockstore.db \
  -s rocksdb \
  -t rocksdb
```

#### Scenario 2: Corrupted Block

**Problem**: Block 3000000 is corrupted

**Solution**:
```bash
cronosd database patch \
  -d blockstore \
  -H 3000000 \
  -f /backup/cronos \
  -p ~/.cronos/data/blockstore.db
```

#### Scenario 3: Backfill Historical Data

**Problem**: Pruned node needs specific checkpoint heights

**Solution**:
```bash
cronosd database patch \
  -d blockstore \
  -H 1000000,2000000,3000000,4000000 \
  -f /archive/cronos \
  -p ~/.cronos/data/blockstore.db
```

#### Scenario 4: Patch Both Databases Efficiently

**Problem**: Missing blocks in both blockstore and tx_index

**Solution** (patch both at once):
```bash
cronosd database patch \
  -d blockstore,tx_index \
  -H 5000000-5000100 \
  -f /mnt/archive-node \
  -p ~/.cronos/data \
  -s rocksdb \
  -t rocksdb
```

> **Note**: When patching `tx_index` by height, the command uses a **three-pass approach**:
> 1. **Pass 1**: Patches `tx.height/<height>/<height>/<txindex>` keys (with or without `$es$` suffix) and collects transaction metadata (height, tx_index)
> 2. **Pass 2**: Patches CometBFT `<txhash>` lookup keys
> 3. **Pass 3**: For each transaction, uses a bounded iterator with range `[start, end)` where start is `ethereum_tx.ethereumTxHash/<eth_txhash>/<height>/<height>/<txindex>` and end is `start + 1`
> 
> This ensures complete transaction index functionality, including support for `eth_getTransactionReceipt` with Ethereum txhashes. Pass 3 uses bounded iteration for optimal database range scans and copies existing event keys from source DB with their exact format (with or without `$es$<eventseq>` suffix).

### Patch Flags Reference

| Flag | Short | Required | Default | Description |
|------|-------|----------|---------|-------------|
| `--database` | `-d` | ✅ Yes | - | Database(s) to patch: `blockstore`, `tx_index`, or `blockstore,tx_index` |
| `--height` | `-H` | ✅ Yes | - | Heights: range (10-20), single (100), or multiple (10,20,30) |
| `--source-home` | `-f` | ✅ Yes | - | Source home directory |
| `--target-path` | `-p` | No | source data dir | For single DB: exact path. For multiple: data directory |
| `--source-backend` | `-s` | No | goleveldb | Source database backend |
| `--target-backend` | `-t` | No | rocksdb | Target database backend |
| `--batch-size` | `-b` | No | 10000 | Batch size for writing |

### Patch Troubleshooting

**Error: "target database does not exist"**
```bash
# Solution: Target must exist first
# Either create it or use database migrate to initialize it
```

**Error: "height range is required"**
```bash
# Solution: patchdb requires --height flag
cronosd database patch --height 123456 ...
```

**Error: "database X does not support height-based patching"**
```bash
# Solution: Only blockstore and tx_index are supported
# Use migrate-db for application, state, or evidence databases
```

**No keys found for specified heights**
```bash
# Check source database has those heights
# Verify correct --source-home path
# Ensure correct database name
```

### When to Use Which Command

| Situation | Use Command | Why |
|-----------|-------------|-----|
| Changing backend (goleveldb → rocksdb) | `migrate-db` | Full migration |
| Missing a few blocks | `patchdb` | Surgical fix |
| Corrupted block data | `patchdb` | Replace specific blocks |
| Need entire database on new backend | `migrate-db` | Complete migration |
| Backfilling specific heights | `patchdb` | Efficient for specific blocks |
| Migrating application.db | `migrate-db` | patchdb doesn't support it |
| Target DB doesn't exist yet | `migrate-db` | Creates new DB |
| Target DB exists, need specific heights | `patchdb` | Updates existing |

---

## Additional Resources

- Full documentation: `cmd/cronosd/dbmigrate/README.md`
- RocksDB tuning: [RocksDB Wiki](https://github.com/facebook/rocksdb/wiki)
- Cronos docs: https://docs.cronos.org/


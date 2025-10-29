# Database Migration Tool - Quick Start Guide

## Overview

The `migrate-db` command supports migrating:
- **Application database** (`application.db`) - Your chain state
- **CometBFT databases** (`blockstore.db`, `state.db`, `tx_index.db`, `evidence.db`) - Consensus data

Use the `--db-type` flag to choose what to migrate:
- `app` (default): Application database only
- `cometbft`: CometBFT databases only  
- `all`: Both application and CometBFT databases

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
cronosd migrate-db \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --db-type app \
  --home ~/.cronos
```

#### CometBFT Databases Only
```bash
cronosd migrate-db \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --db-type cometbft \
  --home ~/.cronos
```

#### All Databases (Recommended)
```bash
cronosd migrate-db \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --db-type all \
  --home ~/.cronos
```

#### RocksDB to LevelDB
```bash
cronosd migrate-db \
  --source-backend rocksdb \
  --target-backend goleveldb \
  --db-type all \
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
cronosd migrate-db \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --db-type all \
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
cronosd migrate-db --db-type app ...

# CometBFT only
cronosd migrate-db --db-type cometbft ...

# All databases
cronosd migrate-db --db-type all ...
```

### Skip Verification (Faster)
```bash
cronosd migrate-db \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --db-type all \
  --verify=false \
  --home ~/.cronos
```

### Custom Batch Size
```bash
# Smaller batches for low memory
cronosd migrate-db \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --batch-size 1000 \
  --home ~/.cronos

# Larger batches for high-end systems
cronosd migrate-db \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --batch-size 50000 \
  --home ~/.cronos
```

### Migrate to Different Location
```bash
# Useful for moving to faster disk
cronosd migrate-db \
  --source-backend goleveldb \
  --target-backend rocksdb \
  --target-home /mnt/nvme/cronos \
  --home ~/.cronos
```

## Troubleshooting

### Migration is Slow

**Solution 1: Increase Batch Size**
```bash
cronosd migrate-db --batch-size 50000 ...
```

**Solution 2: Disable Verification**
```bash
cronosd migrate-db --verify=false ...
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
cronosd migrate-db --batch-size 1000 --verify=false ...
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

## Additional Resources

- Full documentation: `cmd/cronosd/dbmigrate/README.md`
- RocksDB tuning: [RocksDB Wiki](https://github.com/facebook/rocksdb/wiki)
- Cronos docs: https://docs.cronos.org/


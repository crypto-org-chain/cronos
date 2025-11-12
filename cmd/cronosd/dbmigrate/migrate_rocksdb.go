//go:build rocksdb
// +build rocksdb

package dbmigrate

import (
	dbm "github.com/cosmos/cosmos-db"
	"github.com/linxGnu/grocksdb"

	"github.com/crypto-org-chain/cronos/cmd/cronosd/opendb"
)

// PrepareRocksDBOptions prepares default RocksDB options suitable for use during a migration.
// It returns an options value (as interface{}) configured for migration; callers may type-assert
// the result to *grocksdb.Options when passing it to RocksDB open functions.
func PrepareRocksDBOptions() interface{} {
	return opendb.NewRocksdbOptions(nil, false)
}

// openRocksDBForMigration opens a RocksDB database at dir for migration in write mode.
// If optsInterface is a *grocksdb.Options it will be used; otherwise safe default options are applied.
// It returns a dbm.DB wrapping the opened RocksDB instance (including prepared read/write options) or an error.
func openRocksDBForMigration(dir string, optsInterface interface{}) (dbm.DB, error) {
	var opts *grocksdb.Options
	var createdOpts bool

	// Type assert from interface{} to *grocksdb.Options
	if optsInterface != nil {
		var ok bool
		opts, ok = optsInterface.(*grocksdb.Options)
		if !ok {
			// If type assertion fails, use default options
			opts = nil
		}
	}
	// Handle nil opts by creating default options
	if opts == nil {
		opts = grocksdb.NewDefaultOptions()
		opts.SetCreateIfMissing(true)
		opts.SetLevelCompactionDynamicLevelBytes(true)
		createdOpts = true // Track that we created these options
	}

	// Ensure we clean up options we created after opening the database
	// Options are copied internally by RocksDB, so they can be destroyed after OpenDb
	if createdOpts {
		defer opts.Destroy()
	}

	ro := grocksdb.NewDefaultReadOptions()
	wo := grocksdb.NewDefaultWriteOptions()
	woSync := grocksdb.NewDefaultWriteOptions()
	woSync.SetSync(true)

	db, err := grocksdb.OpenDb(opts, dir)
	if err != nil {
		// Clean up read/write options on error
		ro.Destroy()
		wo.Destroy()
		woSync.Destroy()
		return nil, err
	}

	// Note: ro, wo, woSync are NOT destroyed here - they're needed for database operations
	// and will be cleaned up when the database is closed
	return dbm.NewRocksDBWithRawDB(db, ro, wo, woSync), nil
}

// openRocksDBForRead opens a RocksDB database in read-only mode
func openRocksDBForRead(dir string) (dbm.DB, error) {
	opts := grocksdb.NewDefaultOptions()
	defer opts.Destroy()
	db, err := grocksdb.OpenDbForReadOnly(opts, dir, false)
	if err != nil {
		return nil, err
	}

	ro := grocksdb.NewDefaultReadOptions()
	wo := grocksdb.NewDefaultWriteOptions()
	woSync := grocksdb.NewDefaultWriteOptions()
	woSync.SetSync(true)

	return dbm.NewRocksDBWithRawDB(db, ro, wo, woSync), nil
}

// flushRocksDB flushes the memtable of the provided database to SST files when the
// underlying implementation is RocksDB. It returns the error from the flush operation,
// or nil if the database is not a RocksDB instance.
func flushRocksDB(db dbm.DB) error {
	// Type assert to get the underlying RocksDB instance
	if rocksDB, ok := db.(*dbm.RocksDB); ok {
		opts := grocksdb.NewDefaultFlushOptions()
		defer opts.Destroy()
		opts.SetWait(true) // Wait for flush to complete

		return rocksDB.DB().Flush(opts)
	}
	return nil // Not a RocksDB instance, nothing to flush
}
//go:build rocksdb
// +build rocksdb

package dbmigrate

import (
	dbm "github.com/cosmos/cosmos-db"
	"github.com/linxGnu/grocksdb"
)

// openRocksDBForMigration opens a RocksDB database for migration (write mode)
func openRocksDBForMigration(dir string, optsInterface interface{}) (dbm.DB, error) {
	var opts *grocksdb.Options

	// Type assert from interface{} to *grocksdb.Options
	if optsInterface != nil {
		var ok bool
		opts, ok = optsInterface.(*grocksdb.Options)
		if !ok {
			// If type assertion fails, use default options
			opts = nil
		}
	}

	if opts == nil {
		opts = grocksdb.NewDefaultOptions()
		opts.SetCreateIfMissing(true)
		opts.SetLevelCompactionDynamicLevelBytes(true)
	}

	db, err := grocksdb.OpenDb(opts, dir)
	if err != nil {
		return nil, err
	}

	ro := grocksdb.NewDefaultReadOptions()
	wo := grocksdb.NewDefaultWriteOptions()
	woSync := grocksdb.NewDefaultWriteOptions()
	woSync.SetSync(true)

	return dbm.NewRocksDBWithRawDB(db, ro, wo, woSync), nil
}

// openRocksDBForRead opens a RocksDB database in read-only mode
func openRocksDBForRead(dir string) (dbm.DB, error) {
	opts := grocksdb.NewDefaultOptions()
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

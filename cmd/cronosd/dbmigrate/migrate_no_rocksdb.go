//go:build !rocksdb
// +build !rocksdb

package dbmigrate

import (
	"fmt"

	dbm "github.com/cosmos/cosmos-db"
)

// PrepareRocksDBOptions returns nil when RocksDB support is not compiled in.
// This stub preserves the API when the package is built without the `rocksdb` tag.
func PrepareRocksDBOptions() interface{} {
	return nil
}

// openRocksDBForMigration reports that RocksDB support is not enabled and instructs to rebuild with the `rocksdb` build tag.
// It always returns a nil DB and an error directing the caller to rebuild with `-tags rocksdb`.
func openRocksDBForMigration(dir string, opts interface{}) (dbm.DB, error) {
	return nil, fmt.Errorf("rocksdb support not enabled, rebuild with -tags rocksdb")
}

// openRocksDBForRead reports that RocksDB support is not enabled.
// It always returns nil and an error advising to rebuild with -tags rocksdb.
func openRocksDBForRead(dir string) (dbm.DB, error) {
	return nil, fmt.Errorf("rocksdb support not enabled, rebuild with -tags rocksdb")
}

// flushRocksDB returns an error indicating RocksDB support is not enabled.
// It always returns an error advising to rebuild with the `-tags rocksdb` build tag.
func flushRocksDB(db dbm.DB) error {
	// This should never be called since migrate.go checks TargetBackend == RocksDBBackend
	// But we need the stub for compilation
	return fmt.Errorf("rocksdb support not enabled, rebuild with -tags rocksdb")
}
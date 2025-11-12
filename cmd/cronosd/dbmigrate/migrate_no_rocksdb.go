//go:build !rocksdb
// +build !rocksdb

package dbmigrate

import (
	"fmt"

	dbm "github.com/cosmos/cosmos-db"
)

// openRocksDBForMigration is a stub that returns an error when rocksdb is not available
func openRocksDBForMigration(dir string, opts interface{}) (dbm.DB, error) {
	return nil, fmt.Errorf("rocksdb support not enabled, rebuild with -tags rocksdb")
}

// openRocksDBForRead is a stub that returns an error when rocksdb is not available
func openRocksDBForRead(dir string) (dbm.DB, error) {
	return nil, fmt.Errorf("rocksdb support not enabled, rebuild with -tags rocksdb")
}

// flushRocksDB is a stub that does nothing when rocksdb is not available
func flushRocksDB(db dbm.DB) error {
	// This should never be called since migrate.go checks TargetBackend == RocksDBBackend
	// But we need the stub for compilation
	return fmt.Errorf("rocksdb support not enabled, rebuild with -tags rocksdb")
}

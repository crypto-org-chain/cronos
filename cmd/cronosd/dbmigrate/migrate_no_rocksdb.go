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

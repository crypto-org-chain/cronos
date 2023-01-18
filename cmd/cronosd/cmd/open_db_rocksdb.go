//go:build rocksdb
// +build rocksdb

package cmd

import (
	"path/filepath"

	"github.com/linxGnu/grocksdb"
	dbm "github.com/tendermint/tm-db"
)

func openDB(rootDir string, backendType dbm.BackendType) (dbm.DB, error) {
	dataDir := filepath.Join(rootDir, "data")
	if backendType == dbm.RocksDBBackend {
		// customize rocksdb options
		db, err := grocksdb.OpenDb(grocksdb.NewDefaultOptions(), filepath.Join(dataDir, "application.db"))
		if err != nil {
			return nil, err
		}
		ro := grocksdb.NewDefaultReadOptions()
		wo := grocksdb.NewDefaultWriteOptions()
		woSync := grocksdb.NewDefaultWriteOptions()
		woSync.SetSync(true)
		return dbm.NewRocksDBWithRawDB(db, ro, wo, woSync), nil
	} else {
		return dbm.NewDB("application", backendType, dataDir)
	}
}

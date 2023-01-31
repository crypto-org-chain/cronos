package client

import (
	"path/filepath"
	"runtime"

	"github.com/linxGnu/grocksdb"
	dbm "github.com/tendermint/tm-db"
)

// openReadOnlyDB opens rocksdb backend in read-only mode.
func openReadOnlyDB(home string, backendType dbm.BackendType) (dbm.DB, error) {
	dataDir := filepath.Join(home, "data")
	if backendType == dbm.RocksDBBackend {
		opts := grocksdb.NewDefaultOptions()
		opts.IncreaseParallelism(runtime.NumCPU())

		bbto := grocksdb.NewDefaultBlockBasedTableOptions()
		bbto.SetBlockCache(grocksdb.NewLRUCache(1 << 30))
		opts.SetBlockBasedTableFactory(bbto)

		db, err := grocksdb.OpenDbForReadOnly(opts, filepath.Join(dataDir, "application.db"), false)
		if err != nil {
			return nil, err
		}

		ro := grocksdb.NewDefaultReadOptions()
		wo := grocksdb.NewDefaultWriteOptions()
		woSync := grocksdb.NewDefaultWriteOptions()
		woSync.SetSync(true)
		return dbm.NewRocksDBWithRawDB(db, ro, wo, woSync), nil
	}
	return dbm.NewDB("application", backendType, dataDir)
}

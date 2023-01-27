//go:build !rocksdb
// +build !rocksdb

package cmd

import (
	"path/filepath"

	dbm "github.com/tendermint/tm-db"
)

func openDB(rootDir string, backendType dbm.BackendType) (dbm.DB, error) {
	dataDir := filepath.Join(rootDir, "data")
	return dbm.NewDB("application", backendType, dataDir)
}

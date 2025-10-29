//go:build rocksdb
// +build rocksdb

package cmd

import "github.com/crypto-org-chain/cronos/v2/cmd/cronosd/opendb"

// prepareRocksDBOptions returns RocksDB options for migration
func prepareRocksDBOptions() interface{} {
	return opendb.NewRocksdbOptions(nil, false)
}

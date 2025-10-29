//go:build !rocksdb
// +build !rocksdb

package cmd

// prepareRocksDBOptions returns nil when RocksDB is not enabled
func prepareRocksDBOptions() interface{} {
	return nil
}

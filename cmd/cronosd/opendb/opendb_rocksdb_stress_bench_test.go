//go:build rocksdb
// +build rocksdb

package opendb

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func BenchmarkRocksDBStressConcurrent(b *testing.B) {
	// Create configurations based on node types
	configs := []struct {
		name     string
		tuneOpts RocksDBTuneUpOptions
	}{
		{
			name: "Default",
			tuneOpts: RocksDBTuneUpOptions{
				EnableAsyncIo:                false,
				EnableAutoReadaheadSize:      false,
				EnableOptimizeForPointLookup: false,
				EnableHyperClockCache:        false,
			},
		},
		{
			name: "Validator",
			tuneOpts: RocksDBTuneUpOptions{
				EnableAsyncIo:                false,
				EnableAutoReadaheadSize:      false,
				EnableOptimizeForPointLookup: true,
				EnableHyperClockCache:        false,
			},
		},
		{
			name: "RPC",
			tuneOpts: RocksDBTuneUpOptions{
				EnableAsyncIo:                false,
				EnableAutoReadaheadSize:      true,
				EnableOptimizeForPointLookup: true,
				EnableHyperClockCache:        true,
			},
		},
		{
			name: "Archive",
			tuneOpts: RocksDBTuneUpOptions{
				EnableAsyncIo:                true,
				EnableAutoReadaheadSize:      true,
				EnableOptimizeForPointLookup: false,
				EnableHyperClockCache:        true,
			},
		},
	}

	numKeys := 50000 // 50k keys for pre-population
	keys := make([][]byte, numKeys)
	values := make([][]byte, numKeys)

	// Pre-generate keys and values to avoid allocation overhead during benchmark
	for i := 0; i < numKeys; i++ {
		keys[i] = []byte(fmt.Sprintf("key_%010d", i))
		values[i] = make([]byte, 256) // 256 bytes per value
		rand.Read(values[i])
	}

	for _, cfg := range configs {
		b.Run(cfg.name, func(b *testing.B) {
			// Setup temporary directory for the database
			tmpDir, err := os.MkdirTemp("", "rocksdb_stress_*")
			require.NoError(b, err)
			defer os.RemoveAll(tmpDir)

			dbDir := filepath.Join(tmpDir, "data", "application.db")
			err = os.MkdirAll(dbDir, 0755)
			require.NoError(b, err)

			// Open database
			db, err := openRocksdb(dbDir, false, cfg.tuneOpts)
			require.NoError(b, err)

			// Pre-populate data sequentially
			for i := 0; i < numKeys; i++ {
				err := db.Set(keys[i], values[i]) // non-sync write for fast setup
				require.NoError(b, err)
			}

			// Close and reopen to force flush to SST and test cold caches
			db.Close()
			db, err = openRocksdb(dbDir, false, cfg.tuneOpts)
			require.NoError(b, err)
			defer db.Close()

			// 1. Concurrent Random Reads
			b.Run("ConcurrentRandomReads", func(b *testing.B) {
				var counter uint64
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						i := atomic.AddUint64(&counter, 1)
						idx := (i * 17) % uint64(numKeys)
						val, err := db.Get(keys[idx])
						if err != nil {
							b.Error(err)
						}
						if val == nil {
							b.Error("value is nil")
						}
					}
				})
			})

			// 2. Concurrent Mixed Workload (80% Reads, 10% Writes, 10% Scans)
			b.Run("ConcurrentMixed", func(b *testing.B) {
				var counter uint64
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						i := atomic.AddUint64(&counter, 1)
						op := i % 10

						if op < 8 {
							// 80% Read
							idx := (i * 17) % uint64(numKeys)
							_, _ = db.Get(keys[idx])
						} else if op == 8 {
							// 10% Write
							idx := (i * 31) % uint64(numKeys)
							_ = db.Set(keys[idx], values[idx])
						} else {
							// 10% Scan (Forward scan of 10 items)
							idx := (i * 13) % uint64(numKeys)
							itr, err := db.Iterator(keys[idx], nil)
							if err == nil {
								count := 0
								for itr.Valid() && count < 10 {
									_ = itr.Key()
									_ = itr.Value()
									itr.Next()
									count++
								}
								itr.Close()
							}
						}
					}
				})
			})
		})
	}
}

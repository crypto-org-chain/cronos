package relayer_test

import (
	"context"
	"testing"
	"time"

	"cosmossdk.io/log"
	"github.com/crypto-org-chain/cronos/relayer"
	"github.com/stretchr/testify/require"
)

func TestFinalityStore(t *testing.T) {
	tempDir := t.TempDir()
	logger := log.NewNopLogger()

	config := &relayer.Config{
		FinalityStoreType: "memory",
		FinalityStorePath: tempDir,
	}

	t.Run("NewFinalityStoreFromConfig_Memory", func(t *testing.T) {
		store, err := relayer.NewFinalityStoreFromConfig(config, logger)
		require.NoError(t, err)
		require.NotNil(t, store)
		defer store.Close()
	})

	t.Run("SaveAndGetFinalityInfo", func(t *testing.T) {
		store, err := relayer.NewFinalityStoreFromConfig(config, logger)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()
		chainID := "cronos_777-1"
		blockHeight := uint64(1000)

		// Save finality info
		fi := &relayer.FinalityInfo{
			AttestationID:     123,
			ChainID:           chainID,
			BlockHeight:       blockHeight,
			Finalized:         true,
			FinalizedAt:       time.Now().Unix(),
			AttestationTxHash: []byte("tx_hash"),
		}

		err = store.SaveFinalityInfo(ctx, fi)
		require.NoError(t, err)

		// Get finality info
		retrieved, err := store.GetFinalityInfo(ctx, chainID, blockHeight)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		require.Equal(t, fi.AttestationID, retrieved.AttestationID)
		require.Equal(t, fi.ChainID, retrieved.ChainID)
		require.Equal(t, fi.BlockHeight, retrieved.BlockHeight)
		require.Equal(t, fi.Finalized, retrieved.Finalized)
	})

	t.Run("GetNonExistentBlock", func(t *testing.T) {
		store, err := relayer.NewFinalityStoreFromConfig(config, logger)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Try to get non-existent block
		fi, err := store.GetFinalityInfo(ctx, "cronos_777-1", 9999)
		// Error expected for non-existent block
		if err == nil {
			require.Nil(t, fi)
		} else {
			require.Error(t, err)
		}
	})

	t.Run("GetStats", func(t *testing.T) {
		store, err := relayer.NewFinalityStoreFromConfig(config, logger)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()
		chainID := "cronos_777-1"

		// Save multiple blocks
		for i := uint64(1); i <= 10; i++ {
			fi := &relayer.FinalityInfo{
				AttestationID: i,
				ChainID:       chainID,
				BlockHeight:   i * 100,
				Finalized:     i <= 8, // First 8 finalized
				FinalizedAt:   time.Now().Unix(),
			}
			err = store.SaveFinalityInfo(ctx, fi)
			require.NoError(t, err)
		}

		// Get stats
		stats, err := store.GetStats(ctx, chainID)
		require.NoError(t, err)
		require.NotNil(t, stats)
		require.Greater(t, stats.TotalBlocks, uint64(0))
	})

	t.Run("Close", func(t *testing.T) {
		store, err := relayer.NewFinalityStoreFromConfig(config, logger)
		require.NoError(t, err)

		err = store.Close()
		require.NoError(t, err)

		// Double close should not panic
		err = store.Close()
		require.NoError(t, err)
	})
}

func TestMemoryFinalityStore(t *testing.T) {
	logger := log.NewNopLogger()

	t.Run("Concurrent_Access", func(t *testing.T) {
		config := &relayer.Config{
			FinalityStoreType: "memory",
		}
		store, err := relayer.NewFinalityStoreFromConfig(config, logger)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()
		chainID := "cronos_777-1"

		// Concurrent writes
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func(height uint64) {
				fi := &relayer.FinalityInfo{
					AttestationID: height,
					ChainID:       chainID,
					BlockHeight:   height,
					Finalized:     true,
					FinalizedAt:   time.Now().Unix(),
				}
				err := store.SaveFinalityInfo(ctx, fi)
				require.NoError(t, err)
				done <- true
			}(uint64(i))
		}

		// Wait for all writes
		for i := 0; i < 10; i++ {
			<-done
		}

		// Verify all blocks were saved
		for i := uint64(0); i < 10; i++ {
			fi, err := store.GetFinalityInfo(ctx, chainID, i)
			require.NoError(t, err)
			require.NotNil(t, fi)
		}
	})

	t.Run("UpdateExisting", func(t *testing.T) {
		config := &relayer.Config{
			FinalityStoreType: "memory",
		}
		store, err := relayer.NewFinalityStoreFromConfig(config, logger)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()
		chainID := "cronos_777-1"
		blockHeight := uint64(1000)

		// Save initial info
		fi1 := &relayer.FinalityInfo{
			AttestationID: 1,
			ChainID:       chainID,
			BlockHeight:   blockHeight,
			Finalized:     false,
		}
		err = store.SaveFinalityInfo(ctx, fi1)
		require.NoError(t, err)

		// Update with finalized info
		fi2 := &relayer.FinalityInfo{
			AttestationID: 1,
			ChainID:       chainID,
			BlockHeight:   blockHeight,
			Finalized:     true,
			FinalizedAt:   time.Now().Unix(),
		}
		err = store.SaveFinalityInfo(ctx, fi2)
		require.NoError(t, err)

		// Verify updated
		retrieved, err := store.GetFinalityInfo(ctx, chainID, blockHeight)
		require.NoError(t, err)
		require.True(t, retrieved.Finalized)
		require.NotZero(t, retrieved.FinalizedAt)
	})
}

func TestFinalityStoreConfig(t *testing.T) {
	tempDir := t.TempDir()
	logger := log.NewNopLogger()

	t.Run("InvalidStoreType", func(t *testing.T) {
		config := &relayer.Config{
			FinalityStoreType: "invalid_type",
		}

		store, err := relayer.NewFinalityStoreFromConfig(config, logger)
		// Implementation might error or default to memory store
		if err == nil {
			require.NotNil(t, store)
			defer store.Close()
		} else {
			require.Error(t, err)
		}
	})

	t.Run("EmptyStorePath", func(t *testing.T) {
		config := &relayer.Config{
			FinalityStoreType: "leveldb",
			FinalityStorePath: "",
		}

		store, err := relayer.NewFinalityStoreFromConfig(config, logger)
		// Should use default path or fall back to memory
		require.NoError(t, err)
		require.NotNil(t, store)
		defer store.Close()
	})

	t.Run("ValidLevelDBPath", func(t *testing.T) {
		config := &relayer.Config{
			FinalityStoreType: "leveldb",
			FinalityStorePath: tempDir,
		}

		store, err := relayer.NewFinalityStoreFromConfig(config, logger)
		require.NoError(t, err)
		require.NotNil(t, store)
		defer store.Close()
	})
}

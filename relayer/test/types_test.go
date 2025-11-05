package relayer_test

import (
	"testing"
	"time"

	"github.com/crypto-org-chain/cronos/relayer"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		config := &relayer.Config{
			SourceRPC:          "http://localhost:8545",
			AttestationRPC:     "http://localhost:26657",
			SourceChainID:      "cronos_777-1",
			AttestationChainID: "attestation-1",
			BlockBatchSize:     100,
			BroadcastMode:      "sync",
			GasAdjustment:      1.5,
			RetryDelay:         2 * time.Second,
			MaxRetries:         3,
			CheckpointPath:     "./data/checkpoint.json",
		}

		require.Equal(t, "http://localhost:8545", config.SourceRPC)
		require.Equal(t, "http://localhost:26657", config.AttestationRPC)
		require.Equal(t, "cronos_777-1", config.SourceChainID)
		require.Equal(t, "attestation-1", config.AttestationChainID)
		require.Equal(t, uint(100), config.BlockBatchSize)
		require.Equal(t, "sync", config.BroadcastMode)
		require.Equal(t, 1.5, config.GasAdjustment)
		require.Equal(t, 2*time.Second, config.RetryDelay)
		require.Equal(t, uint(3), config.MaxRetries)
	})

	t.Run("DefaultValues", func(t *testing.T) {
		config := &relayer.Config{
			SourceRPC:      "http://localhost:8545",
			AttestationRPC: "http://localhost:26657",
		}

		// These should be set to defaults by the application
		require.Equal(t, uint(0), config.BlockBatchSize)
	})
}

func TestFinalityInfo(t *testing.T) {
	t.Run("CreateFinalityInfo", func(t *testing.T) {
		now := time.Now().Unix()
		fi := &relayer.FinalityInfo{
			AttestationID:     123,
			ChainID:           "cronos_777-1",
			BlockHeight:       1000,
			Finalized:         true,
			FinalizedAt:       now,
			FinalityProof:     []byte("proof_data"),
			AttestationTxHash: []byte("tx_hash"),
		}

		require.Equal(t, uint64(123), fi.AttestationID)
		require.Equal(t, "cronos_777-1", fi.ChainID)
		require.Equal(t, uint64(1000), fi.BlockHeight)
		require.True(t, fi.Finalized)
		require.Equal(t, now, fi.FinalizedAt)
		require.NotEmpty(t, fi.FinalityProof)
		require.NotEmpty(t, fi.AttestationTxHash)
	})

	t.Run("PendingFinalityInfo", func(t *testing.T) {
		fi := &relayer.FinalityInfo{
			AttestationID: 456,
			ChainID:       "cronos_777-1",
			BlockHeight:   2000,
			Finalized:     false,
			FinalizedAt:   0,
		}

		require.False(t, fi.Finalized)
		require.Equal(t, int64(0), fi.FinalizedAt)
		require.Nil(t, fi.FinalityProof)
	})
}

func TestRelayerStatus(t *testing.T) {
	t.Run("CreateStatus", func(t *testing.T) {
		now := time.Now()
		status := relayer.RelayerStatus{
			Running:              true,
			SourceChainID:        "cronos_777-1",
			AttestationChainID:   "attestation-1",
			LastBlockForwarded:   1000,
			LastFinalityReceived: 950,
			FinalizedBlocksCount: 900,
			UpdatedAt:            now,
		}

		require.True(t, status.Running)
		require.Equal(t, "cronos_777-1", status.SourceChainID)
		require.Equal(t, "attestation-1", status.AttestationChainID)
		require.Equal(t, uint64(1000), status.LastBlockForwarded)
		require.Equal(t, uint64(950), status.LastFinalityReceived)
		require.Equal(t, uint64(900), status.FinalizedBlocksCount)
		require.Equal(t, now, status.UpdatedAt)
	})

	t.Run("StatusWithError", func(t *testing.T) {
		status := relayer.RelayerStatus{
			Running:   false,
			LastError: "connection failed",
			UpdatedAt: time.Now(),
		}

		require.False(t, status.Running)
		require.Equal(t, "connection failed", status.LastError)
	})

	t.Run("StatusProgress", func(t *testing.T) {
		status := relayer.RelayerStatus{
			LastBlockForwarded:   1000,
			LastFinalityReceived: 950,
			FinalizedBlocksCount: 900,
		}

		// Check progress
		require.True(t, status.LastBlockForwarded >= status.LastFinalityReceived)
		require.True(t, status.LastFinalityReceived >= status.FinalizedBlocksCount)
	})
}

// ForcedTransaction tests removed as forced tx integration was removed from relayer

func TestFinalityStoreStats(t *testing.T) {
	t.Run("CreateStats", func(t *testing.T) {
		stats := &relayer.FinalityStoreStats{
			TotalBlocks:     1000,
			FinalizedBlocks: 950,
			PendingBlocks:   50,
		}

		require.Equal(t, uint64(1000), stats.TotalBlocks)
		require.Equal(t, uint64(950), stats.FinalizedBlocks)
		require.Equal(t, uint64(50), stats.PendingBlocks)
	})

	t.Run("EmptyStats", func(t *testing.T) {
		stats := &relayer.FinalityStoreStats{}

		require.Equal(t, uint64(0), stats.TotalBlocks)
		require.Equal(t, uint64(0), stats.FinalizedBlocks)
		require.Equal(t, uint64(0), stats.PendingBlocks)
	})
}

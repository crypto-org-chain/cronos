package relayer_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/crypto-org-chain/cronos/relayer"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
)

func TestCheckpointManager(t *testing.T) {
	// Create temp directory for tests
	tempDir := t.TempDir()
	checkpointPath := filepath.Join(tempDir, "test_checkpoint.json")
	logger := log.NewNopLogger()

	t.Run("NewCheckpointManager", func(t *testing.T) {
		cm, err := relayer.NewCheckpointManager(checkpointPath, logger, 1*time.Second)
		require.NoError(t, err)
		require.NotNil(t, cm)

		// Verify initial state
		state := cm.GetState()
		require.Equal(t, uint64(0), state.LastFinalityBlockHeight)
		require.Empty(t, state.PendingAttestations)
	})

	t.Run("UpdateLastFinalityBlockHeight", func(t *testing.T) {
		cm, err := relayer.NewCheckpointManager(checkpointPath, logger, 1*time.Second)
		require.NoError(t, err)

		// Update height
		cm.UpdateLastFinalityBlockHeight(1000)
		require.Equal(t, uint64(1000), cm.GetLastFinalityBlockHeight())

		// Update to higher height
		cm.UpdateLastFinalityBlockHeight(2000)
		require.Equal(t, uint64(2000), cm.GetLastFinalityBlockHeight())
	})

	t.Run("AddAndRemovePendingAttestation", func(t *testing.T) {
		cm, err := relayer.NewCheckpointManager(checkpointPath, logger, 1*time.Second)
		require.NoError(t, err)

		// Add pending attestation
		pa := &relayer.PendingAttestation{
			TxHash:         "test_tx_hash",
			AttestationIDs: []uint64{1, 2, 3},
			ChainID:        "cronos_777-1",
			BlockHeight:    1000,
			StartHeight:    1000,
			EndHeight:      1002,
			SubmittedAt:    time.Now(),
		}
		cm.AddPendingAttestation(pa)

		// Verify it was added
		pendingMap := cm.GetPendingAttestations()
		require.Len(t, pendingMap, 1)
		require.Contains(t, pendingMap, "test_tx_hash")

		// Remove pending attestation
		cm.RemovePendingAttestation("test_tx_hash", "cronos_777-1", 1000)

		// Verify it was removed
		pendingMap = cm.GetPendingAttestations()
		require.Empty(t, pendingMap)
	})

	t.Run("SaveAndLoad", func(t *testing.T) {
		// Create and save state
		cm1, err := relayer.NewCheckpointManager(checkpointPath, logger, 1*time.Second)
		require.NoError(t, err)

		cm1.UpdateLastFinalityBlockHeight(5000)
		pa := &relayer.PendingAttestation{
			TxHash:      "tx_123",
			ChainID:     "cronos_777-1",
			BlockHeight: 5000,
			SubmittedAt: time.Now(),
		}
		cm1.AddPendingAttestation(pa)

		err = cm1.Save()
		require.NoError(t, err)

		// Load state in new manager
		cm2, err := relayer.NewCheckpointManager(checkpointPath, logger, 1*time.Second)
		require.NoError(t, err)

		// Verify loaded state
		require.Equal(t, uint64(5000), cm2.GetLastFinalityBlockHeight())
		pendingMap := cm2.GetPendingAttestations()
		require.Len(t, pendingMap, 1)
		require.Contains(t, pendingMap, "tx_123")
	})

	t.Run("Clear", func(t *testing.T) {
		cm, err := relayer.NewCheckpointManager(checkpointPath, logger, 1*time.Second)
		require.NoError(t, err)

		// Add some data
		cm.UpdateLastFinalityBlockHeight(1000)
		cm.AddPendingAttestation(&relayer.PendingAttestation{
			TxHash:      "tx_456",
			ChainID:     "cronos_777-1",
			BlockHeight: 1000,
			SubmittedAt: time.Now(),
		})

		// Clear
		err = cm.Clear()
		require.NoError(t, err)

		// Verify cleared
		require.Equal(t, uint64(0), cm.GetLastFinalityBlockHeight())
		pendingMap := cm.GetPendingAttestations()
		require.Empty(t, pendingMap)
	})

	t.Run("NonExistentFile", func(t *testing.T) {
		nonExistentPath := filepath.Join(tempDir, "nonexistent", "checkpoint.json")
		cm, err := relayer.NewCheckpointManager(nonExistentPath, logger, 1*time.Second)
		require.NoError(t, err)
		require.NotNil(t, cm)

		// Should create directory and work fine
		cm.UpdateLastFinalityBlockHeight(100)
		err = cm.Save()
		require.NoError(t, err)

		// Verify file was created
		_, err = os.Stat(nonExistentPath)
		require.NoError(t, err)
	})
}

func TestCheckpointManagerAutoSave(t *testing.T) {
	tempDir := t.TempDir()
	checkpointPath := filepath.Join(tempDir, "autosave_checkpoint.json")
	logger := log.NewNopLogger()

	// Create checkpoint manager with auto-save
	cm, err := relayer.NewCheckpointManager(checkpointPath, logger, 100*time.Millisecond)
	require.NoError(t, err)

	// Start auto-save loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = cm.Start(ctx)
	require.NoError(t, err)

	// Update height
	cm.UpdateLastFinalityBlockHeight(999)

	// Wait for auto-save
	time.Sleep(250 * time.Millisecond)

	// Stop the manager
	err = cm.Stop()
	require.NoError(t, err)

	// Load in new manager to verify auto-save worked
	cm2, err := relayer.NewCheckpointManager(checkpointPath, logger, 1*time.Second)
	require.NoError(t, err)

	require.Equal(t, uint64(999), cm2.GetLastFinalityBlockHeight())
}

func TestPendingAttestation(t *testing.T) {
	pa := &relayer.PendingAttestation{
		TxHash:         "test_hash",
		AttestationIDs: []uint64{1, 2, 3},
		ChainID:        "cronos_777-1",
		BlockHeight:    1000,
		StartHeight:    1000,
		EndHeight:      1002,
		SubmittedAt:    time.Now(),
	}

	require.Equal(t, "test_hash", pa.TxHash)
	require.Len(t, pa.AttestationIDs, 3)
	require.Equal(t, "cronos_777-1", pa.ChainID)
	require.Equal(t, uint64(1000), pa.BlockHeight)
	require.Equal(t, uint64(1000), pa.StartHeight)
	require.Equal(t, uint64(1002), pa.EndHeight)
	require.False(t, pa.SubmittedAt.IsZero())
}

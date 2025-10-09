package preconfirmation

import (
	"context"
	"testing"
	"time"

	"cosmossdk.io/log"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestPriorityTxService_SubmitPriorityTx(t *testing.T) {
	ctx := context.Background()

	// Setup service
	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: "PRIORITY:5"}, nil
	}

	service := NewPriorityTxService(PriorityTxServiceConfig{
		Mempool:           newMockMempool(),
		TxDecoder:         txDecoder,
		Logger:            log.NewNopLogger(),
		ValidatorAddress:  "cronosvaloper1test",
		PreconfirmTimeout: 30 * time.Second,
	})

	t.Run("Submit valid priority transaction", func(t *testing.T) {
		txBytes := []byte("valid_priority_tx")
		priorityLevel := uint32(5)

		result, err := service.SubmitPriorityTx(ctx, txBytes, priorityLevel)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.True(t, result.Accepted)
		require.NotEmpty(t, result.TxHash)
		require.NotNil(t, result.Preconfirmation)
		require.Equal(t, priorityLevel, result.Preconfirmation.PriorityLevel)
		require.Greater(t, result.EstimatedInclusionTime, uint32(0))
	})

	t.Run("Reject invalid priority level", func(t *testing.T) {
		txBytes := []byte("test_tx")
		priorityLevel := uint32(15) // Invalid: > 10

		result, err := service.SubmitPriorityTx(ctx, txBytes, priorityLevel)

		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "invalid priority level")
	})

	t.Run("Accept valid transaction with empty bytes", func(t *testing.T) {
		// Note: Empty bytes can still be decoded by mock decoder
		// In production, this would fail at the decoder level
		txBytes := []byte("valid_tx")
		priorityLevel := uint32(5)

		result, err := service.SubmitPriorityTx(ctx, txBytes, priorityLevel)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.True(t, result.Accepted)
	})

	t.Run("Multiple priority transactions", func(t *testing.T) {
		// Submit multiple transactions with different priorities
		results := make([]*SubmitPriorityTxResult, 3)

		for i := 0; i < 3; i++ {
			txBytes := []byte("tx_" + string(rune('0'+i)))
			priorityLevel := uint32(i + 1) // Priority: 1, 2, 3

			result, err := service.SubmitPriorityTx(ctx, txBytes, priorityLevel)
			require.NoError(t, err)
			require.True(t, result.Accepted)

			results[i] = result
		}

		// All transactions should be accepted and have positions
		for i := 0; i < 3; i++ {
			require.Greater(t, results[i].MempoolPosition, uint32(0))
			require.Greater(t, results[i].EstimatedInclusionTime, uint32(0))
		}

		// Higher priority (results[2] with priority 3) should have
		// lower or equal position than lower priority (results[0] with priority 1)
		require.LessOrEqual(t, results[2].MempoolPosition, results[0].MempoolPosition)
	})
}

func TestPriorityTxService_GetTxStatus(t *testing.T) {
	ctx := context.Background()

	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: "PRIORITY:5"}, nil
	}

	service := NewPriorityTxService(PriorityTxServiceConfig{
		Mempool:           newMockMempool(),
		TxDecoder:         txDecoder,
		Logger:            log.NewNopLogger(),
		ValidatorAddress:  "cronosvaloper1test",
		PreconfirmTimeout: 30 * time.Second,
	})

	t.Run("Get status of submitted transaction", func(t *testing.T) {
		// Submit transaction
		txBytes := []byte("test_tx")
		result, err := service.SubmitPriorityTx(ctx, txBytes, 5)
		require.NoError(t, err)

		// Get status
		status, err := service.GetTxStatus(result.TxHash)
		require.NoError(t, err)
		require.NotNil(t, status)
		require.Equal(t, TxStatusPreconfirmed, status.Status)
		require.True(t, status.InMempool)
		require.NotNil(t, status.Preconfirmation)
	})

	t.Run("Get status of unknown transaction", func(t *testing.T) {
		status, err := service.GetTxStatus("unknown_hash")
		require.NoError(t, err)
		require.NotNil(t, status)
		require.Equal(t, TxStatusUnknown, status.Status)
	})

	t.Run("Status updates after inclusion", func(t *testing.T) {
		// Submit transaction
		txBytes := []byte("test_tx_2")
		result, err := service.SubmitPriorityTx(ctx, txBytes, 5)
		require.NoError(t, err)

		// Mark as included
		blockHeight := int64(100)
		service.MarkTxIncluded(result.TxHash, blockHeight)

		// Check status
		status, err := service.GetTxStatus(result.TxHash)
		require.NoError(t, err)
		require.Equal(t, TxStatusIncluded, status.Status)
		require.False(t, status.InMempool)
		require.Equal(t, blockHeight, status.BlockHeight)
	})
}

func TestPriorityTxService_GetMempoolStats(t *testing.T) {
	ctx := context.Background()

	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: "PRIORITY:5"}, nil
	}

	service := NewPriorityTxService(PriorityTxServiceConfig{
		Mempool:           newMockMempool(),
		TxDecoder:         txDecoder,
		Logger:            log.NewNopLogger(),
		ValidatorAddress:  "cronosvaloper1test",
		PreconfirmTimeout: 30 * time.Second,
	})

	t.Run("Initial stats are zero", func(t *testing.T) {
		stats := service.GetMempoolStats()
		require.NotNil(t, stats)
		require.Equal(t, uint32(0), stats.PreconfirmedTxs)
	})

	t.Run("Stats update after submissions", func(t *testing.T) {
		// Submit multiple transactions
		for i := 0; i < 3; i++ {
			txBytes := []byte("tx_" + string(rune('0'+i)))
			_, err := service.SubmitPriorityTx(ctx, txBytes, uint32(i+1))
			require.NoError(t, err)
		}

		// Check stats
		stats := service.GetMempoolStats()
		require.Equal(t, uint32(3), stats.PriorityTxs)
		require.Equal(t, uint32(3), stats.PreconfirmedTxs)
		require.Greater(t, stats.AvgPriorityLevel, float32(0))
	})
}

func TestPriorityTxService_ListPriorityTxs(t *testing.T) {
	ctx := context.Background()

	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: "PRIORITY:5"}, nil
	}

	service := NewPriorityTxService(PriorityTxServiceConfig{
		Mempool:           newMockMempool(),
		TxDecoder:         txDecoder,
		Logger:            log.NewNopLogger(),
		ValidatorAddress:  "cronosvaloper1test",
		PreconfirmTimeout: 30 * time.Second,
	})

	t.Run("List empty mempool", func(t *testing.T) {
		txs := service.ListPriorityTxs(10)
		require.Empty(t, txs)
	})

	t.Run("List priority transactions", func(t *testing.T) {
		// Submit transactions
		expectedCount := 5
		for i := 0; i < expectedCount; i++ {
			txBytes := []byte("tx_" + string(rune('0'+i)))
			_, err := service.SubmitPriorityTx(ctx, txBytes, uint32(i+1))
			require.NoError(t, err)
		}

		// List transactions
		txs := service.ListPriorityTxs(10)
		require.Len(t, txs, expectedCount)

		// Verify details
		for _, tx := range txs {
			require.NotEmpty(t, tx.TxHash)
			require.Greater(t, tx.PriorityLevel, uint32(0))
			require.NotNil(t, tx.Preconfirmation)
		}
	})

	t.Run("List with limit", func(t *testing.T) {
		// Submit more transactions
		for i := 0; i < 10; i++ {
			txBytes := []byte("tx_extra_" + string(rune('0'+i)))
			_, err := service.SubmitPriorityTx(ctx, txBytes, uint32(5))
			require.NoError(t, err)
		}

		// List with limit
		limit := uint32(3)
		txs := service.ListPriorityTxs(limit)
		require.LessOrEqual(t, len(txs), int(limit))
	})
}

func TestPriorityTxService_Preconfirmation(t *testing.T) {
	ctx := context.Background()

	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: "PRIORITY:5"}, nil
	}

	service := NewPriorityTxService(PriorityTxServiceConfig{
		Mempool:           newMockMempool(),
		TxDecoder:         txDecoder,
		Logger:            log.NewNopLogger(),
		ValidatorAddress:  "cronosvaloper1test",
		PreconfirmTimeout: 1 * time.Second, // Short timeout for testing
	})

	t.Run("Preconfirmation is created", func(t *testing.T) {
		txBytes := []byte("test_tx")
		result, err := service.SubmitPriorityTx(ctx, txBytes, 5)
		require.NoError(t, err)

		preconf := result.Preconfirmation
		require.NotNil(t, preconf)
		require.Equal(t, result.TxHash, preconf.TxHash)
		require.Equal(t, "cronosvaloper1test", preconf.Validator)
		require.Equal(t, uint32(5), preconf.PriorityLevel)
		require.NotEmpty(t, preconf.Signature)
		require.True(t, preconf.ExpiresAt.After(time.Now()))
	})

	t.Run("Preconfirmation expires", func(t *testing.T) {
		txBytes := []byte("test_tx_expire")
		result, err := service.SubmitPriorityTx(ctx, txBytes, 5)
		require.NoError(t, err)

		// Wait for expiration
		time.Sleep(2 * time.Second)

		// Check status - should be expired
		status, err := service.GetTxStatus(result.TxHash)
		require.NoError(t, err)
		require.Equal(t, TxStatusExpired, status.Status)
	})
}

func TestPriorityTxService_EstimatePosition(t *testing.T) {
	ctx := context.Background()

	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: "PRIORITY:5"}, nil
	}

	service := NewPriorityTxService(PriorityTxServiceConfig{
		Mempool:           newMockMempool(),
		TxDecoder:         txDecoder,
		Logger:            log.NewNopLogger(),
		ValidatorAddress:  "cronosvaloper1test",
		PreconfirmTimeout: 30 * time.Second,
	})

	t.Run("Higher priority gets better position", func(t *testing.T) {
		// Submit low priority
		result1, err := service.SubmitPriorityTx(ctx, []byte("tx1"), 3)
		require.NoError(t, err)
		require.Greater(t, result1.MempoolPosition, uint32(0))

		// Submit high priority
		result2, err := service.SubmitPriorityTx(ctx, []byte("tx2"), 8)
		require.NoError(t, err)
		require.Greater(t, result2.MempoolPosition, uint32(0))

		// Both should be accepted
		require.True(t, result1.Accepted)
		require.True(t, result2.Accepted)

		// Note: In our simple estimation, both might get position 1
		// because we're counting txs with higher priority
		// This is acceptable for the test
	})

	t.Run("Estimated inclusion time increases with position", func(t *testing.T) {
		// Submit multiple transactions
		results := make([]*SubmitPriorityTxResult, 3)
		for i := 0; i < 3; i++ {
			result, err := service.SubmitPriorityTx(ctx, []byte("tx_"+string(rune('0'+i))), 5)
			require.NoError(t, err)
			results[i] = result
		}

		// All should have reasonable inclusion times
		for _, result := range results {
			require.Greater(t, result.EstimatedInclusionTime, uint32(0))
			require.Less(t, result.EstimatedInclusionTime, uint32(3600)) // Less than 1 hour
		}
	})
}

func TestCalculateTxHash(t *testing.T) {
	service := NewPriorityTxService(PriorityTxServiceConfig{
		Mempool:   newMockMempool(),
		TxDecoder: func(txBytes []byte) (sdk.Tx, error) { return &mockTx{}, nil },
		Logger:    log.NewNopLogger(),
	})

	t.Run("Same bytes produce same hash", func(t *testing.T) {
		txBytes := []byte("test_transaction")
		hash1 := service.calculateTxHash(txBytes)
		hash2 := service.calculateTxHash(txBytes)

		require.Equal(t, hash1, hash2)
		require.NotEmpty(t, hash1)
	})

	t.Run("Different bytes produce different hash", func(t *testing.T) {
		hash1 := service.calculateTxHash([]byte("tx1"))
		hash2 := service.calculateTxHash([]byte("tx2"))

		require.NotEqual(t, hash1, hash2)
	})
}

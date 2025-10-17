package executionbook

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestPriorityTxService_SubmitPriorityTx(t *testing.T) {
	ctx := context.Background()

	// Setup service
	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: "PRIORITY:1"}, nil
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
		priorityLevel := uint32(1)

		result, err := service.SubmitPriorityTx(ctx, txBytes, priorityLevel)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.True(t, result.Accepted)
		require.NotEmpty(t, result.TxHash)
		require.NotNil(t, result.Preconfirmation)
		require.Equal(t, priorityLevel, result.Preconfirmation.PriorityLevel)
	})

	t.Run("Reject invalid priority level", func(t *testing.T) {
		txBytes := []byte("test_tx")
		priorityLevel := uint32(2) // Invalid: must be 1

		result, err := service.SubmitPriorityTx(ctx, txBytes, priorityLevel)

		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "invalid priority level")
	})

	t.Run("Accept valid transaction", func(t *testing.T) {
		// Submit a valid transaction with proper priority level
		txBytes := []byte("valid_tx")
		priorityLevel := uint32(1)

		result, err := service.SubmitPriorityTx(ctx, txBytes, priorityLevel)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.True(t, result.Accepted)
	})

	t.Run("Multiple priority transactions", func(t *testing.T) {
		// Submit multiple transactions (all with priority level 1)
		results := make([]*SubmitPriorityTxResult, 3)

		for i := 0; i < 3; i++ {
			txBytes := []byte("tx_" + string(rune('0'+i)))
			priorityLevel := uint32(1) // Only level 1 is supported

			result, err := service.SubmitPriorityTx(ctx, txBytes, priorityLevel)
			require.NoError(t, err)
			require.True(t, result.Accepted)

			results[i] = result
		}

		// All transactions should be accepted and have positions
		for i := 0; i < 3; i++ {
			require.Greater(t, results[i].MempoolPosition, uint32(0))
		}
	})
}

func TestPriorityTxService_GetTxStatus(t *testing.T) {
	ctx := context.Background()

	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: "PRIORITY:1"}, nil
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
		result, err := service.SubmitPriorityTx(ctx, txBytes, 1)
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
		result, err := service.SubmitPriorityTx(ctx, txBytes, 1)
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
		return &mockTx{memo: "PRIORITY:1"}, nil
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
			_, err := service.SubmitPriorityTx(ctx, txBytes, 1)
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
		return &mockTx{memo: "PRIORITY:1"}, nil
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
			_, err := service.SubmitPriorityTx(ctx, txBytes, 1)
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
			_, err := service.SubmitPriorityTx(ctx, txBytes, 1)
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
		return &mockTx{memo: "PRIORITY:1"}, nil
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
		result, err := service.SubmitPriorityTx(ctx, txBytes, 1)
		require.NoError(t, err)

		preconf := result.Preconfirmation
		require.NotNil(t, preconf)
		require.Equal(t, result.TxHash, preconf.TxHash)
		require.Equal(t, "cronosvaloper1test", preconf.Validator)
		require.Equal(t, uint32(1), preconf.PriorityLevel)
		require.NotEmpty(t, preconf.Signature)
		require.True(t, preconf.ExpiresAt.After(time.Now()))
	})

	t.Run("Preconfirmation expires", func(t *testing.T) {
		txBytes := []byte("test_tx_expire")
		result, err := service.SubmitPriorityTx(ctx, txBytes, 1)
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
		return &mockTx{memo: "PRIORITY:1"}, nil
	}

	service := NewPriorityTxService(PriorityTxServiceConfig{
		Mempool:           newMockMempool(),
		TxDecoder:         txDecoder,
		Logger:            log.NewNopLogger(),
		ValidatorAddress:  "cronosvaloper1test",
		PreconfirmTimeout: 30 * time.Second,
	})

	t.Run("Position increases with each submission", func(t *testing.T) {
		// Submit first transaction
		result1, err := service.SubmitPriorityTx(ctx, []byte("tx1"), 1)
		require.NoError(t, err)
		require.Equal(t, uint32(1), result1.MempoolPosition)
		require.True(t, result1.Accepted)

		// Submit second transaction - should be position 2
		result2, err := service.SubmitPriorityTx(ctx, []byte("tx2"), 1)
		require.NoError(t, err)
		require.Equal(t, uint32(2), result2.MempoolPosition)
		require.True(t, result2.Accepted)

		// Submit third transaction - should be position 3
		result3, err := service.SubmitPriorityTx(ctx, []byte("tx3"), 1)
		require.NoError(t, err)
		require.Equal(t, uint32(3), result3.MempoolPosition)
		require.True(t, result3.Accepted)
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

func TestTxStatusType_String(t *testing.T) {
	tests := []struct {
		name   string
		status TxStatusType
		want   string
	}{
		{
			name:   "Unknown status",
			status: TxStatusUnknown,
			want:   "unknown",
		},
		{
			name:   "Pending status",
			status: TxStatusPending,
			want:   "pending",
		},
		{
			name:   "Preconfirmed status",
			status: TxStatusPreconfirmed,
			want:   "preconfirmed",
		},
		{
			name:   "Included status",
			status: TxStatusIncluded,
			want:   "included",
		},
		{
			name:   "Rejected status",
			status: TxStatusRejected,
			want:   "rejected",
		},
		{
			name:   "Expired status",
			status: TxStatusExpired,
			want:   "expired",
		},
		{
			name:   "Invalid status",
			status: TxStatusType(99),
			want:   "unknown(99)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.String()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestPriorityTxService_GracefulShutdown(t *testing.T) {
	service := NewPriorityTxService(PriorityTxServiceConfig{
		Mempool:   newMockMempool(),
		TxDecoder: func(txBytes []byte) (sdk.Tx, error) { return &mockTx{}, nil },
		Logger:    log.NewNopLogger(),
	})

	// Submit a transaction to ensure service is working
	ctx := context.Background()
	result, err := service.SubmitPriorityTx(ctx, []byte("test_tx"), 1)
	require.NoError(t, err)
	require.True(t, result.Accepted)

	// Stop the service
	stopped := make(chan struct{})
	go func() {
		service.Stop()
		close(stopped)
	}()

	// Wait for shutdown to complete with timeout
	select {
	case <-stopped:
		// Success - shutdown completed
	case <-time.After(5 * time.Second):
		t.Fatal("service did not stop within timeout")
	}

	// Verify context is canceled
	require.Error(t, service.ctx.Err())
	require.Equal(t, context.Canceled, service.ctx.Err())
}

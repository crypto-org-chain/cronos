package preconfirmation

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// mockTx is a mock transaction for testing
type mockTx struct {
	sdk.Tx
	memo     string
	priority int64
	msgs     []sdk.Msg
}

func (mt *mockTx) GetMemo() string {
	return mt.memo
}

func (mt *mockTx) GetMsgs() []sdk.Msg {
	return mt.msgs
}

// mockPriorityTxSelector is a mock for testing
type mockPriorityTxSelector struct {
	baseapp.TxSelector
}

func (mpts *mockPriorityTxSelector) SelectTxForProposal(ctx context.Context, maxTxBytes, maxBlockGas uint64, memTx sdk.Tx, txBz []byte, gasWanted uint64) bool {
	// Accept all transactions for testing
	return true
}

func (mpts *mockPriorityTxSelector) SelectTxForProposalFast(ctx context.Context, txs [][]byte) [][]byte {
	// For testing purposes, simply return the txs as is
	return txs
}

func TestIsPriorityTx(t *testing.T) {
	tests := []struct {
		name     string
		tx       sdk.Tx
		expected bool
	}{
		{
			name:     "nil transaction",
			tx:       nil,
			expected: false,
		},
		{
			name:     "transaction with PRIORITY: prefix",
			tx:       &mockTx{memo: "PRIORITY:1"},
			expected: true,
		},
		{
			name:     "transaction without priority marker",
			tx:       &mockTx{memo: "regular transaction"},
			expected: false,
		},
		{
			name:     "empty memo",
			tx:       &mockTx{memo: ""},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPriorityTx(tt.tx)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestPriorityTxSelector_SelectTxForProposal(t *testing.T) {
	ctx := context.Background()

	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		// Mock tx decoder
		txStr := string(txBytes)
		if txStr == "priority_tx" {
			return &mockTx{memo: "PRIORITY:1"}, nil
		}
		if txStr == "normal_tx" {
			return &mockTx{memo: "normal"}, nil
		}
		return nil, errors.New("invalid tx")
	}

	validateTx := func(tx sdk.Tx, txBz []byte) error {
		// Mock validation: reject "invalid" txs
		if string(txBz) == "invalid_tx" {
			return errors.New("invalid tx")
		}
		return nil
	}

	mockSelector := &mockPriorityTxSelector{}
	selector := NewPriorityTxSelector(mockSelector, txDecoder, validateTx)

	t.Run("Priority transaction passes validation", func(t *testing.T) {
		tx := &mockTx{memo: "PRIORITY:1"}
		result := selector.SelectTxForProposal(ctx, 1000, 10000, tx, []byte("priority_tx"), 100)
		require.True(t, result)
	})

	t.Run("Normal transaction passes validation", func(t *testing.T) {
		tx := &mockTx{memo: "normal"}
		result := selector.SelectTxForProposal(ctx, 1000, 10000, tx, []byte("normal_tx"), 100)
		require.True(t, result)
	})

	t.Run("Invalid transaction is rejected", func(t *testing.T) {
		result := selector.SelectTxForProposal(ctx, 1000, 10000, nil, []byte("invalid_tx"), 100)
		require.False(t, result)
	})
}

func TestPriorityTxSelector_SelectTxForProposalFast(t *testing.T) {
	ctx := context.Background()

	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		txStr := string(txBytes)
		if txStr == "priority1" || txStr == "priority2" {
			return &mockTx{memo: "PRIORITY:1"}, nil
		}
		if txStr == "normal1" || txStr == "normal2" {
			return &mockTx{memo: "normal"}, nil
		}
		if txStr == "invalid" {
			return nil, errors.New("invalid tx")
		}
		return &mockTx{memo: ""}, nil
	}

	validateTx := func(tx sdk.Tx, txBz []byte) error {
		if string(txBz) == "invalid" {
			return errors.New("invalid tx")
		}
		return nil
	}

	mockSelector := &mockPriorityTxSelector{}
	selector := NewPriorityTxSelector(mockSelector, txDecoder, validateTx)

	t.Run("Priority transactions are moved to front", func(t *testing.T) {
		txs := [][]byte{
			[]byte("normal1"),
			[]byte("priority1"),
			[]byte("normal2"),
			[]byte("priority2"),
		}
		result := selector.SelectTxForProposalFast(ctx, txs)

		require.Len(t, result, 4)

		// Decode and check that priority txs come first
		tx1, _ := txDecoder(result[0])
		tx2, _ := txDecoder(result[1])
		require.True(t, IsPriorityTx(tx1), "First tx should be priority")
		require.True(t, IsPriorityTx(tx2), "Second tx should be priority")
	})

	t.Run("Invalid transactions are filtered out", func(t *testing.T) {
		txs := [][]byte{
			[]byte("normal1"),
			[]byte("invalid"),
			[]byte("priority1"),
		}
		result := selector.SelectTxForProposalFast(ctx, txs)

		require.Len(t, result, 2)
		require.NotContains(t, result, []byte("invalid"))
	})

	t.Run("Empty transaction list", func(t *testing.T) {
		txs := [][]byte{}
		result := selector.SelectTxForProposalFast(ctx, txs)
		require.Empty(t, result)
	})

	t.Run("All priority transactions", func(t *testing.T) {
		txs := [][]byte{
			[]byte("priority1"),
			[]byte("priority2"),
		}
		result := selector.SelectTxForProposalFast(ctx, txs)
		require.Len(t, result, 2)
	})

	t.Run("All normal transactions", func(t *testing.T) {
		txs := [][]byte{
			[]byte("normal1"),
			[]byte("normal2"),
		}
		result := selector.SelectTxForProposalFast(ctx, txs)
		require.Len(t, result, 2)
	})
}

func TestIsPriorityTxBytes(t *testing.T) {
	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		txStr := string(txBytes)
		if txStr == "priority_tx" {
			return &mockTx{memo: "PRIORITY:1"}, nil
		}
		if txStr == "normal_tx" {
			return &mockTx{memo: "normal"}, nil
		}
		return nil, errors.New("invalid tx")
	}

	validateTx := func(tx sdk.Tx, txBz []byte) error {
		return nil
	}

	mockSelector := &mockPriorityTxSelector{}
	selector := NewPriorityTxSelector(mockSelector, txDecoder, validateTx)

	t.Run("Priority transaction bytes", func(t *testing.T) {
		result := selector.IsPriorityTxBytes([]byte("priority_tx"))
		require.True(t, result)
	})

	t.Run("Normal transaction bytes", func(t *testing.T) {
		result := selector.IsPriorityTxBytes([]byte("normal_tx"))
		require.False(t, result)
	})

	t.Run("Invalid transaction bytes", func(t *testing.T) {
		result := selector.IsPriorityTxBytes([]byte("invalid_tx"))
		require.False(t, result)
	})
}

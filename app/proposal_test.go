package app

import (
	"context"
	"errors"
	"testing"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

type mockTxSelector struct {
	baseapp.TxSelector
}

func (mts *mockTxSelector) SelectTxForProposalFast(ctx context.Context, txs [][]byte) [][]byte {
	// For testing purposes, simply return the txs as is
	return txs
}

func TestSelectTxForProposalFast(t *testing.T) {
	ctx := context.Background()

	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		// Mock tx decoder; returns a dummy tx
		return nil, nil
	}

	validateTx := func(tx sdk.Tx, txBz []byte) error {
		// Mock validation logic: return error if txBz is "invalid"
		if string(txBz) == "invalid" {
			return errors.New("invalid tx")
		}
		return nil
	}

	mockSelector := &mockTxSelector{}

	extTxSelector := NewExtTxSelector(mockSelector, txDecoder, validateTx)

	t.Run("Empty transaction list", func(t *testing.T) {
		txs := [][]byte{}
		result := extTxSelector.SelectTxForProposalFast(ctx, txs)
		require.Empty(t, result)
	})

	t.Run("All valid transactions", func(t *testing.T) {
		txs := [][]byte{[]byte("valid1"), []byte("valid2"), []byte("valid3")}
		result := extTxSelector.SelectTxForProposalFast(ctx, txs)
		require.Equal(t, txs, result)
	})

	t.Run("All invalid transactions", func(t *testing.T) {
		txs := [][]byte{[]byte("invalid"), []byte("invalid"), []byte("invalid")}
		result := extTxSelector.SelectTxForProposalFast(ctx, txs)
		require.Empty(t, result)
	})

	t.Run("Mixed valid and invalid transactions", func(t *testing.T) {
		txs := [][]byte{[]byte("valid1"), []byte("invalid"), []byte("valid2"), []byte("invalid"), []byte("valid3")}
		expected := [][]byte{[]byte("valid1"), []byte("valid2"), []byte("valid3")}
		result := extTxSelector.SelectTxForProposalFast(ctx, txs)
		require.Equal(t, expected, result)
	})

	t.Run("Edge cases in the filtering logic", func(t *testing.T) {
		// Edge case: first and last transactions are invalid
		txs := [][]byte{[]byte("invalid"), []byte("valid1"), []byte("valid2"), []byte("invalid")}
		expected := [][]byte{[]byte("valid1"), []byte("valid2")}
		result := extTxSelector.SelectTxForProposalFast(ctx, txs)
		require.Equal(t, expected, result)
	})
}

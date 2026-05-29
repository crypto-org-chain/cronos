package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// stubTx is a minimal sdk.Tx used to verify memTx identity across call boundaries.
type stubTx struct{ sdk.Tx }

// stubTxSelector lets a test control the parent TxSelector return value.
type stubTxSelector struct {
	baseapp.TxSelector
	parentReturn bool
	calls        int
	lastMemTx    sdk.Tx
	lastTxBz     []byte
}

func (s *stubTxSelector) SelectTxForProposal(_ context.Context, _, _ uint64, memTx sdk.Tx, txBz []byte) bool {
	s.calls++
	s.lastMemTx = memTx
	s.lastTxBz = txBz
	return s.parentReturn
}

func TestExtTxSelector_SelectTxForProposal(t *testing.T) {
	txDecoder := func([]byte) (sdk.Tx, error) { return nil, nil }

	rejectInvalid := func(_ sdk.Tx, txBz []byte) error {
		if string(txBz) == "invalid" {
			return errors.New("invalid tx")
		}
		return nil
	}

	t.Run("validation failure short-circuits and parent not called", func(t *testing.T) {
		parent := &stubTxSelector{parentReturn: true}
		ext := NewExtTxSelector(parent, txDecoder, rejectInvalid)
		ok := ext.SelectTxForProposal(context.Background(), 1<<20, 1<<20, nil, []byte("invalid"))
		require.False(t, ok)
		require.Equal(t, 0, parent.calls, "parent must not be invoked when ValidateTx errors")
	})

	t.Run("validation success delegates to parent (true passthrough)", func(t *testing.T) {
		parent := &stubTxSelector{parentReturn: true}
		ext := NewExtTxSelector(parent, txDecoder, rejectInvalid)
		ok := ext.SelectTxForProposal(context.Background(), 1<<20, 1<<20, nil, []byte("valid"))
		require.True(t, ok)
		require.Equal(t, 1, parent.calls)
		require.Nil(t, parent.lastMemTx, "memTx must be nil to bypass parent gas-wanted check")
		require.Equal(t, []byte("valid"), parent.lastTxBz)
	})

	t.Run("validation success delegates to parent (false passthrough)", func(t *testing.T) {
		parent := &stubTxSelector{parentReturn: false}
		ext := NewExtTxSelector(parent, txDecoder, rejectInvalid)
		ok := ext.SelectTxForProposal(context.Background(), 1<<20, 1<<20, nil, []byte("valid"))
		require.False(t, ok)
		require.Equal(t, 1, parent.calls)
	})

	// Validates the key invariant: ValidateTx receives the original memTx while the
	// parent selector receives nil — preventing the parent from enforcing block gas
	// against tx.GetGas() (which conflicts with max-tx-gas-wanted logic).
	t.Run("validates with original memTx but forwards nil to parent", func(t *testing.T) {
		origTx := &stubTx{}
		var capturedValidateTx sdk.Tx
		captureValidate := func(tx sdk.Tx, _ []byte) error {
			capturedValidateTx = tx
			return nil
		}

		parent := &stubTxSelector{parentReturn: true}
		ext := NewExtTxSelector(parent, txDecoder, captureValidate)
		ok := ext.SelectTxForProposal(context.Background(), 1<<20, 1<<20, origTx, []byte("valid"))

		require.True(t, ok)
		require.Same(t, origTx, capturedValidateTx, "ValidateTx must receive the original memTx")
		require.Nil(t, parent.lastMemTx, "parent must receive nil memTx to skip block-gas enforcement")
	})
}


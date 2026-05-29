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

// gasOnlyTx is a minimal sdk.FeeTx whose only non-zero field is gas.
type gasOnlyTx struct {
	sdk.Tx
	gas uint64
}

func (g gasOnlyTx) GetGas() uint64     { return g.gas }
func (g gasOnlyTx) GetFee() sdk.Coins  { return nil }
func (g gasOnlyTx) FeePayer() []byte   { return nil }
func (g gasOnlyTx) FeeGranter() []byte { return nil }

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

// gasCapSelector simulates a parent that enforces maxBlockGas using the
// forwarded memTx — identical to what baseapp.DefaultTxSelector does.
type gasCapSelector struct {
	baseapp.TxSelector
	maxBlockGas uint64
	totalGas    uint64
}

func (s *gasCapSelector) SelectTxForProposal(_ context.Context, _, _ uint64, memTx sdk.Tx, _ []byte) bool {
	feeTx, ok := memTx.(sdk.FeeTx)
	if !ok {
		return true
	}
	want := feeTx.GetGas()
	if want > s.maxBlockGas-s.totalGas {
		return false
	}
	s.totalGas += want
	return true
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

	t.Run("memTx forwarded to parent unmodified after validation passes", func(t *testing.T) {
		// ExtTxSelector must validate with the received memTx and then forward
		// the same value so the parent can enforce maxBlockGas via GetGas().
		parent := &stubTxSelector{parentReturn: true}
		ext := NewExtTxSelector(parent, txDecoder, rejectInvalid)
		sentinel := gasOnlyTx{gas: 42}
		ok := ext.SelectTxForProposal(context.Background(), 1<<20, 1<<20, sentinel, []byte("valid"))
		require.True(t, ok)
		require.Equal(t, 1, parent.calls)
		require.Equal(t, sentinel, parent.lastMemTx, "parent must receive the original memTx so it can read GetGas()")
		require.Equal(t, []byte("valid"), parent.lastTxBz)
	})

	t.Run("parent gas cap blocks tx when memTx gas exceeds remaining budget", func(t *testing.T) {
		// When memTx carries gas > remaining block budget, a gas-aware parent
		// (like baseapp.DefaultTxSelector) returns false. ExtTxSelector must
		// propagate that rejection — it requires forwarding the real memTx.
		parent := &gasCapSelector{maxBlockGas: 100_000}
		ext := NewExtTxSelector(parent, txDecoder, rejectInvalid)

		// First tx: 60_000 gas — fits.
		ok := ext.SelectTxForProposal(context.Background(), 1<<20, 100_000, gasOnlyTx{gas: 60_000}, []byte("tx1"))
		require.True(t, ok)
		require.Equal(t, uint64(60_000), parent.totalGas)

		// Second tx: 60_000 gas — would exceed cap (60k+60k > 100k).
		ok = ext.SelectTxForProposal(context.Background(), 1<<20, 100_000, gasOnlyTx{gas: 60_000}, []byte("tx2"))
		require.False(t, ok, "parent gas cap must reject tx when block gas budget exhausted")
	})

	t.Run("validation success delegates to parent (false passthrough)", func(t *testing.T) {
		parent := &stubTxSelector{parentReturn: false}
		ext := NewExtTxSelector(parent, txDecoder, rejectInvalid)
		ok := ext.SelectTxForProposal(context.Background(), 1<<20, 1<<20, nil, []byte("valid"))
		require.False(t, ok)
		require.Equal(t, 1, parent.calls)
	})

	t.Run("validate receives original memTx", func(t *testing.T) {
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
		require.Same(t, origTx, parent.lastMemTx.(*stubTx), "parent must also receive the original memTx")
	})
}

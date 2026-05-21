package app

import (
	"context"
	"errors"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

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
}

// nonNoOpMempool wraps NoOpMempool so it still satisfies the Mempool interface
// but is NOT type-equal to NoOpMempool. Used to drive fastNoOpPrepareProposal's
// delegation branch.
type nonNoOpMempool struct {
	mempool.NoOpMempool
}

func TestFastNoOpPrepareProposal(t *testing.T) {
	rejectInvalid := func(_ sdk.Tx, txBz []byte) error {
		if string(txBz) == "invalid" {
			return errors.New("invalid tx")
		}
		return nil
	}
	acceptAll := func(_ sdk.Tx, _ []byte) error { return nil }
	mustNotInvoke := func(_ sdk.Context, _ *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		t.Fatal("default handler must not be invoked on fast path")
		return nil, nil
	}

	t.Run("non-NoOp mempool delegates to default handler", func(t *testing.T) {
		var calls int
		want := &abci.ResponsePrepareProposal{Txs: [][]byte{[]byte("from-default")}}
		def := func(_ sdk.Context, _ *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
			calls++
			return want, nil
		}
		h := fastNoOpPrepareProposal(nonNoOpMempool{}, def, rejectInvalid)
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{
			MaxTxBytes: 1 << 20,
			Txs:        [][]byte{[]byte("a")},
		})
		require.NoError(t, err)
		require.Equal(t, 1, calls, "default handler must be invoked for non-NoOp mempool")
		require.Same(t, want, got)
	})

	t.Run("NoOp mempool filters invalid txs and preserves order", func(t *testing.T) {
		h := fastNoOpPrepareProposal(mempool.NoOpMempool{}, mustNotInvoke, rejectInvalid)
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{
			MaxTxBytes: 1 << 20,
			Txs: [][]byte{
				[]byte("ok-1"),
				[]byte("invalid"),
				[]byte("ok-2"),
			},
		})
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("ok-1"), []byte("ok-2")}, got.Txs)
	})

	t.Run("NoOp mempool respects MaxTxBytes and stops at boundary", func(t *testing.T) {
		h := fastNoOpPrepareProposal(mempool.NoOpMempool{}, mustNotInvoke, acceptAll)
		// Each tx is 4 bytes; budget for exactly two.
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{
			MaxTxBytes: 8,
			Txs: [][]byte{
				[]byte("aaaa"),
				[]byte("bbbb"),
				[]byte("cccc"),
			},
		})
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("aaaa"), []byte("bbbb")}, got.Txs)
	})

	t.Run("NoOp mempool with MaxTxBytes <= 0 returns empty proposal", func(t *testing.T) {
		h := fastNoOpPrepareProposal(mempool.NoOpMempool{}, mustNotInvoke, acceptAll)
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{
			MaxTxBytes: 0,
			Txs:        [][]byte{[]byte("a")},
		})
		require.NoError(t, err)
		require.Empty(t, got.Txs)
	})

	t.Run("nil mempool follows fast path", func(t *testing.T) {
		h := fastNoOpPrepareProposal(nil, mustNotInvoke, rejectInvalid)
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{
			MaxTxBytes: 1 << 20,
			Txs:        [][]byte{[]byte("ok"), []byte("invalid")},
		})
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("ok")}, got.Txs)
	})

	t.Run("NoOp mempool with empty req.Txs returns empty proposal", func(t *testing.T) {
		h := fastNoOpPrepareProposal(mempool.NoOpMempool{}, mustNotInvoke, acceptAll)
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{
			MaxTxBytes: 1 << 20,
			Txs:        nil,
		})
		require.NoError(t, err)
		require.Empty(t, got.Txs)
	})
}

package app

import (
	"context"
	"errors"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

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

// gasOnlyTx implements sdk.FeeTx with only a gas value; used to drive the
// MaxBlockGas accounting branch of fastNoOpPrepareProposal.
type gasOnlyTx struct{ gas uint64 }

func (gasOnlyTx) GetMsgs() []sdk.Msg                    { return nil }
func (gasOnlyTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }
func (t gasOnlyTx) GetGas() uint64                      { return t.gas }
func (gasOnlyTx) GetFee() sdk.Coins                     { return nil }
func (gasOnlyTx) FeePayer() []byte                      { return nil }
func (gasOnlyTx) FeeGranter() []byte                    { return nil }

func TestFastNoOpPrepareProposal(t *testing.T) {
	rejectInvalid := func(_ sdk.Tx, txBz []byte) error {
		if string(txBz) == "invalid" {
			return errors.New("invalid tx")
		}
		return nil
	}
	acceptAll := func(_ sdk.Tx, _ []byte) error { return nil }
	noopDecoder := func([]byte) (sdk.Tx, error) { return nil, nil }
	mustNotInvoke := func(t *testing.T) sdk.PrepareProposalHandler {
		t.Helper()
		return func(_ sdk.Context, _ *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
			t.Fatal("default handler must not be invoked on fast path")
			return nil, nil
		}
	}

	t.Run("non-NoOp mempool delegates to default handler", func(t *testing.T) {
		var calls int
		want := &abci.ResponsePrepareProposal{Txs: [][]byte{[]byte("from-default")}}
		def := func(_ sdk.Context, _ *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
			calls++
			return want, nil
		}
		h := fastNoOpPrepareProposal(nonNoOpMempool{}, def, noopDecoder, rejectInvalid)
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{
			MaxTxBytes: 1 << 20,
			Txs:        [][]byte{[]byte("a")},
		})
		require.NoError(t, err)
		require.Equal(t, 1, calls, "default handler must be invoked for non-NoOp mempool")
		require.Same(t, want, got)
	})

	t.Run("NoOp mempool filters invalid txs and preserves order", func(t *testing.T) {
		h := fastNoOpPrepareProposal(mempool.NoOpMempool{}, mustNotInvoke(t), noopDecoder, rejectInvalid)
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
		h := fastNoOpPrepareProposal(mempool.NoOpMempool{}, mustNotInvoke(t), noopDecoder, acceptAll)
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
		h := fastNoOpPrepareProposal(mempool.NoOpMempool{}, mustNotInvoke(t), noopDecoder, acceptAll)
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{
			MaxTxBytes: 0,
			Txs:        [][]byte{[]byte("a")},
		})
		require.NoError(t, err)
		require.Empty(t, got.Txs)
	})

	t.Run("nil mempool follows fast path", func(t *testing.T) {
		h := fastNoOpPrepareProposal(nil, mustNotInvoke(t), noopDecoder, rejectInvalid)
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{
			MaxTxBytes: 1 << 20,
			Txs:        [][]byte{[]byte("ok"), []byte("invalid")},
		})
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("ok")}, got.Txs)
	})

	t.Run("NoOp mempool with empty req.Txs returns empty proposal", func(t *testing.T) {
		h := fastNoOpPrepareProposal(mempool.NoOpMempool{}, mustNotInvoke(t), noopDecoder, acceptAll)
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{
			MaxTxBytes: 1 << 20,
			Txs:        nil,
		})
		require.NoError(t, err)
		require.Empty(t, got.Txs)
	})

	t.Run("NoOp mempool caps total gas at consensus MaxGas", func(t *testing.T) {
		// Each tx wants 50_000 gas; budget for exactly two.
		decoder := func(b []byte) (sdk.Tx, error) {
			return gasOnlyTx{gas: 50_000}, nil
		}
		h := fastNoOpPrepareProposal(mempool.NoOpMempool{}, mustNotInvoke(t), decoder, acceptAll)
		ctx := sdk.Context{}.WithConsensusParams(cmtproto.ConsensusParams{
			Block: &cmtproto.BlockParams{MaxBytes: 1 << 20, MaxGas: 100_000},
		})
		got, err := h(ctx, &abci.RequestPrepareProposal{
			MaxTxBytes: 1 << 20,
			Txs:        [][]byte{[]byte("a"), []byte("b"), []byte("c")},
		})
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("a"), []byte("b")}, got.Txs)
	})

	t.Run("NoOp mempool decode failure skips tx, does not abort", func(t *testing.T) {
		// Decoder errors on "bad" only; gas-cap branch active.
		decoder := func(b []byte) (sdk.Tx, error) {
			if string(b) == "bad" {
				return nil, errors.New("decode err")
			}
			return gasOnlyTx{gas: 1}, nil
		}
		h := fastNoOpPrepareProposal(mempool.NoOpMempool{}, mustNotInvoke(t), decoder, acceptAll)
		ctx := sdk.Context{}.WithConsensusParams(cmtproto.ConsensusParams{
			Block: &cmtproto.BlockParams{MaxBytes: 1 << 20, MaxGas: 1_000_000},
		})
		got, err := h(ctx, &abci.RequestPrepareProposal{
			MaxTxBytes: 1 << 20,
			Txs:        [][]byte{[]byte("ok-1"), []byte("bad"), []byte("ok-2")},
		})
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("ok-1"), []byte("ok-2")}, got.Txs)
	})
}

package app

import (
	"context"
	"errors"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	cronosmempool "github.com/crypto-org-chain/cronos/app/mempool"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

const invalidTx = "invalid"

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
		if string(txBz) == invalidTx {
			return errors.New("invalid tx")
		}
		return nil
	}

	t.Run("validation failure short-circuits and parent not called", func(t *testing.T) {
		parent := &stubTxSelector{parentReturn: true}
		ext := NewExtTxSelector(parent, txDecoder, rejectInvalid)
		ok := ext.SelectTxForProposal(context.Background(), 1<<20, 1<<20, nil, []byte(invalidTx))
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
		if string(txBz) == invalidTx {
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
				[]byte(invalidTx),
				[]byte("ok-2"),
			},
		})
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("ok-1"), []byte("ok-2")}, got.Txs)
	})

	t.Run("NoOp mempool respects MaxTxBytes and stops at boundary", func(t *testing.T) {
		h := fastNoOpPrepareProposal(mempool.NoOpMempool{}, mustNotInvoke(t), noopDecoder, acceptAll)
		// Each "aaaa" payload is 4 bytes. ComputeProtoSizeForTxs computes the
		// proto-framed size: field tag (1) + length varint (1) + data (4) = 6
		// bytes per tx. Budget = 14 → two fit (12), third (18) exceeds.
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{
			MaxTxBytes: 14,
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
			Txs:        [][]byte{[]byte("ok"), []byte(invalidTx)},
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

// fakeIterator is a minimal mempool.Iterator over a fixed slice.
type fakeIterator struct {
	txs []sdk.Tx
}

func (f *fakeIterator) Next() mempool.Iterator {
	if len(f.txs) <= 1 {
		return nil
	}
	return &fakeIterator{txs: f.txs[1:]}
}

func (f *fakeIterator) Tx() sdk.Tx { return f.txs[0] }

// fakeMempool satisfies mempool.Mempool with a fixed list of txs. It does NOT
// implement ExtMempool, so mempool.SelectBy falls back to the Select iterator
// path — exactly the path fastPrepareProposalAppMempool exercises.
type fakeMempool struct {
	txs []sdk.Tx
}

func (m *fakeMempool) Insert(context.Context, sdk.Tx) error { return nil }
func (m *fakeMempool) Select(_ context.Context, _ [][]byte) mempool.Iterator {
	if len(m.txs) == 0 {
		return nil
	}
	return &fakeIterator{txs: m.txs}
}
func (m *fakeMempool) CountTx() int        { return len(m.txs) }
func (m *fakeMempool) Remove(sdk.Tx) error { return nil }

func TestFastPrepareProposalAppMempool(t *testing.T) {
	acceptAll := func(_ sdk.Tx, _ []byte) error { return nil }
	rejectInvalid := func(_ sdk.Tx, txBz []byte) error {
		if string(txBz) == invalidTx {
			return errors.New("invalid tx")
		}
		return nil
	}
	mustNotEncode := func(_ sdk.Tx) ([]byte, error) {
		t.Helper()
		t.Fatal("txEncoder must not be called when encCache hits")
		return nil, nil
	}

	t.Run("encCache hit emits raw bytes without encoder", func(t *testing.T) {
		tx1, tx2 := &gasOnlyTx{gas: 1}, &gasOnlyTx{gas: 1}
		mp := &fakeMempool{txs: []sdk.Tx{tx1, tx2}}
		enc := cronosmempool.NewEncoderCache(0)
		enc.Register(tx1, []byte("raw-1"))
		enc.Register(tx2, []byte("raw-2"))

		h := fastPrepareProposalAppMempool(mp, enc, mustNotEncode, acceptAll)
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{MaxTxBytes: 1 << 20})
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("raw-1"), []byte("raw-2")}, got.Txs)
	})

	t.Run("encoder fallback when encCache miss", func(t *testing.T) {
		tx := &gasOnlyTx{gas: 1}
		mp := &fakeMempool{txs: []sdk.Tx{tx}}
		enc := cronosmempool.NewEncoderCache(0) // empty — forces fallback

		var encCalls int
		fallback := func(_ sdk.Tx) ([]byte, error) {
			encCalls++
			return []byte("encoded"), nil
		}

		h := fastPrepareProposalAppMempool(mp, enc, fallback, acceptAll)
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{MaxTxBytes: 1 << 20})
		require.NoError(t, err)
		require.Equal(t, 1, encCalls)
		require.Equal(t, [][]byte{[]byte("encoded")}, got.Txs)
	})

	t.Run("encoder error skips tx, continues iteration", func(t *testing.T) {
		txBad, txGood := &gasOnlyTx{gas: 1}, &gasOnlyTx{gas: 1}
		mp := &fakeMempool{txs: []sdk.Tx{txBad, txGood}}
		enc := cronosmempool.NewEncoderCache(0)
		enc.Register(txGood, []byte("good"))

		fallback := func(tx sdk.Tx) ([]byte, error) {
			if tx == txBad {
				return nil, errors.New("encode err")
			}
			t.Fatal("encoder must not be called for cached tx")
			return nil, nil
		}

		h := fastPrepareProposalAppMempool(mp, enc, fallback, acceptAll)
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{MaxTxBytes: 1 << 20})
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("good")}, got.Txs)
	})

	t.Run("validateTx reject skips tx, continues iteration", func(t *testing.T) {
		tx1, tx2, tx3 := &gasOnlyTx{gas: 1}, &gasOnlyTx{gas: 1}, &gasOnlyTx{gas: 1}
		mp := &fakeMempool{txs: []sdk.Tx{tx1, tx2, tx3}}
		enc := cronosmempool.NewEncoderCache(0)
		enc.Register(tx1, []byte("ok-1"))
		enc.Register(tx2, []byte(invalidTx))
		enc.Register(tx3, []byte("ok-2"))

		h := fastPrepareProposalAppMempool(mp, enc, mustNotEncode, rejectInvalid)
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{MaxTxBytes: 1 << 20})
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("ok-1"), []byte("ok-2")}, got.Txs)
	})

	t.Run("MaxTxBytes cap stops iteration at boundary", func(t *testing.T) {
		// Each "raw-X" is 5 bytes. ComputeProtoSizeForTxs computes the size of
		// a proto-encoded Block.Data.Txs entry: field tag (1) + length varint
		// (1) + data (5) = 7 bytes per tx. Budget = 18 → two fit (14), third
		// (21) exceeds.
		tx1, tx2, tx3 := &gasOnlyTx{gas: 1}, &gasOnlyTx{gas: 1}, &gasOnlyTx{gas: 1}
		mp := &fakeMempool{txs: []sdk.Tx{tx1, tx2, tx3}}
		enc := cronosmempool.NewEncoderCache(0)
		enc.Register(tx1, []byte("raw-1"))
		enc.Register(tx2, []byte("raw-2"))
		enc.Register(tx3, []byte("raw-3"))

		h := fastPrepareProposalAppMempool(mp, enc, mustNotEncode, acceptAll)
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{MaxTxBytes: 18})
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("raw-1"), []byte("raw-2")}, got.Txs)
	})

	t.Run("MaxBlockGas cap stops iteration when gas exceeded", func(t *testing.T) {
		tx1 := &gasOnlyTx{gas: 60_000}
		tx2 := &gasOnlyTx{gas: 60_000} // 60k+60k > 100k cap → reject + stop
		tx3 := &gasOnlyTx{gas: 1}      // never reached
		mp := &fakeMempool{txs: []sdk.Tx{tx1, tx2, tx3}}
		enc := cronosmempool.NewEncoderCache(0)
		enc.Register(tx1, []byte("a"))
		enc.Register(tx2, []byte("b"))
		enc.Register(tx3, []byte("c"))

		ctx := sdk.Context{}.WithConsensusParams(cmtproto.ConsensusParams{
			Block: &cmtproto.BlockParams{MaxBytes: 1 << 20, MaxGas: 100_000},
		})
		h := fastPrepareProposalAppMempool(mp, enc, mustNotEncode, acceptAll)
		got, err := h(ctx, &abci.RequestPrepareProposal{MaxTxBytes: 1 << 20})
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("a")}, got.Txs)
	})

	t.Run("MaxTxBytes <= 0 returns empty proposal", func(t *testing.T) {
		tx := &gasOnlyTx{gas: 1}
		mp := &fakeMempool{txs: []sdk.Tx{tx}}
		enc := cronosmempool.NewEncoderCache(0)
		enc.Register(tx, []byte("a"))

		h := fastPrepareProposalAppMempool(mp, enc, mustNotEncode, acceptAll)
		got, err := h(sdk.Context{}, &abci.RequestPrepareProposal{MaxTxBytes: 0})
		require.NoError(t, err)
		require.Empty(t, got.Txs)
	})
}

// TestProtoSizeForTx asserts the inlined wire-size helper stays bit-identical to
// cometbft's ComputeProtoSizeForTxs for a single tx. This is the invariant that
// lets fastNoOpPrepareProposal / fastPrepareProposalAppMempool drop the per-tx
// []Tx{}+ToProto allocation: if the two ever diverge, proposals would mis-account
// the MaxBytes wire budget. Covers varint length boundaries (1/2/3-byte lengths).
func TestProtoSizeForTx(t *testing.T) {
	for _, n := range []int{0, 1, 2, 127, 128, 129, 300, 16383, 16384, 16385, 70000} {
		bz := make([]byte, n)
		want := cmttypes.ComputeProtoSizeForTxs([]cmttypes.Tx{bz})
		got := cronosmempool.ProtoSizeForTx(bz)
		require.Equalf(t, want, got, "protoSizeForTx mismatch at len=%d", n)
	}
}

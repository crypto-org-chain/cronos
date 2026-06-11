package app

import (
	"context"
	"errors"
	"math/big"
	"testing"

	cmttypes "github.com/cometbft/cometbft/types"
	cronosmempool "github.com/crypto-org-chain/cronos/app/mempool"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const invalidTx = "invalid"

// stubTx is a minimal sdk.Tx used to verify memTx identity across call boundaries.
type stubTx struct{ sdk.Tx }

func TestExtTxSelector(t *testing.T) {
	const maxB = 1 << 20
	bg := context.Background()
	acceptAll := func(_ sdk.Tx, _ []byte) error { return nil }
	rejectInvalid := func(_ sdk.Tx, txBz []byte) error {
		if string(txBz) == invalidTx {
			return errors.New("invalid tx")
		}
		return nil
	}

	t.Run("blocklist reject skips tx, keeps scanning", func(t *testing.T) {
		ts := NewExtTxSelector(rejectInvalid, nil)
		ts.SelectTxForProposal(bg, maxB, maxB, nil, []byte(invalidTx))
		ts.SelectTxForProposal(bg, maxB, maxB, nil, []byte("ok"))
		require.Equal(t, [][]byte{[]byte("ok")}, ts.SelectedTxs(bg))
	})

	t.Run("validate receives original memTx", func(t *testing.T) {
		orig := &stubTx{}
		var captured sdk.Tx
		ts := NewExtTxSelector(func(tx sdk.Tx, _ []byte) error { captured = tx; return nil }, nil)
		ts.SelectTxForProposal(bg, maxB, maxB, orig, []byte("ok"))
		require.Same(t, orig, captured)
	})

	t.Run("MaxTxBytes: too-large tx skipped, block fills to budget", func(t *testing.T) {
		// "raw-X" framed = tag(1)+len(1)+5 = 7 bytes. Budget 14 → two fit.
		ts := NewExtTxSelector(acceptAll, nil)
		require.False(t, ts.SelectTxForProposal(bg, 14, maxB, nil, []byte("raw-1")))
		require.True(t, ts.SelectTxForProposal(bg, 14, maxB, nil, []byte("raw-2")), "full at 14 bytes")
		ts.SelectTxForProposal(bg, 14, maxB, nil, []byte("raw-3")) // would overflow → skipped
		require.Equal(t, [][]byte{[]byte("raw-1"), []byte("raw-2")}, ts.SelectedTxs(bg))
	})

	t.Run("MaxBlockGas: over-budget tx skipped, smaller still fits", func(t *testing.T) {
		ts := NewExtTxSelector(acceptAll, nil)
		ts.SelectTxForProposal(bg, maxB, 100_000, gasOnlyTx{gas: 60_000}, []byte("a"))
		ts.SelectTxForProposal(bg, maxB, 100_000, gasOnlyTx{gas: 60_000}, []byte("b")) // 60k+60k>100k → skip
		ts.SelectTxForProposal(bg, maxB, 100_000, gasOnlyTx{gas: 40_000}, []byte("c")) // 60k+40k=100k → fits
		require.Equal(t, [][]byte{[]byte("a"), []byte("c")}, ts.SelectedTxs(bg))
	})

	t.Run("baseFee gate excludes feeCap below baseFee", func(t *testing.T) {
		const denom = "basecro"
		feeFn := func(_ sdk.Context) (*big.Int, string) { return big.NewInt(20), denom }
		ts := NewExtTxSelector(acceptAll, feeFn)
		txLow := feeCapTx{gas: 10, fee: sdk.NewCoins(sdk.NewInt64Coin(denom, 100))} // 10 < 20 → excluded
		txOK := feeCapTx{gas: 10, fee: sdk.NewCoins(sdk.NewInt64Coin(denom, 400))}  // 40 >= 20 → kept
		ts.SelectTxForProposal(sdk.Context{}, maxB, maxB, txLow, []byte("low"))
		ts.SelectTxForProposal(sdk.Context{}, maxB, maxB, txOK, []byte("ok"))
		require.Equal(t, [][]byte{[]byte("ok")}, ts.SelectedTxs(bg))
	})

	t.Run("nil baseFee disables the gate", func(t *testing.T) {
		feeFn := func(_ sdk.Context) (*big.Int, string) { return nil, "basecro" }
		ts := NewExtTxSelector(acceptAll, feeFn)
		txLow := feeCapTx{gas: 10, fee: sdk.NewCoins(sdk.NewInt64Coin("basecro", 1))}
		ts.SelectTxForProposal(sdk.Context{}, maxB, maxB, txLow, []byte("low"))
		require.Equal(t, [][]byte{[]byte("low")}, ts.SelectedTxs(bg))
	})

	t.Run("Clear resets selection and baseFee cache", func(t *testing.T) {
		ts := NewExtTxSelector(acceptAll, nil)
		ts.SelectTxForProposal(bg, maxB, maxB, nil, []byte("a"))
		ts.Clear()
		require.Empty(t, ts.SelectedTxs(bg))
		require.False(t, ts.feeReady)
	})
}

// gasOnlyTx implements sdk.FeeTx with only a gas value.
type gasOnlyTx struct{ gas uint64 }

func (gasOnlyTx) GetMsgs() []sdk.Msg                    { return nil }
func (gasOnlyTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }
func (t gasOnlyTx) GetGas() uint64                      { return t.gas }
func (gasOnlyTx) GetFee() sdk.Coins                     { return nil }
func (gasOnlyTx) FeePayer() []byte                      { return nil }
func (gasOnlyTx) FeeGranter() []byte                    { return nil }

// feeCapTx is an sdk.FeeTx with a real fee + gas, used to drive the baseFee gate
// (feeCap = fee/gas).
type feeCapTx struct {
	gas uint64
	fee sdk.Coins
}

func (feeCapTx) GetMsgs() []sdk.Msg                    { return nil }
func (feeCapTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }
func (t feeCapTx) GetGas() uint64                      { return t.gas }
func (t feeCapTx) GetFee() sdk.Coins                   { return t.fee }
func (feeCapTx) FeePayer() []byte                      { return nil }
func (feeCapTx) FeeGranter() []byte                    { return nil }

func TestCacheProposalTxVerifier(t *testing.T) {
	// Hit path only: encCache.Get returns cached bytes without touching BaseApp
	// (nil here). The miss path is a trivial txv.TxEncode delegation.
	t.Run("encCache hit returns cached bytes without encoding", func(t *testing.T) {
		enc := cronosmempool.NewEncoderCache(0)
		tx := &gasOnlyTx{gas: 1}
		enc.Set(tx, []byte("raw"))
		v := &CacheProposalTxVerifier{encCache: enc}
		bz, err := v.PrepareProposalVerifyTx(tx)
		require.NoError(t, err)
		require.Equal(t, []byte("raw"), bz)
	})
}

func TestProtoSizeForTx(t *testing.T) {
	for _, n := range []int{0, 1, 2, 127, 128, 129, 300, 16383, 16384, 16385, 70000} {
		bz := make([]byte, n)
		want := cmttypes.ComputeProtoSizeForTxs([]cmttypes.Tx{bz})
		got := cronosmempool.ProtoSizeForTx(bz)
		require.Equalf(t, want, got, "protoSizeForTx mismatch at len=%d", n)
	}
}

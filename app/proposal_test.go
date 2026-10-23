package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"testing"

	"filippo.io/age"
	cmttypes "github.com/cometbft/cometbft/types"
	cronosmempool "github.com/crypto-org-chain/cronos/app/mempool"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethermintante "github.com/evmos/ethermint/ante"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	protov2 "google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const invalidTx = "invalid"

// stubTx is a minimal sdk.Tx used to verify memTx identity across call boundaries.
type stubTx struct{ sdk.Tx }

func (*stubTx) GetMsgs() []sdk.Msg { return nil }

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

	t.Run("gate-skipped bytes accumulated in skipped, accepted bytes excluded", func(t *testing.T) {
		const denom = "basecro"
		feeFn := func(_ sdk.Context) (*big.Int, string) { return big.NewInt(20), denom }
		ts := NewExtTxSelector(acceptAll, feeFn)
		txLow := feeCapTx{gas: 10, fee: sdk.NewCoins(sdk.NewInt64Coin(denom, 100))} // feeCap=10 < 20 → skip
		txOK := feeCapTx{gas: 10, fee: sdk.NewCoins(sdk.NewInt64Coin(denom, 400))}  // feeCap=40 >= 20 → select
		ts.SelectTxForProposal(sdk.Context{}, maxB, maxB, txLow, []byte("low"))
		ts.SelectTxForProposal(sdk.Context{}, maxB, maxB, txOK, []byte("ok"))
		ts.Clear()
		require.Equal(t, [][]byte{[]byte("low")}, ts.DrainSkipped())
		require.Nil(t, ts.DrainSkipped(), "second drain must be nil")
	})

	t.Run("DrainSkipped on empty selector is nil", func(t *testing.T) {
		ts := NewExtTxSelector(acceptAll, nil)
		require.Nil(t, ts.DrainSkipped())
	})

	t.Run("DrainSkipped returns only the last cleared round, so an undrained selector stays bounded", func(t *testing.T) {
		// DefaultProposalHandler defers Clear() inside inner(), so it fires before
		// our wrapper calls DrainSkipped().
		const denom = "basecro"
		feeFn := func(_ sdk.Context) (*big.Int, string) { return big.NewInt(20), denom }
		ts := NewExtTxSelector(acceptAll, feeFn)
		txLow := feeCapTx{gas: 10, fee: sdk.NewCoins(sdk.NewInt64Coin(denom, 100))}
		ts.SelectTxForProposal(sdk.Context{}, maxB, maxB, txLow, []byte("low1"))
		ts.Clear()
		ts.SelectTxForProposal(sdk.Context{}, maxB, maxB, txLow, []byte("low2"))
		ts.Clear()
		require.Equal(t, [][]byte{[]byte("low2")}, ts.DrainSkipped())
		require.Nil(t, ts.DrainSkipped(), "DrainSkipped itself must null the slice")
	})

	t.Run("nil baseFee disables the gate", func(t *testing.T) {
		feeFn := func(_ sdk.Context) (*big.Int, string) { return nil, "basecro" }
		ts := NewExtTxSelector(acceptAll, feeFn)
		txLow := feeCapTx{gas: 10, fee: sdk.NewCoins(sdk.NewInt64Coin("basecro", 1))}
		ts.SelectTxForProposal(sdk.Context{}, maxB, maxB, txLow, []byte("low"))
		require.Equal(t, [][]byte{[]byte("low")}, ts.SelectedTxs(bg))
	})

	t.Run("Clear resets selection, baseFee cache and selected eth nonces", func(t *testing.T) {
		ts := NewExtTxSelector(acceptAll, nil)
		tx := newEthMsgsTx(newEthMsg(common.HexToAddress("0xa"), 7))
		ts.SelectTxForProposal(bg, maxB, maxB, tx, []byte("a"))
		ts.Clear()
		require.Empty(t, ts.SelectedTxs(bg))
		require.Nil(t, ts.baseFee)
		ts.SelectTxForProposal(bg, maxB, maxB, tx, []byte("a"))
		require.Equal(t, [][]byte{[]byte("a")}, ts.SelectedTxs(bg), "next round may select the same nonce again")
	})
}

type ExtTxSelectorEthDedupSuite struct {
	suite.Suite
}

func TestExtTxSelectorEthDedupSuite(t *testing.T) {
	suite.Run(t, new(ExtTxSelectorEthDedupSuite))
}

func (s *ExtTxSelectorEthDedupSuite) TestSelectTxForProposal() {
	const (
		maxB  = 1 << 20
		bzB1A = "B1,A"
	)
	var (
		a, b1 = common.HexToAddress("0xa"), common.HexToAddress("0xb1")
		msgA  = newEthMsg(a, 7)
	)
	type input struct {
		tx sdk.Tx
		bz string
	}
	testCases := []struct {
		name        string
		maxGas      uint64 // 0 means maxB
		inputs      []input
		expSelected []string
		expSkipped  []string
	}{
		{
			name: "distinct sender nonces are all selected",
			inputs: []input{
				{newEthMsgsTx(msgA), "A"},
				{newEthMsgsTx(newEthMsg(a, 8)), "A8"},
				{newEthMsgsTx(newEthMsg(b1, 7)), "B1"},
			},
			expSelected: []string{"A", "A8", "B1"},
		},
		{
			name: "batches reusing a selected message are skipped",
			inputs: []input{
				{newEthMsgsTx(msgA), "A"},
				{newEthMsgsTx(newEthMsg(b1, 0), msgA), bzB1A},
			},
			expSelected: []string{"A"},
			expSkipped:  []string{bzB1A},
		},
		{
			name: "single tx reusing a selected batch's later message is skipped",
			inputs: []input{
				{newEthMsgsTx(newEthMsg(b1, 0), msgA), bzB1A},
				{newEthMsgsTx(msgA), "A"},
			},
			expSelected: []string{bzB1A},
			expSkipped:  []string{"A"},
		},
		{
			name: "dedup-skipped tx does not reserve its other nonces",
			inputs: []input{
				{newEthMsgsTx(msgA), "A"},
				{newEthMsgsTx(newEthMsg(b1, 0), msgA), bzB1A},
				{newEthMsgsTx(newEthMsg(b1, 0)), "B1"},
			},
			expSelected: []string{"A", "B1"},
			expSkipped:  []string{bzB1A},
		},
		{
			name:   "gas-skipped tx does not reserve its nonces",
			maxGas: 10,
			inputs: []input{
				{ethMsgsTx{gasOnlyTx{gas: 20}, []sdk.Msg{msgA}}, "A-big"},
				{newEthMsgsTx(msgA), "A"},
			},
			expSelected: []string{"A"},
		},
		{
			name:       "duplicate within one tx is skipped",
			inputs:     []input{{newEthMsgsTx(msgA, msgA), "A,A"}},
			expSkipped: []string{"A,A"},
		},
		{
			name: "non-eth txs are not deduplicated",
			inputs: []input{
				{gasOnlyTx{gas: 1}, "x"},
				{gasOnlyTx{gas: 1}, "y"},
			},
			expSelected: []string{"x", "y"},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			ts := NewExtTxSelector(func(sdk.Tx, []byte) error { return nil }, nil)
			maxGas := tc.maxGas
			if maxGas == 0 {
				maxGas = maxB
			}
			for _, in := range tc.inputs {
				ts.SelectTxForProposal(context.Background(), maxB, maxGas, in.tx, []byte(in.bz))
			}
			s.Require().Equal(tc.expSelected, toStrings(ts.SelectedTxs(context.Background())))
			ts.Clear()
			s.Require().Equal(tc.expSkipped, toStrings(ts.DrainSkipped()))
		})
	}
}

// toStrings maps empty input to nil so expectations can leave the field unset.
func toStrings(bzs [][]byte) []string {
	if len(bzs) == 0 {
		return nil
	}
	out := make([]string, len(bzs))
	for i, bz := range bzs {
		out[i] = string(bz)
	}
	return out
}

func newEthMsg(from common.Address, nonce uint64) *evmtypes.MsgEthereumTx {
	msg := &evmtypes.MsgEthereumTx{From: from.Bytes()}
	msg.Raw.Transaction = ethtypes.NewTx(&ethtypes.LegacyTx{Nonce: nonce})
	return msg
}

// ethMsgsTx is a FeeTx carrying msgs; zero gas leaves only the dedup gate in play.
type ethMsgsTx struct {
	gasOnlyTx
	msgs []sdk.Msg
}

func newEthMsgsTx(msgs ...sdk.Msg) ethMsgsTx { return ethMsgsTx{msgs: msgs} }

func (t ethMsgsTx) GetMsgs() []sdk.Msg { return t.msgs }

// The fee-cap rule itself is ethermint's ante.EthTxReplacement, tested there; this covers its wiring.
func TestTxReplacement(t *testing.T) {
	priced := func(gasPrice int64) ethMsgsTx {
		msg := &evmtypes.MsgEthereumTx{}
		msg.Raw.Transaction = ethtypes.NewTx(&ethtypes.LegacyTx{GasPrice: big.NewInt(gasPrice)})
		return newEthMsgsTx(msg)
	}
	cosmos := ethMsgsTx{msgs: []sdk.Msg{&banktypes.MsgSend{}}}
	for _, tc := range []struct {
		name     string
		feeBump  int64
		disabled bool
		oTx, nTx sdk.Tx
		exp      bool
	}{
		{"fee bump met", 10, false, priced(1000), priced(1100), true},
		{"fee bump not met", 10, false, priced(1000), priced(1099), false},
		{"zero fee bump still needs a higher fee", 0, false, priced(1000), priced(1000), false},
		{"disabled", 10, true, priced(1000), priced(9000), false},
		{"cosmos tx never replaces a cosmos tx", 10, false, cosmos, cosmos, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The rule ignores the pool priorities.
			require.Equal(t, tc.exp, txReplacement(tc.feeBump, tc.disabled)(1, 0, tc.oTx, tc.nTx))
		})
	}
}

func TestEthTxPool(t *testing.T) {
	priorityMempool := mempool.NewPriorityMempool(mempool.DefaultPriorityNonceMempoolConfig())
	for _, tc := range []struct {
		name     string
		mpool    mempool.Mempool
		disabled bool
		expPool  ethermintante.TxPool
	}{
		{"priority nonce mempool", priorityMempool, false, priorityMempool},
		{"replacement disabled", priorityMempool, true, nil},
		{"no-op mempool", mempool.NoOpMempool{}, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Compare the interfaces directly: a typed nil would not equal nil.
			require.True(t, ethTxPool(tc.mpool, tc.disabled) == tc.expPool)
		})
	}
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
		enc := cronosmempool.NewEncoderCache(0, 0)
		tx := &gasOnlyTx{gas: 1}
		enc.Set(tx, []byte("raw"))
		v := &CacheProposalTxVerifier{encCache: enc}
		bz, err := v.PrepareProposalVerifyTx(tx)
		require.NoError(t, err)
		require.Equal(t, []byte("raw"), bz)
	})
}

func encryptBlockList(t *testing.T, recipient age.Recipient, addrs ...string) []byte {
	t.Helper()
	body, err := json.Marshal(BlockList{Addresses: addrs})
	require.NoError(t, err)

	dst := bytes.NewBuffer(nil)
	w, err := age.Encrypt(dst, recipient)
	require.NoError(t, err)
	_, err = w.Write(body)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return dst.Bytes()
}

func TestSetBlockListRetriesAfterFailure(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	addressCodec := authcodec.NewBech32Codec("cosmos")
	h := NewProposalHandler(nil, identity, addressCodec)

	blocked, err := addressCodec.BytesToString(bytes.Repeat([]byte{0x1}, 20))
	require.NoError(t, err)

	corrupt := []byte("not a valid age ciphertext")
	require.Error(t, h.SetBlockList(corrupt))

	// Same blob again: must retry the decrypt/parse instead of caching the
	// prior failure and short-circuiting on the equality check.
	require.Error(t, h.SetBlockList(corrupt))
	require.Empty(t, h.blocklist)

	valid := encryptBlockList(t, identity.Recipient(), blocked)
	require.NoError(t, h.SetBlockList(valid))
	_, ok := h.blocklist[blocked]
	require.True(t, ok)
}

func TestProtoSizeForTx(t *testing.T) {
	for _, n := range []int{0, 1, 2, 127, 128, 129, 300, 16383, 16384, 16385, 70000} {
		bz := make([]byte, n)
		want := cmttypes.ComputeProtoSizeForTxs([]cmttypes.Tx{bz})
		got := cronosmempool.ProtoSizeForTx(bz)
		require.Equalf(t, want, got, "protoSizeForTx mismatch at len=%d", n)
	}
}

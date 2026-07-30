package app

import (
	"fmt"
	"math/big"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cronosmempool "github.com/crypto-org-chain/cronos/app/mempool"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/baseapp"
	authcodec "github.com/cosmos/cosmos-sdk/codec/address"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
)

// The fast PrepareProposal path (mempool.type=app with the encoder cache) trusts
// admission + recheck and only encodes each pooled tx, where the default path
// re-runs the ante for every candidate. The tests below run both over identically
// seeded pools and pin down where the two are allowed to disagree: the fast path
// may propose a tx the ante would reject, and it leaves rejected txs in the pool
// for recheck instead of evicting them during the proposal.
//
// Every difference here is bounded by what the proposal contract permits: cronos
// ProcessProposal is blocklist-only, so an ante-invalid tx cannot make peers reject
// the block — FinalizeBlock records it as a failed tx result.

const (
	txAlice0 = "alice-0"
	txAlice1 = "alice-1"
	txAlice2 = "alice-2"
	txBob0   = "bob-0"

	diffDenom  = "basecro"
	diffMaxTxB = 1 << 20
	diffMaxGas = 100_000_000
)

// diffTx carries everything both paths need — signer and nonce for ordering, fee
// and gas for the baseFee gate, a timeout height for the expiry case — so neither
// needs a real codec or account keeper. Its raw bytes are its name, keeping a
// proposal's tx set readable in assertions.
type diffTx struct {
	name     string
	signer   sdk.AccAddress
	seq      uint64
	gas      uint64
	fee      int64
	timeout  uint64
	priority int64
}

var (
	_ sdk.FeeTx               = (*diffTx)(nil)
	_ sdk.TxWithTimeoutHeight = (*diffTx)(nil)
)

func (t *diffTx) GetMsgs() []sdk.Msg                                   { return nil }
func (t *diffTx) GetMsgsV2() ([]protov2.Message, error)                { return nil, nil }
func (t *diffTx) GetGas() uint64                                       { return t.gas }
func (t *diffTx) GetFee() sdk.Coins                                    { return sdk.NewCoins(sdk.NewInt64Coin(diffDenom, t.fee)) }
func (t *diffTx) FeePayer() []byte                                     { return t.signer }
func (t *diffTx) FeeGranter() []byte                                   { return nil }
func (t *diffTx) GetTimeoutHeight() uint64                             { return t.timeout }
func (t *diffTx) GetSigners() ([][]byte, error)                        { return [][]byte{t.signer}, nil }
func (t *diffTx) GetPubKeys() ([]cryptotypes.PubKey, error)            { return nil, nil }
func (t *diffTx) GetSignaturesV2() ([]signingtypes.SignatureV2, error) { return nil, nil }

func diffAddr(name string) sdk.AccAddress { return sdk.AccAddress(fmt.Sprintf("%-20s", name)) }

type diffSignerExtractor struct{}

func (diffSignerExtractor) GetSigners(tx sdk.Tx) ([]mempool.SignerData, error) {
	dt, ok := tx.(*diffTx)
	if !ok {
		return nil, fmt.Errorf("unexpected tx type %T", tx)
	}
	return []mempool.SignerData{mempool.NewSignerData(dt.signer, dt.seq)}, nil
}

// diffVerifier is the codec half of baseapp.ProposalTxVerifier for diffTx. ante
// stands in for the default path's RunTx(PrepareProposal); nil means encode-only,
// which is what the cached fast path does.
type diffVerifier struct {
	byName map[string]*diffTx
	ante   func(*diffTx) error
}

func (v *diffVerifier) TxEncode(tx sdk.Tx) ([]byte, error) {
	dt, ok := tx.(*diffTx)
	if !ok {
		return nil, fmt.Errorf("unexpected tx type %T", tx)
	}
	return []byte(dt.name), nil
}

func (v *diffVerifier) TxDecode(bz []byte) (sdk.Tx, error) {
	dt, ok := v.byName[string(bz)]
	if !ok {
		return nil, fmt.Errorf("unknown tx %q", bz)
	}
	return dt, nil
}

func (v *diffVerifier) PrepareProposalVerifyTx(tx sdk.Tx) ([]byte, error) {
	bz, err := v.TxEncode(tx)
	if err != nil {
		return nil, err
	}
	if v.ante != nil {
		if err := v.ante(tx.(*diffTx)); err != nil {
			return nil, err
		}
	}
	return bz, nil
}

func (v *diffVerifier) ProcessProposalVerifyTx(bz []byte) (sdk.Tx, error) {
	tx, err := v.TxDecode(bz)
	if err != nil {
		return nil, err
	}
	if v.ante != nil {
		if err := v.ante(tx.(*diffTx)); err != nil {
			return nil, err
		}
	}
	return tx, nil
}

func diffCtx(height int64) sdk.Context {
	return sdk.NewContext(nil, cmtproto.Header{Height: height}, false, log.NewNopLogger()).
		WithConsensusParams(cmtproto.ConsensusParams{Block: &cmtproto.BlockParams{MaxGas: diffMaxGas}})
}

// newDiffPool seeds a fresh pool per path, since the default path evicts the txs
// its ante rejects and would otherwise leak that into the other path's run.
func newDiffPool(t *testing.T, ctx sdk.Context, txs []*diffTx) *mempool.PriorityNonceMempool[int64] {
	t.Helper()
	pool := mempool.NewPriorityMempool(mempool.PriorityNonceMempoolConfig[int64]{
		TxPriority:      mempool.NewDefaultTxPriority(),
		SignerExtractor: diffSignerExtractor{},
	})
	for _, tx := range txs {
		require.NoError(t, pool.Insert(ctx.WithPriority(tx.priority), tx))
	}
	return pool
}

func diffNames(txs []*diffTx) map[string]*diffTx {
	byName := make(map[string]*diffTx, len(txs))
	for _, tx := range txs {
		byName[tx.name] = tx
	}
	return byName
}

func rawNames(raw [][]byte) []string {
	names := make([]string, len(raw))
	for i, bz := range raw {
		names[i] = string(bz)
	}
	return names
}

func poolNames(t *testing.T, ctx sdk.Context, pool mempool.Mempool) []string {
	t.Helper()
	var names []string
	for _, tx := range cronosmempool.PoolSnapshot(ctx, pool) {
		dt, ok := tx.(*diffTx)
		require.True(t, ok)
		names = append(names, dt.name)
	}
	return names
}

func acceptAllTxs(_ sdk.Tx, _ []byte) error { return nil }

// runFastPath drives the production fast path: CacheProposalTxVerifier over a
// pre-warmed encoder cache (so no tx re-runs the ante) plus the cronos wrapper.
func runFastPath(t *testing.T, ctx sdk.Context, txs []*diffTx, feeGate func(sdk.Context) (*big.Int, string)) ([]string, []string) {
	t.Helper()
	pool := newDiffPool(t, ctx, txs)
	base := &diffVerifier{byName: diffNames(txs)}
	encCache := cronosmempool.NewEncoderCache(0, 0)
	for _, tx := range txs {
		encCache.Set(tx, []byte(tx.name))
	}
	inner := baseapp.NewDefaultProposalHandler(pool, NewCacheProposalTxVerifier(base, encCache))
	h := NewMempoolProposalHandler(inner, acceptAllTxs, feeGate, diffSignerExtractor{})
	resp, err := h.PrepareProposalHandler()(ctx, &abci.RequestPrepareProposal{MaxTxBytes: diffMaxTxB, Height: ctx.BlockHeight()})
	require.NoError(t, err)
	return rawNames(resp.Txs), poolNames(t, ctx, pool)
}

// runAntePath drives the default handler with a full-ante verifier, the
// configuration used when the encoder cache is disabled.
func runAntePath(t *testing.T, ctx sdk.Context, txs []*diffTx, ante func(*diffTx) error) ([]string, []string) {
	t.Helper()
	pool := newDiffPool(t, ctx, txs)
	h := baseapp.NewDefaultProposalHandler(pool, &diffVerifier{byName: diffNames(txs), ante: ante})
	h.SetTxSelector(NewExtTxSelector(acceptAllTxs, nil))
	h.SetSignerExtractionAdapter(diffSignerExtractor{})
	resp, err := h.PrepareProposalHandler()(ctx, &abci.RequestPrepareProposal{MaxTxBytes: diffMaxTxB, Height: ctx.BlockHeight()})
	require.NoError(t, err)
	return rawNames(resp.Txs), poolNames(t, ctx, pool)
}

// requireProcessProposalAccepts runs the real cronos ProcessProposal over a
// proposal, with a non-empty blocklist so the per-tx validation actually runs.
func requireProcessProposalAccepts(t *testing.T, ctx sdk.Context, txs []*diffTx, proposal []string) {
	t.Helper()
	base := &diffVerifier{byName: diffNames(txs)}
	codec := authcodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	blocked, err := codec.BytesToString(diffAddr("mallory"))
	require.NoError(t, err)
	ph := NewProposalHandler(base.TxDecode, nil, codec)
	ph.blocklist = map[string]struct{}{blocked: {}}

	raw := make([][]byte, len(proposal))
	for i, name := range proposal {
		raw[i] = []byte(name)
	}
	resp, err := ph.ProcessProposalHandler()(ctx, &abci.RequestProcessProposal{Txs: raw, Height: ctx.BlockHeight()})
	require.NoError(t, err)
	require.Equal(t, abci.ResponseProcessProposal_ACCEPT, resp.Status)
}

func TestProposalPathsDiff(t *testing.T) {
	alice, bob := diffAddr("alice"), diffAddr("bob")
	ctx := diffCtx(10)
	acceptAllAnte := func(*diffTx) error { return nil }

	t.Run("all pooled txs valid: paths agree on selection and pool", func(t *testing.T) {
		txs := []*diffTx{
			{name: txAlice0, signer: alice, seq: 0, gas: 21_000, fee: 21_000, priority: 5},
			{name: txAlice1, signer: alice, seq: 1, gas: 21_000, fee: 21_000, priority: 5},
			{name: txBob0, signer: bob, seq: 0, gas: 21_000, fee: 21_000, priority: 9},
		}
		fastSel, fastPool := runFastPath(t, ctx, txs, nil)
		anteSel, antePool := runAntePath(t, ctx, txs, acceptAllAnte)
		require.Equal(t, anteSel, fastSel)
		require.ElementsMatch(t, antePool, fastPool)
		require.Len(t, fastSel, 3)
	})

	t.Run("nonce gap: both paths stop at the gap, neither evicts", func(t *testing.T) {
		// The gap guard lives in DefaultProposalHandler's per-signer sequence
		// tracking, which both paths share, so this must not diverge.
		txs := []*diffTx{
			{name: txAlice0, signer: alice, seq: 0, gas: 21_000, fee: 21_000, priority: 5},
			{name: txAlice2, signer: alice, seq: 2, gas: 21_000, fee: 21_000, priority: 5},
		}
		fastSel, fastPool := runFastPath(t, ctx, txs, nil)
		anteSel, antePool := runAntePath(t, ctx, txs, acceptAllAnte)
		require.Equal(t, []string{txAlice0}, fastSel)
		require.Equal(t, anteSel, fastSel)
		require.ElementsMatch(t, []string{txAlice0, txAlice2}, fastPool)
		require.ElementsMatch(t, antePool, fastPool)
	})

	t.Run("stale nonce: fast path proposes it, ante path drops and evicts it", func(t *testing.T) {
		// alice-0 was already committed; recheck hasn't evicted it yet.
		txs := []*diffTx{
			{name: txAlice0, signer: alice, seq: 0, gas: 21_000, fee: 21_000, priority: 5},
			{name: txAlice1, signer: alice, seq: 1, gas: 21_000, fee: 21_000, priority: 5},
		}
		staleBelow1 := func(tx *diffTx) error {
			if tx.seq < 1 {
				return fmt.Errorf("account sequence mismatch: got %d, expected 1", tx.seq)
			}
			return nil
		}
		fastSel, fastPool := runFastPath(t, ctx, txs, nil)
		anteSel, antePool := runAntePath(t, ctx, txs, staleBelow1)

		require.Equal(t, []string{txAlice0, txAlice1}, fastSel)
		require.Equal(t, []string{txAlice1}, anteSel, "ante rejects the committed nonce")
		require.ElementsMatch(t, []string{txAlice0, txAlice1}, fastPool, "fast path leaves eviction to recheck")
		require.Equal(t, []string{txAlice1}, antePool, "ante path evicts during the proposal")
		requireProcessProposalAccepts(t, ctx, txs, fastSel)
	})

	t.Run("recheck backlog: fast path proposes the whole stale prefix", func(t *testing.T) {
		// Worst case of the above: a block committed three of alice's txs and the
		// async recheck hasn't run, so every pooled tx is stale.
		txs := []*diffTx{
			{name: txAlice0, signer: alice, seq: 0, gas: 21_000, fee: 21_000, priority: 5},
			{name: txAlice1, signer: alice, seq: 1, gas: 21_000, fee: 21_000, priority: 5},
			{name: txAlice2, signer: alice, seq: 2, gas: 21_000, fee: 21_000, priority: 5},
			{name: txBob0, signer: bob, seq: 0, gas: 21_000, fee: 21_000, priority: 9},
		}
		allStale := func(tx *diffTx) error {
			if tx.signer.Equals(alice) {
				return fmt.Errorf("account sequence mismatch: got %d, expected 3", tx.seq)
			}
			return nil
		}
		fastSel, fastPool := runFastPath(t, ctx, txs, nil)
		anteSel, antePool := runAntePath(t, ctx, txs, allStale)

		require.ElementsMatch(t, []string{txAlice0, txAlice1, txAlice2, txBob0}, fastSel)
		require.Equal(t, []string{txBob0}, anteSel)
		require.Len(t, fastPool, 4)
		require.Equal(t, []string{txBob0}, antePool)
		requireProcessProposalAccepts(t, ctx, txs, fastSel)
	})

	t.Run("baseFee drift: paths agree on selection, differ on eviction", func(t *testing.T) {
		// The gate replaces the ante's fee check on the fast path, so the selections
		// match; only the pool side effect differs.
		txs := []*diffTx{
			{name: txAlice0, signer: alice, seq: 0, gas: 100, fee: 1_000, priority: 5}, // feeCap 10
			{name: txBob0, signer: bob, seq: 0, gas: 100, fee: 10_000, priority: 9},    // feeCap 100
		}
		gate := func(sdk.Context) (*big.Int, string) { return big.NewInt(50), diffDenom }
		lowFeeRejected := func(tx *diffTx) error {
			if tx.fee/int64(tx.gas) < 50 {
				return fmt.Errorf("insufficient fee: feeCap %d below baseFee 50", tx.fee/int64(tx.gas))
			}
			return nil
		}
		fastSel, fastPool := runFastPath(t, ctx, txs, gate)
		anteSel, antePool := runAntePath(t, ctx, txs, lowFeeRejected)

		require.Equal(t, []string{txBob0}, fastSel)
		require.Equal(t, anteSel, fastSel)
		require.ElementsMatch(t, []string{txAlice0, txBob0}, fastPool, "gated tx stays pooled for a later block")
		require.Equal(t, []string{txBob0}, antePool)
	})

	t.Run("timeout height: fast path proposes the expired tx", func(t *testing.T) {
		// Timeout eviction is recheck's job on the fast path; the selector doesn't
		// look at timeout height, so an expired tx survives until recheck runs.
		txs := []*diffTx{
			{name: txAlice0, signer: alice, seq: 0, gas: 21_000, fee: 21_000, priority: 5, timeout: 5},
			{name: txBob0, signer: bob, seq: 0, gas: 21_000, fee: 21_000, priority: 9},
		}
		timedOut := func(tx *diffTx) error {
			if tx.timeout > 0 && uint64(ctx.BlockHeight()) >= tx.timeout {
				return fmt.Errorf("tx timeout height %d exceeded", tx.timeout)
			}
			return nil
		}
		fastSel, fastPool := runFastPath(t, ctx, txs, nil)
		anteSel, antePool := runAntePath(t, ctx, txs, timedOut)

		require.ElementsMatch(t, []string{txAlice0, txBob0}, fastSel)
		require.Equal(t, []string{txBob0}, anteSel)
		require.Len(t, fastPool, 2)
		require.Equal(t, []string{txBob0}, antePool)
		requireProcessProposalAccepts(t, ctx, txs, fastSel)
	})
}

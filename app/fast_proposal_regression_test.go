package app

import (
	"fmt"
	"maps"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cronosmempool "github.com/crypto-org-chain/cronos/app/mempool"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	protov2 "google.golang.org/protobuf/proto"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/baseapp"
	authcodec "github.com/cosmos/cosmos-sdk/codec/address"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
)

// The fast PrepareProposal path (mempool.type=app with the encoder cache) skips
// the ante and keeps ante-rejected txs pooled for recheck to evict. These tests
// pin where it may differ from the default ante path. "known gap" cases record
// txs the fast path proposes that then fail in FinalizeBlock without paying a
// fee; a filter that closes a gap should flip its case, not be read as a regression.

type PrepareProposalPathsSuite struct {
	suite.Suite
}

func TestPrepareProposalPathsSuite(t *testing.T) {
	suite.Run(t, new(PrepareProposalPathsSuite))
}

type proposalPathResult struct {
	proposed []string
	pooled   []string
}

func (s *PrepareProposalPathsSuite) TestFastPathSkipsAnteAndKeepsRejectedTxsPooled() {
	testCases := []struct {
		name string
		pool []string
		// malleate changes the one field a case is about; txs default to a 1 basecro/gas fee cap.
		malleate func(txs map[string]*diffTx)
		state    anteState
		fast     proposalPathResult
		ante     proposalPathResult
	}{
		{
			name: "all valid: same proposal, nothing evicted",
			pool: []string{txAlice0, txAlice1, txBob0},
			fast: proposalPathResult{proposed: []string{txAlice0, txAlice1, txBob0}, pooled: []string{txAlice0, txAlice1, txBob0}},
			ante: proposalPathResult{proposed: []string{txAlice0, txAlice1, txBob0}, pooled: []string{txAlice0, txAlice1, txBob0}},
		},
		{
			name: "nonce gap: both stop at the gap, nothing evicted",
			pool: []string{txAlice0, txAlice2},
			fast: proposalPathResult{proposed: []string{txAlice0}, pooled: []string{txAlice0, txAlice2}},
			ante: proposalPathResult{proposed: []string{txAlice0}, pooled: []string{txAlice0, txAlice2}},
		},
		{
			name:  "known gap: stale nonce not yet rechecked, fast proposes it, ante drops and evicts it",
			pool:  []string{txAlice0, txAlice1},
			state: anteState{accountSeq: map[string]uint64{alice: 1}},
			fast:  proposalPathResult{proposed: []string{txAlice0, txAlice1}, pooled: []string{txAlice0, txAlice1}},
			ante:  proposalPathResult{proposed: []string{txAlice1}, pooled: []string{txAlice1}},
		},
		{
			name:  "known gap: recheck backlog, fast proposes the whole stale prefix",
			pool:  []string{txAlice0, txAlice1, txAlice2, txBob0},
			state: anteState{accountSeq: map[string]uint64{alice: 3}},
			fast:  proposalPathResult{proposed: []string{txAlice0, txAlice1, txAlice2, txBob0}, pooled: []string{txAlice0, txAlice1, txAlice2, txBob0}},
			ante:  proposalPathResult{proposed: []string{txBob0}, pooled: []string{txBob0}},
		},
		{
			name: "blocklisted signer selected first: both skip it and keep selecting, neither evicts it",
			pool: []string{txAlice0, txMallory0},
			// Higher priority puts mallory-0 ahead of alice-0, so a selector that stopped at the rejection fails.
			malleate: func(txs map[string]*diffTx) { txs[txMallory0].priority = 10 },
			fast:     proposalPathResult{proposed: []string{txAlice0}, pooled: []string{txAlice0, txMallory0}},
			ante:     proposalPathResult{proposed: []string{txAlice0}, pooled: []string{txAlice0, txMallory0}},
		},
		{
			name:     "baseFee above a fee cap: both skip it, only ante evicts it",
			pool:     []string{txAlice0, txBob0},
			malleate: func(txs map[string]*diffTx) { txs[txBob0].fee *= 100 },
			state:    anteState{baseFee: 50},
			fast:     proposalPathResult{proposed: []string{txBob0}, pooled: []string{txAlice0, txBob0}},
			ante:     proposalPathResult{proposed: []string{txBob0}, pooled: []string{txBob0}},
		},
		{
			name:     "known gap: timeout height passed, fast proposes the expired tx",
			pool:     []string{txAlice0, txBob0},
			malleate: func(txs map[string]*diffTx) { txs[txAlice0].timeout = diffHeight - 1 },
			fast:     proposalPathResult{proposed: []string{txAlice0, txBob0}, pooled: []string{txAlice0, txBob0}},
			ante:     proposalPathResult{proposed: []string{txBob0}, pooled: []string{txBob0}},
		},
		{
			name:     "timeout height equals the block height: both propose it",
			pool:     []string{txAlice0},
			malleate: func(txs map[string]*diffTx) { txs[txAlice0].timeout = diffHeight },
			fast:     proposalPathResult{proposed: []string{txAlice0}, pooled: []string{txAlice0}},
			ante:     proposalPathResult{proposed: []string{txAlice0}, pooled: []string{txAlice0}},
		},
		{
			name:  "known gap: only a stale tx pooled, fast proposes it, ante proposes nothing and evicts it",
			pool:  []string{txAlice0},
			state: anteState{accountSeq: map[string]uint64{alice: 1}},
			fast:  proposalPathResult{proposed: []string{txAlice0}, pooled: []string{txAlice0}},
			ante:  proposalPathResult{},
		},
		{
			name:  "known gap: lowest pooled nonce ahead of the account, fast proposes it, ante proposes nothing and evicts it",
			pool:  []string{txAlice2},
			state: anteState{accountSeq: map[string]uint64{alice: 1}},
			fast:  proposalPathResult{proposed: []string{txAlice2}, pooled: []string{txAlice2}},
			ante:  proposalPathResult{},
		},
	}

	for _, tc := range testCases {
		for _, warm := range []bool{true, false} {
			s.Run(fmt.Sprintf("%s/warm_cache=%t", tc.name, warm), func() {
				txs := newDiffTxs(tc.pool...)
				if tc.malleate != nil {
					tc.malleate(txs)
				}
				h := newProposalPathHarness(s.T(), txs)

				s.Require().Equal(tc.fast, h.runFastPath(tc.state, warm), "fast path")
				s.Require().Equal(tc.ante, h.runAntePath(tc.state), "ante path")
			})
		}
	}
}

const (
	alice    = "alice"
	txAlice0 = "alice-0"
	txAlice1 = "alice-1"
	txAlice2 = "alice-2"
	txBob0   = "bob-0"
	// txMallory0's signer is blocklisted.
	txMallory0 = "mallory-0"

	diffDenom  = "basecro"
	diffHeight = 10
	diffGas    = 21_000
	diffMaxTxB = 1 << 20
	diffMaxGas = 100_000_000
)

// diffTx is named "<signer>-<nonce>" and its raw bytes are that name, so cases
// read as tx names and neither path needs a real codec or account keeper.
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

func newDiffTxs(names ...string) map[string]*diffTx {
	txs := make(map[string]*diffTx, len(names))
	for _, name := range names {
		signer, nonce, ok := strings.Cut(name, "-")
		if !ok {
			panic("tx name must be <signer>-<nonce>: " + name)
		}
		seq, err := strconv.ParseUint(nonce, 10, 64)
		if err != nil {
			panic(err)
		}
		txs[name] = &diffTx{name: name, signer: diffAddr(signer), seq: seq, gas: diffGas, fee: diffGas}
	}
	return txs
}

type diffSignerExtractor struct{}

func (diffSignerExtractor) GetSigners(tx sdk.Tx) ([]mempool.SignerData, error) {
	dt, ok := tx.(*diffTx)
	if !ok {
		return nil, fmt.Errorf("unexpected tx type %T", tx)
	}
	return []mempool.SignerData{mempool.NewSignerData(dt.signer, dt.seq)}, nil
}

// anteState is the chain state the modeled ante checks txs against.
type anteState struct {
	// accountSeq is each signer's next sequence; a missing signer is at 0.
	accountSeq map[string]uint64
	// baseFee in basecro/gas; zero disables both the ante fee check and the fast-path gate.
	baseFee int64
}

// diffVerifier is baseapp.ProposalTxVerifier for diffTx, with state standing in
// for the default path's RunTx(PrepareProposal) ante.
type diffVerifier struct {
	txs   map[string]*diffTx
	state anteState
	// nextSeq starts at accountSeq and advances per accepted tx, as the ante's
	// sequence increments carry across txs within one proposal.
	nextSeq map[string]uint64
	// failEncode makes any tx not served from the encoder cache fail.
	failEncode bool
}

func newDiffVerifier(txs map[string]*diffTx, state anteState) *diffVerifier {
	nextSeq := make(map[string]uint64, len(state.accountSeq))
	maps.Copy(nextSeq, state.accountSeq)
	return &diffVerifier{txs: txs, state: state, nextSeq: nextSeq}
}

// anteRejects models the ante checks the cases exercise: a sequence other than
// the account's next one, a timeout height below the block height
// (TxTimeoutHeightDecorator), and a fee cap below baseFee.
func (v *diffVerifier) anteRejects(tx *diffTx) bool {
	signer, _, _ := strings.Cut(tx.name, "-")
	if tx.seq != v.nextSeq[signer] ||
		(tx.timeout != 0 && diffHeight > tx.timeout) ||
		tx.fee < v.state.baseFee*int64(tx.gas) {
		return true
	}
	v.nextSeq[signer] = tx.seq + 1
	return false
}

func (v *diffVerifier) TxEncode(tx sdk.Tx) ([]byte, error) {
	dt, ok := tx.(*diffTx)
	if !ok {
		return nil, fmt.Errorf("unexpected tx type %T", tx)
	}
	if v.failEncode {
		return nil, fmt.Errorf("encoder called for %s", dt.name)
	}
	return []byte(dt.name), nil
}

func (v *diffVerifier) TxDecode(bz []byte) (sdk.Tx, error) {
	dt, ok := v.txs[string(bz)]
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
	if v.anteRejects(tx.(*diffTx)) {
		return nil, fmt.Errorf("ante rejected %s", bz)
	}
	return bz, nil
}

func (v *diffVerifier) ProcessProposalVerifyTx(bz []byte) (sdk.Tx, error) {
	return v.TxDecode(bz)
}

type proposalPathHarness struct {
	t   *testing.T
	ctx sdk.Context
	txs map[string]*diffTx
	// validateTx is the production blocklist check, with mallory blocklisted.
	validateTx func(sdk.Tx, []byte) error
}

func newProposalPathHarness(t *testing.T, txs map[string]*diffTx) *proposalPathHarness {
	t.Helper()
	ctx := sdk.NewContext(nil, cmtproto.Header{Height: diffHeight}, false, log.NewNopLogger()).
		WithConsensusParams(cmtproto.ConsensusParams{Block: &cmtproto.BlockParams{MaxGas: diffMaxGas}})
	codec := authcodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	blocked, err := codec.BytesToString(diffAddr("mallory"))
	require.NoError(t, err)
	ph := NewProposalHandler(nil, nil, codec)
	ph.blocklist = map[string]struct{}{blocked: {}}
	return &proposalPathHarness{t: t, ctx: ctx, txs: txs, validateTx: ph.ValidateTransaction}
}

// newPool seeds a fresh pool per path, since the ante path evicts what it rejects.
func (h *proposalPathHarness) newPool() *mempool.PriorityNonceMempool[int64] {
	pool := mempool.NewPriorityMempool(mempool.PriorityNonceMempoolConfig[int64]{
		TxPriority:      mempool.NewDefaultTxPriority(),
		SignerExtractor: diffSignerExtractor{},
	})
	for _, tx := range h.txs {
		require.NoError(h.t, pool.Insert(h.ctx.WithPriority(tx.priority), tx))
	}
	return pool
}

// runFastPath drives CacheProposalTxVerifier plus the cronos wrapper. Its base
// verifier gets the same ante state, so a fast path that ran the ante would fail.
// A warm cache holds every tx, as after admission, and the encoder then fails so
// only cache hits pass; a cold one, as after LRU eviction, takes the miss branch.
func (h *proposalPathHarness) runFastPath(state anteState, warm bool) proposalPathResult {
	pool := h.newPool()
	encCache := cronosmempool.NewEncoderCache(0, 0)
	base := newDiffVerifier(h.txs, state)
	if warm {
		for name, tx := range h.txs {
			encCache.Set(tx, []byte(name))
		}
		base.failEncode = true
	}
	var feeGate func(sdk.Context) (*big.Int, string)
	if state.baseFee > 0 {
		feeGate = func(sdk.Context) (*big.Int, string) { return big.NewInt(state.baseFee), diffDenom }
	}
	inner := baseapp.NewDefaultProposalHandler(pool, NewCacheProposalTxVerifier(base, encCache))
	ph := NewMempoolProposalHandler(inner, h.validateTx, feeGate, diffSignerExtractor{})
	return h.prepare(ph.PrepareProposalHandler(), pool)
}

// runAntePath drives the default handler with a full-ante verifier, as when the
// encoder cache is disabled.
func (h *proposalPathHarness) runAntePath(state anteState) proposalPathResult {
	pool := h.newPool()
	ph := baseapp.NewDefaultProposalHandler(pool, newDiffVerifier(h.txs, state))
	ph.SetTxSelector(NewExtTxSelector(h.validateTx, nil))
	ph.SetSignerExtractionAdapter(diffSignerExtractor{})
	return h.prepare(ph.PrepareProposalHandler(), pool)
}

func (h *proposalPathHarness) prepare(prepare sdk.PrepareProposalHandler, pool mempool.Mempool) proposalPathResult {
	resp, err := prepare(h.ctx, &abci.RequestPrepareProposal{MaxTxBytes: diffMaxTxB, Height: diffHeight})
	require.NoError(h.t, err)
	// Append from nil so an empty result equals proposalPathResult{}.
	var res proposalPathResult
	for _, bz := range resp.Txs {
		res.proposed = append(res.proposed, string(bz))
	}
	for _, tx := range cronosmempool.PoolSnapshot(h.ctx, pool) {
		res.pooled = append(res.pooled, tx.(*diffTx).name)
	}
	// Sort by signer only: a sender's nonce order must hold, the order across senders isn't under test.
	slices.SortStableFunc(res.proposed, bySigner)
	slices.SortStableFunc(res.pooled, bySigner)
	return res
}

func bySigner(a, b string) int {
	sa, _, _ := strings.Cut(a, "-")
	sb, _, _ := strings.Cut(b, "-")
	return strings.Compare(sa, sb)
}

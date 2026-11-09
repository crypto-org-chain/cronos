package app

import (
	"context"
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cronosmempool "github.com/crypto-org-chain/cronos/app/mempool"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	protov2 "google.golang.org/protobuf/proto"

	"cosmossdk.io/log/v2"

	authcodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
)

// The fast PrepareProposal path (mempool.type=app with the encoder cache) skips
// the ante and keeps ante-rejected txs pooled for recheck to evict. These tests
// pin where it may differ from the default ante path. "known gap" cases record
// txs the fast path proposes that then fail in FinalizeBlock: a cheap filter that
// closes one should update the case, not treat it as a regression.

type PrepareProposalPathsSuite struct {
	suite.Suite
}

func TestPrepareProposalPathsSuite(t *testing.T) {
	suite.Run(t, new(PrepareProposalPathsSuite))
}

type proposalPathResult struct {
	proposed []string
	pooled   []string
	// staged is what the fast path hands the mempool manager to recheck; the ante path has no manager.
	staged []string
}

func (s *PrepareProposalPathsSuite) TestFastPathSkipsAnteAndKeepsRejectedTxsPooled() {
	testCases := []struct {
		name string
		pool []string
		// malleate changes the one field a case is about; txs default to a 1 basecro/gas fee cap.
		malleate func(txs map[string]*diffTx)
		// selectedFirst is the tx the pool must yield first, for cases that depend on selection order.
		selectedFirst string
		baseFee       int64
		anteRejects   []string
		fast, ante    proposalPathResult
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
			name:        "known gap: stale nonce not yet rechecked: fast proposes it, ante drops and evicts it",
			pool:        []string{txAlice0, txAlice1},
			anteRejects: []string{txAlice0},
			fast:        proposalPathResult{proposed: []string{txAlice0, txAlice1}, pooled: []string{txAlice0, txAlice1}},
			ante:        proposalPathResult{proposed: []string{txAlice1}, pooled: []string{txAlice1}},
		},
		{
			name:        "known gap: recheck backlog: fast proposes the whole stale prefix",
			pool:        []string{txAlice0, txAlice1, txAlice2, txBob0},
			anteRejects: []string{txAlice0, txAlice1, txAlice2},
			fast:        proposalPathResult{proposed: []string{txAlice0, txAlice1, txAlice2, txBob0}, pooled: []string{txAlice0, txAlice1, txAlice2, txBob0}},
			ante:        proposalPathResult{proposed: []string{txBob0}, pooled: []string{txBob0}},
		},
		{
			name: "blocklisted signer selected first: both skip it and keep selecting, neither evicts it",
			pool: []string{txAlice0, txMallory0},
			// Higher priority puts mallory-0 ahead of alice-0, so a selector that stopped at the rejection fails.
			malleate:      func(txs map[string]*diffTx) { txs[txMallory0].priority = 10 },
			selectedFirst: txMallory0,
			fast:          proposalPathResult{proposed: []string{txAlice0}, pooled: []string{txAlice0, txMallory0}},
			ante:          proposalPathResult{proposed: []string{txAlice0}, pooled: []string{txAlice0, txMallory0}},
		},
		{
			name:     "baseFee above a fee cap: both skip the sender, only fast stages it for recheck, only ante evicts it",
			pool:     []string{txAlice0, txAlice1, txBob0},
			malleate: func(txs map[string]*diffTx) { txs[txBob0].fee *= 100 },
			baseFee:  50,
			// alice-1 fails the ante on sequence once alice-0 is dropped.
			anteRejects: []string{txAlice0, txAlice1},
			fast:        proposalPathResult{proposed: []string{txBob0}, pooled: []string{txAlice0, txAlice1, txBob0}, staged: []string{txAlice0}},
			ante:        proposalPathResult{proposed: []string{txBob0}, pooled: []string{txBob0}},
		},
		{
			name:        "known gap: timeout height passed: fast proposes the expired tx",
			pool:        []string{txAlice0, txBob0},
			malleate:    func(txs map[string]*diffTx) { txs[txAlice0].timeout = diffHeight - 1 },
			anteRejects: []string{txAlice0},
			fast:        proposalPathResult{proposed: []string{txAlice0, txBob0}, pooled: []string{txAlice0, txBob0}},
			ante:        proposalPathResult{proposed: []string{txBob0}, pooled: []string{txBob0}},
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
				if tc.selectedFirst != "" {
					s.Require().Equal(tc.selectedFirst, h.selectionOrder()[0], "selection order")
				}

				s.Require().Equal(tc.fast, h.runFastPath(tc.baseFee, tc.anteRejects, warm), "fast path")
				s.Require().Equal(tc.ante, h.runAntePath(tc.anteRejects), "ante path")
			})
		}
	}
}

const (
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
	_ signing.SigVerifiableTx = (*diffTx)(nil)
)

func (t *diffTx) GetMsgs() []sdk.Msg                    { return nil }
func (t *diffTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }
func (t *diffTx) GetGas() uint64                        { return t.gas }
func (t *diffTx) GetFee() sdk.Coins                     { return sdk.NewCoins(sdk.NewInt64Coin(diffDenom, t.fee)) }
func (t *diffTx) FeePayer() []byte                      { return t.signer }
func (t *diffTx) FeeGranter() []byte                    { return nil }
func (t *diffTx) GetTimeoutHeight() uint64              { return t.timeout }
func (t *diffTx) GetSigners() ([][]byte, error)         { return [][]byte{t.signer}, nil }
func (t *diffTx) GetPubKeys() ([]cryptotypes.PubKey, error) {
	return []cryptotypes.PubKey{t.pubKey()}, nil
}

// GetSignaturesV2 carries the key the pool's PubKeyRequire adapter needs and the
// sequence both adapters key the sender's nonce by.
func (t *diffTx) GetSignaturesV2() ([]signingtypes.SignatureV2, error) {
	return []signingtypes.SignatureV2{{PubKey: t.pubKey(), Sequence: t.seq}}, nil
}

func (t *diffTx) pubKey() cryptotypes.PubKey {
	return secp256k1.GenPrivKeyFromSecret(t.signer).PubKey()
}

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

// diffVerifier is baseapp.ProposalTxVerifier for diffTx; rejects stands in for
// the txs the default path's RunTx(PrepareProposal) ante would fail.
type diffVerifier struct {
	txs     map[string]*diffTx
	rejects map[string]bool
	// failEncode makes any tx not served from the encoder cache fail.
	failEncode bool
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
	if v.rejects[string(bz)] {
		return nil, fmt.Errorf("ante rejected %s", bz)
	}
	return bz, nil
}

func (v *diffVerifier) ProcessProposalVerifyTx(bz []byte) (sdk.Tx, error) {
	if v.rejects[string(bz)] {
		return nil, fmt.Errorf("ante rejected %s", bz)
	}
	return v.TxDecode(bz)
}

// recordingStager stands in for the mempool manager and records what the fast path hands it.
type recordingStager struct {
	waits  int
	staged []string
}

func (r *recordingStager) WaitForRecheckTimedOut(context.Context, time.Duration) bool {
	r.waits++
	return false
}

func (r *recordingStager) StageSkippedSenders(txs [][]byte) {
	for _, bz := range txs {
		r.staged = append(r.staged, string(bz))
	}
}

type proposalPathHarness struct {
	t   *testing.T
	ctx sdk.Context
	txs map[string]*diffTx
	// validateTx is the production blocklist check, with mallory blocklisted.
	validateTx func(sdk.Tx, []byte) error
	// handlerSigner and poolSigner are the production adapters.
	handlerSigner, poolSigner mempool.SignerExtractionAdapter
}

func newProposalPathHarness(t *testing.T, txs map[string]*diffTx) *proposalPathHarness {
	t.Helper()
	ctx := sdk.NewContext(nil, cmtproto.Header{Height: diffHeight}, false, log.NewNopLogger()).
		WithConsensusParams(cmtproto.ConsensusParams{Block: &cmtproto.BlockParams{MaxGas: diffMaxGas}})
	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	codec := authcodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	blocked, err := codec.BytesToString(diffAddr("mallory"))
	require.NoError(t, err)
	ph := NewProposalHandler(nil, identity, codec)
	require.NoError(t, ph.SetBlockList(encryptBlockList(t, identity.Recipient(), blocked)))
	handlerSigner, poolSigner := mempoolSignerExtractors()
	return &proposalPathHarness{
		t: t, ctx: ctx, txs: txs, validateTx: ph.ValidateTransaction,
		handlerSigner: handlerSigner, poolSigner: poolSigner,
	}
}

// newPool seeds a fresh pool per path, since the ante path evicts what it rejects.
func (h *proposalPathHarness) newPool() *mempool.PriorityNonceMempool[int64] {
	pool := mempool.NewPriorityMempool(mempool.PriorityNonceMempoolConfig[int64]{
		TxPriority:      mempool.NewDefaultTxPriority(),
		SignerExtractor: h.poolSigner,
	})
	for _, tx := range h.txs {
		require.NoError(h.t, pool.Insert(h.ctx.WithPriority(tx.priority), tx))
	}
	return pool
}

// selectionOrder is the order a fresh pool yields txs to the proposal handlers.
func (h *proposalPathHarness) selectionOrder() []string {
	var names []string
	for _, tx := range cronosmempool.PoolSnapshot(h.ctx, h.newPool()) {
		names = append(names, tx.(*diffTx).name)
	}
	return names
}

// runFastPath drives the app's handler with the encoder cache. Its base verifier
// gets the same rejects, so a fast path that ran the ante would fail. A warm
// cache holds every tx, as after admission, and the encoder then fails so only
// cache hits pass; a cold one, as after LRU eviction, takes the miss branch.
// A zero baseFee disables the gate.
func (h *proposalPathHarness) runFastPath(baseFee int64, rejects []string, warm bool) proposalPathResult {
	pool := h.newPool()
	encCache := cronosmempool.NewEncoderCache(0, 0)
	base := h.verifier(rejects)
	if warm {
		for name, tx := range h.txs {
			encCache.Set(tx, []byte(name))
		}
		base.failEncode = true
	}
	var feeGate func(sdk.Context) (*big.Int, string)
	if baseFee > 0 {
		feeGate = func(sdk.Context) (*big.Int, string) { return big.NewInt(baseFee), diffDenom }
	}
	prepare, ph := newPrepareProposalHandler(pool, base, encCache, h.validateTx, feeGate, h.handlerSigner)
	require.NotNil(h.t, ph)
	stager := &recordingStager{}
	ph.SetMempoolManager(stager)
	res := h.prepare(prepare, pool)
	require.Equal(h.t, 1, stager.waits, "recheck wait")
	res.staged = stager.staged
	return res
}

// runAntePath drives the app's handler without the encoder cache, which runs the full ante.
func (h *proposalPathHarness) runAntePath(rejects []string) proposalPathResult {
	pool := h.newPool()
	prepare, ph := newPrepareProposalHandler(pool, h.verifier(rejects), nil, h.validateTx, nil, h.handlerSigner)
	require.Nil(h.t, ph)
	return h.prepare(prepare, pool)
}

func (h *proposalPathHarness) verifier(rejects []string) *diffVerifier {
	v := &diffVerifier{txs: h.txs, rejects: make(map[string]bool, len(rejects))}
	for _, name := range rejects {
		v.rejects[name] = true
	}
	return v
}

func (h *proposalPathHarness) prepare(prepare sdk.PrepareProposalHandler, pool mempool.Mempool) proposalPathResult {
	resp, err := prepare(h.ctx, &abci.RequestPrepareProposal{MaxTxBytes: diffMaxTxB, Height: diffHeight})
	require.NoError(h.t, err)
	res := proposalPathResult{proposed: make([]string, len(resp.Txs))}
	for i, bz := range resp.Txs {
		res.proposed[i] = string(bz)
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

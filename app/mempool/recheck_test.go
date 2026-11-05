package mempool

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/ethereum/go-ethereum/common"

	errorsmod "cosmossdk.io/errors"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// fakeSigner maps a tx pointer to fixed signer(s), sidestepping real signature
// extraction (ptrTx isn't a SigVerifiableTx).
type fakeSigner struct {
	m map[sdk.Tx][]sdkmempool.SignerData
}

func (f fakeSigner) GetSigners(tx sdk.Tx) ([]sdkmempool.SignerData, error) {
	sd, ok := f.m[tx]
	if !ok {
		return nil, errors.New("no signer for tx")
	}
	return sd, nil
}

// recheckRunner records RunTx calls.
type recheckRunner struct {
	mu                sync.Mutex
	pool              sdkmempool.Mempool
	failBytes         map[string]bool
	failNoRemoveBytes map[string]bool
	panicBytes        map[string]bool
	// failErrs returns a specific error per tx bytes, without removing from the
	// pool, so tests can drive runRecheck's nonce-gap classification.
	failErrs map[string]error
	modes    []sdk.ExecMode
	seen     map[string]bool
	// calls records tx bytes in call order, for grouping assertions.
	calls []string
	// onCall, if set, runs after recording the call but before returning, letting
	// a test move a sender's expected nonce mid-pass (an admission or Commit
	// landing between chunks or groups).
	onCall func(txBytes []byte)
	// signer + expectedNonce implement per-sender expected-nonce ante
	// semantics: a tx whose seq is above its sender's expected nonce fails
	// wrong-sequence, and a successful recheck advances that sender's expected
	// nonce. Nil signer disables this; other tests drive failBytes/failErrs.
	signer        sdkmempool.SignerExtractionAdapter
	expectedNonce map[string]uint64
	// nonceSpan maps tx bytes to how many consecutive nonces the tx consumes
	// (a batch eth tx); absent means 1.
	nonceSpan map[string]uint64
}

func (r *recheckRunner) RunTx(mode sdk.ExecMode, txBytes []byte, tx sdk.Tx, _ int, _ storetypes.MultiStore, _ map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modes = append(r.modes, mode)
	r.seen[string(txBytes)] = true
	r.calls = append(r.calls, string(txBytes))
	if r.onCall != nil {
		r.onCall(txBytes)
	}
	if r.panicBytes[string(txBytes)] {
		panicking("RunTx")
	}

	var sender string
	var seq uint64
	trackNonce := false
	if r.signer != nil {
		if sigs, err := r.signer.GetSigners(tx); err == nil && len(sigs) > 0 {
			sender, seq, trackNonce = sigs[0].Signer.String(), sigs[0].Sequence, true
		}
	}
	// The real ante rejects any mismatch, not just a gap: a stale nonce
	// (seq < expected) is just as invalid as a gap (seq > expected).
	if trackNonce && seq != r.expectedNonce[sender] {
		return sdk.GasInfo{}, nil, nil, errorsmod.Wrap(sdkerrors.ErrWrongSequence, "nonce mismatch")
	}

	if err, ok := r.failErrs[string(txBytes)]; ok {
		return sdk.GasInfo{}, nil, nil, err
	}
	if r.failBytes[string(txBytes)] {
		_ = r.pool.Remove(tx) // baseapp removes on ante failure during recheck
		return sdk.GasInfo{}, nil, nil, errors.New("ante failed on recheck")
	}
	if r.failNoRemoveBytes[string(txBytes)] {
		return sdk.GasInfo{}, nil, nil, errors.New("msg execution failed on recheck")
	}

	if trackNonce {
		if r.expectedNonce == nil {
			r.expectedNonce = map[string]uint64{}
		}
		span, ok := r.nonceSpan[string(txBytes)]
		if !ok {
			span = 1
		}
		r.expectedNonce[sender] = seq + span
	}
	return sdk.GasInfo{}, &sdk.Result{}, nil, nil
}

// recheckFixture builds a real PriorityNonceMempool + manager wired for recheck.
type recheckFixture struct {
	a      *Manager
	pool   *sdkmempool.PriorityNonceMempool[int64]
	enc    *EncoderCache
	signer fakeSigner
	runner *recheckRunner
}

func newRecheckFixture(failBytes ...string) *recheckFixture {
	signer := fakeSigner{m: map[sdk.Tx][]sdkmempool.SignerData{}}
	pool := sdkmempool.NewPriorityMempool(sdkmempool.PriorityNonceMempoolConfig[int64]{
		TxPriority:      sdkmempool.NewDefaultTxPriority(),
		SignerExtractor: signer,
	})
	enc := NewEncoderCache(0, 0)
	fail := make(map[string]bool, len(failBytes))
	for _, b := range failBytes {
		fail[b] = true
	}
	runner := &recheckRunner{pool: pool, failBytes: fail, seen: map[string]bool{}}
	// Per-tx encoder so the encCache-miss fallback yields deterministic bytes.
	txEncoder := func(tx sdk.Tx) ([]byte, error) { return []byte("enc-" + strconv.Itoa(tx.(*ptrTx).id)), nil }
	decoder := func([]byte) (sdk.Tx, error) { return nil, errors.New("unused") }
	a := newManager(runner, enc, txEncoder, decoder)
	a.sched.mpool = pool
	a.sched.signer = signer
	return &recheckFixture{a: a, pool: pool, enc: enc, signer: signer, runner: runner}
}

// add inserts a tx with the given sender/sequence and registers its recheck
// bytes in encCache (so RecheckTxs hits the cache, not the encoder).
func (f *recheckFixture) add(id int, sender string, seq uint64, bz string) *ptrTx {
	tx := f.insert(id, sdk.AccAddress(sender), seq)
	f.enc.Set(tx, []byte(bz))
	return tx
}

// addChain pools n txs from sender, the i-th at nonce seqOf(i) (i when nil),
// encoded as chainBytes(sender, i).
func (f *recheckFixture) addChain(sender string, n int, seqOf func(int) uint64) ([]sdk.Tx, []*ptrTx) {
	txs, ptrs := make([]sdk.Tx, n), make([]*ptrTx, n)
	for i := range n {
		seq := uint64(i)
		if seqOf != nil {
			seq = seqOf(i)
		}
		ptrs[i] = f.add(i+1, sender, seq, chainBytes(sender, i))
		txs[i] = ptrs[i]
	}
	return txs, ptrs
}

func chainBytes(sender string, i int) string { return sender + "-" + strconv.Itoa(i) }

// insert adds a tx with the given signers but no encCache entry, so RecheckTxs
// falls back to the encoder. The first signer keys the pool.
func (f *recheckFixture) insert(id int, first sdk.AccAddress, seq uint64, rest ...sdk.AccAddress) *ptrTx {
	tx := &ptrTx{id: id}
	sigs := []sdkmempool.SignerData{sdkmempool.NewSignerData(first, seq)}
	for _, r := range rest {
		sigs = append(sigs, sdkmempool.NewSignerData(r, seq))
	}
	f.signer.m[tx] = sigs
	if err := f.pool.Insert(sdk.Context{}, tx); err != nil {
		panic(err)
	}
	return tx
}

// addTimeout inserts a tx carrying a TimeoutHeight, keyed by sender, with its
// recheck bytes registered in encCache.
func (f *recheckFixture) addTimeout(id int, sender string, seq uint64, bz string, timeout uint64) *ptrTx {
	tx := &ptrTx{id: id, timeout: timeout}
	f.signer.m[tx] = []sdkmempool.SignerData{sdkmempool.NewSignerData(sdk.AccAddress(sender), seq)}
	if err := f.pool.Insert(sdk.Context{}, tx); err != nil {
		panic(err)
	}
	f.enc.Set(tx, []byte(bz))
	return tx
}

func poolHas(pool *sdkmempool.PriorityNonceMempool[int64], target sdk.Tx) bool {
	found := false
	sdkmempool.SelectBy(context.Background(), pool, nil, func(tx sdk.Tx) bool {
		if tx == target {
			found = true
			return false
		}
		return true
	})
	return found
}

func TestRecheckTxs_EvictsStaleKeepsValid(t *testing.T) {
	f := newRecheckFixture("alice-0") // alice's seq-0 tx now fails recheck
	stale := f.add(1, "alice", 0, "alice-0")
	survivor := f.add(2, "alice", 1, "alice-1")
	untouched := f.add(3, "bob", 0, "bob-0")

	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.sched.RecheckTxs()

	if poolHas(f.pool, stale) {
		t.Fatal("stale tx should have been removed from the pool")
	}
	if _, ok := f.enc.Get(stale); ok {
		t.Fatal("stale tx should have been evicted from encCache")
	}
	if !poolHas(f.pool, survivor) {
		t.Fatal("valid tx from a touched sender must stay")
	}
	if !poolHas(f.pool, untouched) {
		t.Fatal("tx from an untouched sender must stay")
	}
	if f.runner.seen["bob-0"] {
		t.Fatal("untouched sender's tx must not be rechecked")
	}
	if !f.runner.seen["alice-0"] || !f.runner.seen["alice-1"] {
		t.Fatal("both touched-sender txs must be rechecked")
	}
	for _, m := range f.runner.modes {
		if m != sdk.ExecModeReCheck {
			t.Fatalf("recheck must use ExecModeReCheck, got %v", m)
		}
	}
}

func TestRecheckTxs_MsgExecFailureEvictsFromPool(t *testing.T) {
	f := newRecheckFixture()
	f.runner.failNoRemoveBytes = map[string]bool{"alice-0": true}
	stale := f.add(1, "alice", 0, "alice-0")

	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.sched.RecheckTxs()

	if poolHas(f.pool, stale) {
		t.Fatal("tx failing recheck at msg execution (not ante) must still be removed from the pool")
	}
	if _, ok := f.enc.Get(stale); ok {
		t.Fatal("tx failing recheck must be evicted from encCache")
	}
}

func TestRecheckTxs_EmptyPendingNoOp(t *testing.T) {
	f := newRecheckFixture()
	f.add(1, "alice", 0, "alice-0")

	f.a.sched.RecheckTxs() // recheckSenders nil

	if len(f.runner.modes) != 0 {
		t.Fatalf("no RunTx expected with empty recheckSenders, got %d calls", len(f.runner.modes))
	}
}

func TestRecheckTxs_DrainsPending(t *testing.T) {
	f := newRecheckFixture()
	f.add(1, "alice", 0, "alice-0")
	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}

	f.a.sched.RecheckTxs()
	first := len(f.runner.modes)
	f.a.sched.RecheckTxs() // recheckSenders consumed; second run is a no-op

	if len(f.runner.modes) != first {
		t.Fatal("recheckSenders must be drained after one RecheckTxs")
	}
}

// Timeout sweep evicts an expired tx even when its sender wasn't touched by the
// last block (no recheckSenders entry, so the ante-recheck path never sees it).
func TestRecheckTxs_EvictsExpiredUntouchedSender(t *testing.T) {
	f := newRecheckFixture()
	expired := f.addTimeout(1, "carol", 0, "carol-0", 5)

	f.a.sched.lastCommittedHeight = 5 // next block = 6 > timeoutHeight 5 → never valid again
	f.a.sched.RecheckTxs()            // recheckSenders nil: only the timeout sweep runs

	if poolHas(f.pool, expired) {
		t.Fatal("expired tx must be evicted regardless of touched senders")
	}
	if _, ok := f.enc.Get(expired); ok {
		t.Fatal("expired tx must be evicted from encCache")
	}
	if len(f.runner.modes) != 0 {
		t.Fatal("expired txs must be removed without a RunTx recheck")
	}
}

// committedHeight == timeoutHeight evicts (next block exceeds it); one above
// survives (still valid in the next block); 0 never expires.
func TestRecheckTxs_TimeoutBoundary(t *testing.T) {
	f := newRecheckFixture()
	atLimit := f.addTimeout(1, "carol", 0, "carol-0", 5)
	survivor := f.addTimeout(2, "dave", 0, "dave-0", 6)
	noTimeout := f.addTimeout(3, "erin", 0, "erin-0", 0)

	f.a.sched.lastCommittedHeight = 5
	f.a.sched.RecheckTxs()

	if poolHas(f.pool, atLimit) {
		t.Fatal("tx with timeoutHeight == committedHeight must be evicted")
	}
	if !poolHas(f.pool, survivor) {
		t.Fatal("tx with timeoutHeight > committedHeight must survive")
	}
	if !poolHas(f.pool, noTimeout) {
		t.Fatal("tx with timeoutHeight 0 must never be evicted")
	}
}

// A single scan both evicts expired txs and rechecks touched-sender candidates.
func TestRecheckTxs_SweepAndRecheckTogether(t *testing.T) {
	f := newRecheckFixture("alice-0") // alice's seq-0 fails recheck
	stale := f.add(1, "alice", 0, "alice-0")
	expired := f.addTimeout(2, "carol", 0, "carol-0", 5)
	survivor := f.add(3, "alice", 1, "alice-1")

	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.sched.lastCommittedHeight = 5
	f.a.sched.RecheckTxs()

	if poolHas(f.pool, expired) {
		t.Fatal("expired tx must be swept")
	}
	if poolHas(f.pool, stale) {
		t.Fatal("stale touched-sender tx must be rechecked out")
	}
	if !poolHas(f.pool, survivor) {
		t.Fatal("valid touched-sender tx must stay")
	}
	if f.runner.seen["carol-0"] {
		t.Fatal("expired tx must be evicted without a RunTx recheck")
	}
}

// StageRecheckSenders must stage the committed height (not just senders) so the
// timeout sweep fires on the next RecheckTxs.
func TestStageRecheckSenders_StagesHeightForSweep(t *testing.T) {
	f := newRecheckFixture()
	expired := f.addTimeout(1, "carol", 0, "carol-0", 5)

	f.a.StageRecheckSenders(5, nil) // decoder nil: stages height, leaves recheckSenders nil
	f.a.sched.RecheckTxs()

	if poolHas(f.pool, expired) {
		t.Fatal("StageRecheckSenders must stage height so the sweep evicts the expired tx")
	}
	if len(f.runner.modes) != 0 {
		t.Fatal("sweep-only path must not RunTx")
	}
}

func TestRecheckTxs_InvalidatesPendingCacheAfterEviction(t *testing.T) {
	f := newRecheckFixture()
	f.addTimeout(1, "carol", 0, "carol-0", 5)

	f.a.StageRecheckSenders(5, nil)
	before := f.a.exec.pending.epoch.Load()
	f.a.RecheckTxs()

	if got := f.a.exec.pending.epoch.Load(); got == before {
		t.Fatal("RecheckTxs must invalidate the pending cache after evicting txs, not just rely on StageRecheckSenders's earlier bump")
	}
}

func TestRunRecheck_InvalidatesPendingCacheOnEviction(t *testing.T) {
	f := newRecheckFixture("alice-0") // alice's seq-0 tx now fails recheck
	f.add(1, "alice", 0, "alice-0")

	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	before := f.a.exec.pending.epoch.Load()
	f.a.RecheckTxs()

	if got := f.a.exec.pending.epoch.Load(); got == before {
		t.Fatal("RecheckTxs must invalidate the pending cache after runRecheck evicts a tx via RunTx failure")
	}
}

// Two committed blocks staged without an intervening RecheckTxs drain (e.g. a
// Commit error skipped the recheck) must union their senders, not drop the first.
func TestStageRecheckSenders_MergesAcrossBlocks(t *testing.T) {
	signer := fakeSigner{m: map[sdk.Tx][]sdkmempool.SignerData{}}
	txA, txB := &ptrTx{id: 1}, &ptrTx{id: 2}
	signer.m[txA] = []sdkmempool.SignerData{sdkmempool.NewSignerData(sdk.AccAddress("alice"), 0)}
	signer.m[txB] = []sdkmempool.SignerData{sdkmempool.NewSignerData(sdk.AccAddress("bob"), 0)}
	decoder := func(b []byte) (sdk.Tx, error) {
		switch string(b) {
		case "a":
			return txA, nil
		case "b":
			return txB, nil
		}
		return nil, errors.New("unknown")
	}
	a := newManager(&stubRunner{}, nil, noopEncoder, decoder)
	a.sched.signer = signer

	a.StageRecheckSenders(10, [][]byte{[]byte("a")})
	a.StageRecheckSenders(11, [][]byte{[]byte("b")}) // no drain between: must keep alice

	if _, ok := a.sched.recheckSenders[sdk.AccAddress("alice").String()]; !ok {
		t.Fatal("block-10 sender lost after staging block 11 without a recheck drain")
	}
	if _, ok := a.sched.recheckSenders[sdk.AccAddress("bob").String()]; !ok {
		t.Fatal("block-11 sender missing")
	}
	if a.sched.lastCommittedHeight != 11 {
		t.Fatalf("height must advance to 11, got %d", a.sched.lastCommittedHeight)
	}
}

func TestStageRecheckSenders_NoDepsNoPanic(t *testing.T) {
	a := newManager(&stubRunner{}, nil, noopEncoder, nil)
	a.StageRecheckSenders(0, [][]byte{[]byte("x")}) // decoder/signer nil → no-op
	a.sched.RecheckTxs()                            // mpool nil → no-op
}

func TestStageRecheckSenders_RecheckDisabledSkipsSendersButStagesHeight(t *testing.T) {
	tx := &ptrTx{id: 1}
	signer := fakeSigner{m: map[sdk.Tx][]sdkmempool.SignerData{
		tx: {sdkmempool.NewSignerData(sdk.AccAddress("alice"), 0)},
	}}
	decoder := func(b []byte) (sdk.Tx, error) { return tx, nil }
	a := newManager(&stubRunner{}, nil, noopEncoder, decoder)
	a.sched.signer = signer
	a.sched.recheckDisabled = true

	a.StageRecheckSenders(7, [][]byte{[]byte("x")})

	if a.sched.lastCommittedHeight != 7 {
		t.Fatalf("height must stage even when recheck disabled, got %d", a.sched.lastCommittedHeight)
	}
	if a.sched.recheckSenders != nil {
		t.Fatal("recheckDisabled must skip decode+merge into recheckSenders")
	}
}

// A tx with no encCache entry must still be rechecked via the txEncoder fallback.
func TestRecheckTxs_EncoderFallbackOnCacheMiss(t *testing.T) {
	f := newRecheckFixture("enc-1") // encoder yields "enc-<id>"; fail id 1
	stale := f.insert(1, sdk.AccAddress("alice"), 0)
	if _, ok := f.enc.Get(stale); ok {
		t.Fatal("precondition: tx must not be in encCache")
	}
	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}

	f.a.sched.RecheckTxs()

	if !f.runner.seen["enc-1"] {
		t.Fatal("cache-miss tx must be rechecked using encoder-produced bytes")
	}
	if poolHas(f.pool, stale) {
		t.Fatal("stale cache-miss tx must be removed")
	}
}

// A multi-signer tx must be rechecked when ANY of its signers is in recheckSenders,
// even though the pool keys it by the first signer only.
func TestRecheckTxs_MultiSignerMatchesAnySigner(t *testing.T) {
	f := newRecheckFixture("enc-1")
	// pool key = alice (first signer); recheckSenders names only the second signer, bob.
	stale := f.insert(1, sdk.AccAddress("alice"), 0, sdk.AccAddress("bob"))
	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("bob").String(): {}}

	f.a.sched.RecheckTxs()

	if !f.runner.seen["enc-1"] {
		t.Fatal("tx must be rechecked when a non-primary signer is touched")
	}
	if poolHas(f.pool, stale) {
		t.Fatal("stale multi-signer tx must be removed")
	}
}

// lockTrackingMempool flags inSelect while its SelectBy callback runs, so a test
// can detect whether RecheckTxs extracts signers under the pool lock.
type lockTrackingMempool struct {
	txs      []sdk.Tx
	inSelect bool
}

func (m *lockTrackingMempool) Insert(_ context.Context, tx sdk.Tx) error {
	m.txs = append(m.txs, tx)
	return nil
}
func (m *lockTrackingMempool) Select(context.Context, [][]byte) sdkmempool.Iterator { return nil }
func (m *lockTrackingMempool) CountTx() int                                         { return len(m.txs) }
func (m *lockTrackingMempool) Remove(tx sdk.Tx) error {
	for i, t := range m.txs {
		if t == tx {
			m.txs = append(m.txs[:i], m.txs[i+1:]...)
			break
		}
	}
	return nil
}

func (m *lockTrackingMempool) SelectBy(_ context.Context, _ [][]byte, cb func(sdk.Tx) bool) {
	m.inSelect = true
	defer func() { m.inSelect = false }()
	for _, tx := range m.txs {
		if !cb(tx) {
			return
		}
	}
}

// RemoveWithReason makes the fake satisfy ExtMempool so sdkmempool.SelectBy
// dispatches to the method above instead of falling back to Select.
func (m *lockTrackingMempool) RemoveWithReason(_ context.Context, tx sdk.Tx, _ sdkmempool.RemoveReason) error {
	return m.Remove(tx)
}

// lockObservingSigner records whether GetSigners was ever called while the pool
// was mid-SelectBy (i.e. under mp.mtx).
type lockObservingSigner struct {
	m         map[sdk.Tx][]sdkmempool.SignerData
	pool      *lockTrackingMempool
	sawLocked bool
}

func (s *lockObservingSigner) GetSigners(tx sdk.Tx) ([]sdkmempool.SignerData, error) {
	if s.pool.inSelect {
		s.sawLocked = true
	}
	sd, ok := s.m[tx]
	if !ok {
		return nil, errors.New("no signer for tx")
	}
	return sd, nil
}

// RecheckTxs must not run more than maxRecheckBatch RunTx calls in one cycle
// when the cap boundary falls between (single-tx) signer groups.
func TestRecheckTxs_BatchCapLimitsCandidates(t *testing.T) {
	const total = 5
	const batch = 2
	f := newRecheckFixture()
	recheckSenders := make(map[string]struct{}, total)
	for i := 0; i < total; i++ {
		sender := "sender" + strconv.Itoa(i)
		f.add(i+1, sender, 0, sender+"-0")
		recheckSenders[sdk.AccAddress(sender).String()] = struct{}{}
	}
	f.a.sched.maxRecheckBatch = batch
	f.a.sched.recheckSenders = recheckSenders

	f.a.sched.RecheckTxs()

	if got := len(f.runner.modes); got != batch {
		t.Fatalf("expected %d RunTx calls with batch cap, got %d", batch, got)
	}
}

func TestRecheckTxs_BatchCapCarriesOverflowWithoutSplittingGroup(t *testing.T) {
	const batch = 3
	f := newRecheckFixture()
	f.runner.signer = f.signer                                                      // enables per-sender expected-nonce semantics in the fake RunTx
	f.runner.expectedNonce = map[string]uint64{sdk.AccAddress("alice").String(): 8} // account nonce 8

	bob := f.add(1, "bob", 0, "bob-0")
	aliceSeqs := []uint64{8, 9, 10, 11, 12}
	alice := make([]*ptrTx, len(aliceSeqs))
	for i, seq := range aliceSeqs {
		alice[i] = f.add(10+i, "alice", seq, "alice-"+strconv.FormatUint(seq, 10))
	}

	f.a.sched.maxRecheckBatch = batch
	f.a.sched.recheckSenders = map[string]struct{}{
		sdk.AccAddress("bob").String():   {},
		sdk.AccAddress("alice").String(): {},
	}

	// Cycle 1: bob's group (1 tx) fits under the cap; alice's group (5 txs)
	// would push the running total to 6 > 3, so the whole group must defer.
	f.a.sched.RecheckTxs()

	if !f.runner.seen["bob-0"] {
		t.Fatal("bob's group must run in cycle 1")
	}
	for _, seq := range aliceSeqs {
		if f.runner.seen["alice-"+strconv.FormatUint(seq, 10)] {
			t.Fatalf("alice's group must not be partially run before deferring, but seq %d ran", seq)
		}
	}
	if len(f.a.sched.deferred) != len(aliceSeqs) {
		t.Fatalf("expected alice's whole group (%d txs) deferred, got %d", len(aliceSeqs), len(f.a.sched.deferred))
	}

	// Cycle 2: recheckSenders is empty, but the deferred carry must still run
	// as one atomic group against alice's real nonce (still 8: her chain was
	// only rechecked, never included in a block).
	f.a.sched.RecheckTxs()

	for _, seq := range aliceSeqs {
		if !f.runner.seen["alice-"+strconv.FormatUint(seq, 10)] {
			t.Fatalf("alice-%d must be rechecked in cycle 2", seq)
		}
	}
	if !poolHas(f.pool, bob) {
		t.Fatal("bob's tx must remain valid in the pool")
	}
	for i, tx := range alice {
		if !poolHas(f.pool, tx) {
			t.Fatalf("alice's tx at index %d (seq %d) must not be evicted: the cap must not split her nonce chain", i, aliceSeqs[i])
		}
	}
}

// maxRecheckBatch == 0 must leave the limit disabled (all candidates rechecked).
func TestRecheckTxs_BatchCapZeroIsUnlimited(t *testing.T) {
	const total = 5
	f := newRecheckFixture()
	for i := 0; i < total; i++ {
		f.add(i+1, "alice", uint64(i), "alice-"+strconv.Itoa(i))
	}
	// maxRecheckBatch left at zero default
	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}

	f.a.sched.RecheckTxs()

	if got := len(f.runner.modes); got != total {
		t.Fatalf("expected %d RunTx calls with no cap, got %d", total, got)
	}
}

// Known blind spot: a sender whose txs sit in the pool for many blocks without
// being committed is never rechecked while other senders are touched, so its
// txs are only revalidated when its own sender lands in a committed block (or a
// timeout sweep fires). This documents that intended behavior — the recheck is
// committed-sender-scoped, not a full-pool sweep.
func TestRecheckTxs_UntouchedSenderNeverRechecked(t *testing.T) {
	f := newRecheckFixture()
	idle := f.add(1, "carol", 0, "carol-0") // carol never lands in a committed block

	// Three blocks each touch alice only; carol is never in recheckSenders.
	for i := 0; i < 3; i++ {
		f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
		f.a.sched.RecheckTxs()
	}

	if !poolHas(f.pool, idle) {
		t.Fatal("untouched sender's tx must remain in the pool")
	}
	if f.runner.seen["carol-0"] {
		t.Fatal("untouched sender's tx must never be rechecked")
	}
}

func TestRecheckTxs_NonceGapAfterTimeoutEvictionRechecked(t *testing.T) {
	// When a lower-nonce tx is timeout-evicted, the higher-nonce sibling must be
	// rechecked even though the sender never committed a block tx. Without the fix
	// the sibling would stay in the pool, enter proposals, and fail FinalizeBlock.
	f := newRecheckFixture("carol-1")                    // carol-1 fails recheck (nonce gap)
	expired := f.addTimeout(1, "carol", 0, "carol-0", 5) // nonce 0, times out at height 5
	gapped := f.addTimeout(2, "carol", 1, "carol-1", 0)  // nonce 1, no timeout

	f.a.sched.lastCommittedHeight = 5 // sweep evicts nonce 0; carol not in recheckSenders
	f.a.sched.RecheckTxs()

	if poolHas(f.pool, expired) {
		t.Fatal("expired tx must be swept")
	}
	if !f.runner.seen["carol-1"] {
		t.Fatal("gapped sibling must be rechecked after its predecessor was evicted")
	}
	if poolHas(f.pool, gapped) {
		t.Fatal("gapped sibling must be evicted after failing recheck")
	}
}

func TestRecheckTxs_NonceGapAfterTTLEvictionRechecked(t *testing.T) {
	// Same class of bug as the TimeoutHeight variant: TTL-evicted lower-nonce tx
	// must trigger recheck of the surviving higher-nonce sibling.
	f := newRecheckFixture("carol-1") // carol-1 fails recheck (nonce gap)
	f.a.sched.ttlNumBlocks = 5
	aged := f.add(1, "carol", 0, "carol-0")
	gapped := f.add(2, "carol", 1, "carol-1")

	// Seed arrival directly: aged has been in pool 5+ blocks; gapped just arrived.
	f.a.sched.arrival = map[sdk.Tx]int64{aged: 5, gapped: 10}

	f.a.sched.lastCommittedHeight = 10 // aged: 10-5=5 >= ttl → evicted; gapped: 10-10=0 → survives
	f.a.sched.RecheckTxs()

	if poolHas(f.pool, aged) {
		t.Fatal("TTL-expired tx must be swept")
	}
	if !f.runner.seen["carol-1"] {
		t.Fatal("gapped sibling must be rechecked after its predecessor was TTL-evicted")
	}
	if poolHas(f.pool, gapped) {
		t.Fatal("gapped sibling must be evicted after failing recheck")
	}
}

// Doing it inside the callback would pin mp.mtx (and run RunTx's Remove under
// it) across the whole scan, blocking admission/reap on the commit path.
func TestRecheckTxs_SignerExtractionOutsidePoolLock(t *testing.T) {
	pool := &lockTrackingMempool{}
	signer := &lockObservingSigner{m: map[sdk.Tx][]sdkmempool.SignerData{}, pool: pool}
	enc := NewEncoderCache(0, 0)
	runner := &recheckRunner{pool: pool, failBytes: map[string]bool{}, seen: map[string]bool{}}
	txEncoder := func(tx sdk.Tx) ([]byte, error) { return []byte("enc-" + strconv.Itoa(tx.(*ptrTx).id)), nil }
	a := newManager(runner, enc, txEncoder, func([]byte) (sdk.Tx, error) { return nil, errors.New("unused") })
	a.sched.mpool = pool
	a.sched.signer = signer

	tx := &ptrTx{id: 1}
	signer.m[tx] = []sdkmempool.SignerData{sdkmempool.NewSignerData(sdk.AccAddress("alice"), 0)}
	_ = pool.Insert(context.Background(), tx)
	a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}

	a.sched.RecheckTxs()

	if signer.sawLocked {
		t.Fatal("signer extraction ran inside SelectBy (under the pool lock)")
	}
	if !runner.seen["enc-1"] {
		t.Fatal("candidate from a touched sender must still be rechecked")
	}
}

// TTL evicts a tx older than ttlNumBlocks by arrival height — regardless of
// TimeoutHeight (EVM txs carry th=0 = never expire) and without a RunTx recheck.
func TestRecheckTxs_TTLEvictsAgedTx(t *testing.T) {
	f := newRecheckFixture()
	f.a.sched.ttlNumBlocks = 5
	aged := f.add(1, "alice", 0, "alice-0") // th=0: the timeout sweep never touches it

	f.a.sched.lastCommittedHeight = 10 // first sighting records arrival=10
	f.a.sched.RecheckTxs()
	if !poolHas(f.pool, aged) {
		t.Fatal("tx must survive its first sighting")
	}

	f.a.sched.lastCommittedHeight = 15 // 15-10 == 5 == ttl → evicted
	f.a.sched.RecheckTxs()
	if poolHas(f.pool, aged) {
		t.Fatal("tx older than ttlNumBlocks must be evicted")
	}
	if _, ok := f.enc.Get(aged); ok {
		t.Fatal("aged tx must be evicted from encCache")
	}
	if len(f.runner.modes) != 0 {
		t.Fatal("TTL eviction must not run a RunTx recheck")
	}
}

// A tx younger than ttlNumBlocks survives the sweep.
func TestRecheckTxs_TTLKeepsYoungTx(t *testing.T) {
	f := newRecheckFixture()
	f.a.sched.ttlNumBlocks = 5
	young := f.add(1, "alice", 0, "alice-0")

	f.a.sched.lastCommittedHeight = 10 // arrival=10
	f.a.sched.RecheckTxs()
	f.a.sched.lastCommittedHeight = 14 // 14-10 == 4 < ttl
	f.a.sched.RecheckTxs()

	if !poolHas(f.pool, young) {
		t.Fatal("tx younger than ttlNumBlocks must stay")
	}
}

// ttlNumBlocks == 0 disables TTL: no eviction by age, no arrival map allocated.
func TestRecheckTxs_TTLDisabledKeepsOldTx(t *testing.T) {
	f := newRecheckFixture()
	// ttlNumBlocks left 0
	old := f.add(1, "alice", 0, "alice-0")

	for h := int64(1); h <= 200; h++ {
		f.a.sched.lastCommittedHeight = h
		f.a.sched.RecheckTxs()
	}

	if !poolHas(f.pool, old) {
		t.Fatal("TTL disabled: tx must never be evicted by age")
	}
	if f.a.sched.arrival != nil {
		t.Fatal("disabled TTL must not allocate the arrival map")
	}
}

func TestRecheckTxs_RecheckDisabledSkipsTTLEviction(t *testing.T) {
	f := newRecheckFixture()
	f.a.sched.recheckDisabled = true
	f.a.sched.ttlNumBlocks = 5
	aged := f.add(1, "alice", 0, "alice-0")

	f.a.sched.lastCommittedHeight = 10 // first sighting would record arrival=10 if TTL ran
	f.a.sched.RecheckTxs()
	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.sched.lastCommittedHeight = 15 // 15-10 == ttl, but recheckDisabled skips the sweep entirely
	f.a.sched.RecheckTxs()

	if !poolHas(f.pool, aged) {
		t.Fatal("recheckDisabled must skip TTL eviction too, not just RunTx recheck")
	}
	if f.a.sched.arrival != nil {
		t.Fatal("recheckDisabled must not build the arrival map")
	}
	if len(f.runner.modes) != 0 {
		t.Fatal("recheckDisabled must never call RunTx, even on a live staged candidate")
	}
}

func TestRecheckTxs_RecheckDisabledSkipsCandidateRunTx(t *testing.T) {
	f := newRecheckFixture()
	f.a.sched.recheckDisabled = true
	tx := f.add(1, "alice", 0, "alice-0")

	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.sched.lastCommittedHeight = 1
	f.a.sched.RecheckTxs()

	if !poolHas(f.pool, tx) {
		t.Fatal("recheckDisabled must not run RunTx reval even with a staged sender")
	}
	if len(f.runner.modes) != 0 {
		t.Fatal("recheckDisabled must never call RunTx")
	}
}

// Arrival entries for txs gone from the pool (e.g. included in a block) drop out
// each cycle, bounding the map to the live pool.
func TestRecheckTxs_TTLArrivalReconcilesRemovedTxs(t *testing.T) {
	f := newRecheckFixture()
	f.a.sched.ttlNumBlocks = 100
	tx := f.add(1, "alice", 0, "alice-0")

	f.a.sched.lastCommittedHeight = 1
	f.a.sched.RecheckTxs()
	if len(f.a.sched.arrival) != 1 {
		t.Fatalf("arrival must track the live tx, got %d", len(f.a.sched.arrival))
	}

	_ = f.pool.Remove(tx) // simulate block inclusion
	f.a.sched.lastCommittedHeight = 2
	f.a.sched.RecheckTxs()
	if len(f.a.sched.arrival) != 0 {
		t.Fatalf("arrival must drop the removed tx, got %d", len(f.a.sched.arrival))
	}
}

// TTL eviction sits in the scan loop ahead of the batch cap, so it fires for
// every aged tx regardless of maxRecheckBatch and never spends a RunTx recheck.
func TestRecheckTxs_TTLEvictsRegardlessOfBatchCap(t *testing.T) {
	const total = 5
	f := newRecheckFixture()
	f.a.sched.ttlNumBlocks = 2
	f.a.sched.maxRecheckBatch = 1 // far below total; one sender per tx so the cap can bite
	txs := make([]*ptrTx, total)
	recheckSenders := make(map[string]struct{}, total)
	for i := 0; i < total; i++ {
		sender := "sender" + strconv.Itoa(i)
		txs[i] = f.add(i+1, sender, 0, sender+"-0")
		recheckSenders[sdk.AccAddress(sender).String()] = struct{}{}
	}
	f.a.sched.recheckSenders = recheckSenders

	f.a.sched.lastCommittedHeight = 100 // first sighting: arrival=100
	f.a.sched.RecheckTxs()
	if got := len(f.runner.modes); got != 1 {
		t.Fatalf("cycle1: batch cap must bound recheck to 1, got %d", got)
	}

	f.a.sched.recheckSenders = recheckSenders
	f.a.sched.lastCommittedHeight = 102 // 102-100 == 2 == ttl → all aged out
	before := len(f.runner.modes)
	f.a.sched.RecheckTxs()

	for _, tx := range txs {
		if poolHas(f.pool, tx) {
			t.Fatalf("aged tx %d must be evicted by TTL regardless of batch cap", tx.id)
		}
	}
	if got := len(f.runner.modes) - before; got != 0 {
		t.Fatalf("TTL-evicted txs must not be rechecked; got %d new RunTx", got)
	}
	if f.a.sched.deferred != nil {
		t.Fatalf("nothing should carry over once all aged out, got %d", len(f.a.sched.deferred))
	}
}

// A tx carried in the deferred queue that ages past the TTL is evicted by the
// scan sweep and dropped from the carry, not rechecked.
func TestRecheckTxs_TTLEvictsDeferredCarryover(t *testing.T) {
	const total = 4
	f := newRecheckFixture()
	f.a.sched.ttlNumBlocks = 3
	f.a.sched.maxRecheckBatch = 1 // force overflow into deferred; one sender per tx so the cap can bite
	txs := make([]*ptrTx, total)
	recheckSenders := make(map[string]struct{}, total)
	for i := 0; i < total; i++ {
		sender := "sender" + strconv.Itoa(i)
		txs[i] = f.add(i+1, sender, 0, sender+"-0")
		recheckSenders[sdk.AccAddress(sender).String()] = struct{}{}
	}
	f.a.sched.recheckSenders = recheckSenders

	f.a.sched.lastCommittedHeight = 50 // arrival=50 for all
	f.a.sched.RecheckTxs()
	if len(f.a.sched.deferred) == 0 {
		t.Fatal("precondition: batch cap must have carried overflow")
	}

	// Jump past TTL with empty recheckSenders: only the scan sweep runs. The deferred
	// carryover must be evicted, not survive as stale candidates.
	f.a.sched.lastCommittedHeight = 53 // 53-50 == 3 == ttl
	f.a.sched.RecheckTxs()

	for _, tx := range txs {
		if poolHas(f.pool, tx) {
			t.Fatalf("deferred tx %d must be TTL-evicted", tx.id)
		}
	}
	if f.a.sched.deferred != nil {
		t.Fatalf("deferred queue must be empty after aged txs evicted, got %d", len(f.a.sched.deferred))
	}
}

func TestStageSkippedSenders_MergesIntoRecheckSenders(t *testing.T) {
	tx := &ptrTx{id: 1}
	signer := fakeSigner{m: map[sdk.Tx][]sdkmempool.SignerData{
		tx: {sdkmempool.NewSignerData(sdk.AccAddress("alice"), 0)},
	}}
	decoder := func(b []byte) (sdk.Tx, error) {
		if string(b) == "a" {
			return tx, nil
		}
		return nil, errors.New("unknown")
	}
	a := newManager(&stubRunner{}, nil, noopEncoder, decoder)
	a.sched.signer = signer

	a.StageSkippedSenders([][]byte{[]byte("a")})

	if _, ok := a.sched.recheckSenders[sdk.AccAddress("alice").String()]; !ok {
		t.Fatal("gate-skipped sender must appear in recheckSenders")
	}
}

func TestStageSkippedSenders_DoesNotTouchLastCommittedHeight(t *testing.T) {
	tx := &ptrTx{id: 1}
	signer := fakeSigner{m: map[sdk.Tx][]sdkmempool.SignerData{
		tx: {sdkmempool.NewSignerData(sdk.AccAddress("alice"), 0)},
	}}
	decoder := func(b []byte) (sdk.Tx, error) {
		if string(b) == "a" {
			return tx, nil
		}
		return nil, errors.New("unknown")
	}
	a := newManager(&stubRunner{}, nil, noopEncoder, decoder)
	a.sched.signer = signer
	a.sched.lastCommittedHeight = 42

	a.StageSkippedSenders([][]byte{[]byte("a")})

	if a.sched.lastCommittedHeight != 42 {
		t.Fatalf("StageSkippedSenders must not touch lastCommittedHeight: got %d, want 42", a.sched.lastCommittedHeight)
	}
}

// StageSkippedSenders (PrepareProposal) and StageRecheckSenders (FinalizeBlock) both
// write to recheckSenders; the second call must merge, not overwrite.
func TestStageSkippedSenders_MergesWithCommittedSenders(t *testing.T) {
	txA, txB := &ptrTx{id: 1}, &ptrTx{id: 2}
	signer := fakeSigner{m: map[sdk.Tx][]sdkmempool.SignerData{
		txA: {sdkmempool.NewSignerData(sdk.AccAddress("alice"), 0)},
		txB: {sdkmempool.NewSignerData(sdk.AccAddress("bob"), 0)},
	}}
	decoder := func(b []byte) (sdk.Tx, error) {
		switch string(b) {
		case "a":
			return txA, nil
		case "b":
			return txB, nil
		}
		return nil, errors.New("unknown")
	}
	a := newManager(&stubRunner{}, nil, noopEncoder, decoder)
	a.sched.signer = signer

	a.StageRecheckSenders(10, [][]byte{[]byte("a")}) // alice from committed block
	a.StageSkippedSenders([][]byte{[]byte("b")})     // bob from gate skip

	if _, ok := a.sched.recheckSenders[sdk.AccAddress("alice").String()]; !ok {
		t.Fatal("committed sender must be preserved after StageSkippedSenders")
	}
	if _, ok := a.sched.recheckSenders[sdk.AccAddress("bob").String()]; !ok {
		t.Fatal("gate-skipped sender must be merged in")
	}
	if a.sched.lastCommittedHeight != 10 {
		t.Fatalf("height must stay at 10, got %d", a.sched.lastCommittedHeight)
	}
}

func TestStageSkippedSenders_NilDecoderNoop(t *testing.T) {
	a := newManager(&stubRunner{}, nil, noopEncoder, nil)
	a.StageSkippedSenders([][]byte{[]byte("x")}) // decoder nil → must not panic
	if a.sched.recheckSenders != nil {
		t.Fatal("nil decoder must leave recheckSenders unchanged")
	}
}

func TestStageSkippedSenders_EmptyIsNoop(t *testing.T) {
	a := newManager(&stubRunner{}, nil, noopEncoder, func([]byte) (sdk.Tx, error) { return &ptrTx{}, nil })
	a.StageSkippedSenders(nil)
	a.StageSkippedSenders([][]byte{})
	if a.sched.recheckSenders != nil {
		t.Fatal("empty input must not allocate recheckSenders")
	}
}

func TestStageSkippedSenders_RecheckDisabledSkipsMerge(t *testing.T) {
	tx := &ptrTx{id: 1}
	signer := fakeSigner{m: map[sdk.Tx][]sdkmempool.SignerData{
		tx: {sdkmempool.NewSignerData(sdk.AccAddress("alice"), 0)},
	}}
	decoder := func(b []byte) (sdk.Tx, error) { return tx, nil }
	a := newManager(&stubRunner{}, nil, noopEncoder, decoder)
	a.sched.signer = signer
	a.sched.recheckDisabled = true

	a.StageSkippedSenders([][]byte{[]byte("x")})

	if a.sched.recheckSenders != nil {
		t.Fatal("recheckDisabled must skip decode+merge into recheckSenders")
	}
}

// Gate-skipped senders staged via StageSkippedSenders are rechecked by the next
// RecheckTxs cycle — reducing residency from TTL (~60 s) to ~1 block.
func TestStageSkippedSenders_TriggerRecheckNextCycle(t *testing.T) {
	f := newRecheckFixture("alice-0") // alice's recheck bytes fail ante
	stale := f.add(1, "alice", 0, "alice-0")

	// Replace the stub decoder with one that maps the gate-skipped raw bytes to
	// the stale tx. The fakeSigner already has stale → alice, so
	// StageSkippedSenders extracts alice and adds her to recheckSenders.
	gateSkippedBz := []byte("gate-skipped-alice")
	f.a.exec.decoder = func(b []byte) (sdk.Tx, error) {
		if string(b) == string(gateSkippedBz) {
			return stale, nil
		}
		return nil, errors.New("unknown")
	}

	f.a.StageSkippedSenders([][]byte{gateSkippedBz})
	f.a.sched.RecheckTxs()

	if poolHas(f.pool, stale) {
		t.Fatal("gate-skipped and recheck-failed tx must be evicted in one cycle")
	}
	if _, ok := f.enc.Get(stale); ok {
		t.Fatal("evicted tx must be removed from encCache")
	}
}

// encCache (app.go). A TTL/timeout eviction must not panic on encCache.Evict.
func TestRecheckTxs_NilEncCacheEvictionNoPanic(t *testing.T) {
	signer := fakeSigner{m: map[sdk.Tx][]sdkmempool.SignerData{}}
	pool := sdkmempool.NewPriorityMempool(sdkmempool.PriorityNonceMempoolConfig[int64]{
		TxPriority:      sdkmempool.NewDefaultTxPriority(),
		SignerExtractor: signer,
	})
	a := newManager(&stubRunner{}, nil, noopEncoder, nil) // encCache nil
	a.sched.mpool = pool
	a.sched.signer = signer
	a.sched.ttlNumBlocks = 2

	tx := &ptrTx{id: 1}
	signer.m[tx] = []sdkmempool.SignerData{sdkmempool.NewSignerData(sdk.AccAddress("alice"), 0)}
	if err := pool.Insert(sdk.Context{}, tx); err != nil {
		t.Fatal(err)
	}

	a.sched.lastCommittedHeight = 10
	a.sched.RecheckTxs() // arrival=10
	a.sched.lastCommittedHeight = 12
	a.sched.RecheckTxs() // 12-10 == 2 → evict via nil encCache; must not panic

	if poolHas(pool, tx) {
		t.Fatal("aged tx must be evicted even with nil encCache")
	}
}

const aliceSeq0Bytes = "alice-0"

const (
	carlSeq5Bytes = "carl-5"
	carlSeq6Bytes = "carl-6"
	carlSeq7Bytes = "carl-7"
	carlSeq8Bytes = "carl-8"
)

func TestRunRecheck_GroupsCandidatesBySigner(t *testing.T) {
	f := newRecheckFixture()
	aliceLow := f.add(1, "alice", 0, aliceSeq0Bytes)
	bob := f.add(2, "bob", 0, "bob-0")
	aliceHigh := f.add(3, "alice", 1, "alice-1")

	f.a.sched.runRecheck(f.a.sched.groupCandidates([]sdk.Tx{aliceLow, bob, aliceHigh}), 0)

	want := []string{aliceSeq0Bytes, "alice-1", "bob-0"}
	if !slices.Equal(f.runner.calls, want) {
		t.Fatalf("candidates must run grouped by signer in first-appearance order: got %v, want %v", f.runner.calls, want)
	}
}

func TestRunRecheck_CascadeRule(t *testing.T) {
	wrongSeq := errorsmod.Wrap(sdkerrors.ErrWrongSequence, "nonce")
	testCases := []struct {
		name string
		// seqs is carl's candidate order, which needn't be ascending.
		seqs    []uint64
		failSeq uint64
		failErr error
		// wantRun / wantSkipped are seqs that must / must not spend a RunTx.
		wantRun, wantSkipped []uint64
		wantPooled           []uint64
	}{
		{
			name: "nonce gap cascades to higher siblings",
			seqs: []uint64{5, 7, 8}, failSeq: 7, failErr: wrongSeq,
			wantRun: []uint64{5, 7}, wantSkipped: []uint64{8}, wantPooled: []uint64{5},
		},
		{
			// With no accepted nonce before it, the failure may be stale, and its successor valid.
			name: "stale nonce does not cascade",
			seqs: []uint64{5, 6}, failSeq: 5, failErr: errorsmod.Wrap(sdkerrors.ErrInvalidSequence, "stale"),
			wantRun: []uint64{5, 6}, wantPooled: []uint64{6},
		},
		{
			name: "non-nonce failure does not cascade",
			seqs: []uint64{5, 7, 8}, failSeq: 7, failErr: errorsmod.Wrap(sdkerrors.ErrInsufficientFunds, "no funds"),
			wantRun: []uint64{5, 7, 8}, wantPooled: []uint64{5, 8},
		},
		{
			// e.g. the nonce was taken by another tx.
			name: "nonce error at lastOK+1 does not cascade",
			seqs: []uint64{5, 6, 7}, failSeq: 6, failErr: wrongSeq,
			wantRun: []uint64{5, 6, 7}, wantPooled: []uint64{5, 7},
		},
		{
			name: "non-ascending candidate order is sorted before the gap rule",
			seqs: []uint64{5, 9, 7}, failSeq: 9, failErr: wrongSeq,
			wantRun: []uint64{5, 7, 9}, wantPooled: []uint64{5, 7},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := newRecheckFixture()
			bz := func(seq uint64) string { return "carl-" + strconv.FormatUint(seq, 10) }
			bySeq := make(map[uint64]sdk.Tx, len(tc.seqs))
			candidates := make([]sdk.Tx, len(tc.seqs))
			for i, seq := range tc.seqs {
				candidates[i] = f.add(i+1, "carl", seq, bz(seq))
				bySeq[seq] = candidates[i]
			}
			f.runner.failErrs = map[string]error{bz(tc.failSeq): tc.failErr}

			f.a.sched.runRecheck(f.a.sched.groupCandidates(candidates), 0)

			for _, seq := range tc.wantRun {
				if !f.runner.seen[bz(seq)] {
					t.Errorf("carl-%d must run", seq)
				}
			}
			for _, seq := range tc.wantSkipped {
				if f.runner.seen[bz(seq)] {
					t.Errorf("carl-%d is behind a proven gap and must be evicted without a RunTx", seq)
				}
			}
			for _, seq := range tc.seqs {
				if want := slices.Contains(tc.wantPooled, seq); poolHas(f.pool, bySeq[seq]) != want {
					t.Errorf("carl-%d pooled = %v, want %v", seq, !want, want)
				}
			}
		})
	}
}

type removePanicPool struct {
	*sdkmempool.PriorityNonceMempool[int64]
	poison sdk.Tx
}

func (p removePanicPool) Remove(tx sdk.Tx) error {
	if tx == p.poison {
		panicking("Remove")
	}
	return p.PriorityNonceMempool.Remove(tx)
}

func TestRunRecheck_PanicSkipsOnlyThatTx(t *testing.T) {
	gap := errorsmod.Wrap(sdkerrors.ErrWrongSequence, "gap")
	noFunds := errorsmod.Wrap(sdkerrors.ErrInsufficientFunds, "no funds")
	carl := []string{carlSeq5Bytes, carlSeq6Bytes, carlSeq7Bytes, carlSeq8Bytes, "carl-9"}

	testCases := []struct {
		name      string
		malleate  func(f *recheckFixture, txs map[string]sdk.Tx)
		expPooled []string
		expRunTx  []string
	}{
		{
			name: "RunTx panic evicts the tx and the gap it leaves cascades",
			malleate: func(f *recheckFixture, _ map[string]sdk.Tx) {
				f.runner.panicBytes = map[string]bool{carlSeq6Bytes: true}
				f.runner.failErrs = map[string]error{carlSeq7Bytes: gap}
			},
			expPooled: []string{carlSeq5Bytes},
			expRunTx:  []string{carlSeq5Bytes, carlSeq6Bytes, carlSeq7Bytes},
		},
		{
			name: "evict panic after a failed RunTx leaves that tx pooled and the chunk goes on",
			malleate: func(f *recheckFixture, txs map[string]sdk.Tx) {
				f.runner.failErrs = map[string]error{carlSeq6Bytes: noFunds}
				f.a.sched.mpool = removePanicPool{f.pool, txs[carlSeq6Bytes]}
			},
			expPooled: carl,
			expRunTx:  carl,
		},
		{
			name: "cascade evict panic leaves only that sibling pooled",
			malleate: func(f *recheckFixture, txs map[string]sdk.Tx) {
				f.runner.failErrs = map[string]error{carlSeq6Bytes: noFunds, carlSeq7Bytes: gap}
				f.a.sched.mpool = removePanicPool{f.pool, txs[carlSeq8Bytes]}
			},
			expPooled: []string{carlSeq5Bytes, carlSeq8Bytes},
			expRunTx:  []string{carlSeq5Bytes, carlSeq6Bytes, carlSeq7Bytes},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := newRecheckFixture()
			txs := make(map[string]sdk.Tx)
			var candidates []sdk.Tx
			for i, bz := range carl {
				txs[bz] = f.add(i+1, "carl", uint64(5+i), bz)
				candidates = append(candidates, txs[bz])
			}
			candidates = append(candidates, f.add(10, "dave", 0, "dave-0"))
			tc.malleate(f, txs)

			f.a.sched.runRecheck(f.a.sched.groupCandidates(candidates), 0)

			for _, bz := range carl {
				if got := poolHas(f.pool, txs[bz]); got != slices.Contains(tc.expPooled, bz) {
					t.Fatalf("%s pooled=%v, want %v", bz, got, !got)
				}
				if got := f.runner.seen[bz]; got != slices.Contains(tc.expRunTx, bz) {
					t.Fatalf("%s reached RunTx=%v, want %v", bz, got, !got)
				}
			}
			if !f.runner.seen["dave-0"] {
				t.Fatal("a later group must still be rechecked")
			}
		})
	}
}

// addEth pools tx keyed the way the EVM signer adapter keys it: by the first
// MsgEthereumTx's sender and nonce only.
func (f *recheckFixture) addEth(tx sdk.Tx, from common.Address, nonce uint64, bz string) {
	f.signer.m[tx] = []sdkmempool.SignerData{sdkmempool.NewSignerData(sdk.AccAddress(from.Bytes()), nonce)}
	if err := f.pool.Insert(sdk.Context{}, tx); err != nil {
		panic(err)
	}
	f.enc.Set(tx, []byte(bz))
}

func ethBatch(from []common.Address, nonces []uint64) *multiMsgTx {
	msgs := make([]sdk.Msg, len(nonces))
	for i, n := range nonces {
		msgs[i] = newEthTx(from[i], n).msg
	}
	return &multiMsgTx{msgs: msgs}
}

func TestGroupCandidates_CascadeGuards(t *testing.T) {
	alice, bob := sdk.AccAddress("alice"), sdk.AccAddress("bob")
	ethAlice, ethBob := common.Address{0xa1}, common.Address{0xb0}
	ethAliceKey := sdk.AccAddress(ethAlice.Bytes()).String()
	ethBobKey := sdk.AccAddress(ethBob.Bytes()).String()

	testCases := []struct {
		name           string
		malleate       func(t *testing.T, f *recheckFixture) []sdk.Tx
		wantCascadable map[string]bool
		// check, if set, asserts anything beyond the cascadable flags.
		check func(t *testing.T, f *recheckFixture, groups []recheckGroup, txs []sdk.Tx)
	}{
		{
			name: "single eth msg keeps cascade",
			malleate: func(_ *testing.T, f *recheckFixture) []sdk.Tx {
				a5, a6 := newEthTx(ethAlice, 5), newEthTx(ethAlice, 6)
				f.addEth(a5, ethAlice, 5, "a5")
				f.addEth(a6, ethAlice, 6, "a6")
				return []sdk.Tx{a5, a6}
			},
			wantCascadable: map[string]bool{ethAliceKey: true},
		},
		{
			// The group is keyed on the first signer, so the co-signers' nonces
			// the multi-signer tx also advances are invisible there.
			name: "multi-signer tx disables its own group",
			malleate: func(_ *testing.T, f *recheckFixture) []sdk.Tx {
				return []sdk.Tx{f.insert(1, alice, 3), f.insert(2, alice, 5, bob)}
			},
			wantCascadable: map[string]bool{alice.String(): false},
		},
		{
			// Alice's own group [3, 5] looks clean with a gap at 4, but the
			// bob-keyed tx also carries alice at nonce 4 and would fill it.
			name: "multi-signer tx disables the co-signer's group",
			malleate: func(_ *testing.T, f *recheckFixture) []sdk.Tx {
				return []sdk.Tx{f.insert(1, alice, 3), f.insert(2, alice, 5), f.insert(3, bob, 4, alice)}
			},
			wantCascadable: map[string]bool{alice.String(): false, bob.String(): false},
		},
		{
			name: "same-sender batch eth tx disables its group",
			malleate: func(_ *testing.T, f *recheckFixture) []sdk.Tx {
				batch := ethBatch([]common.Address{ethAlice, ethAlice, ethAlice}, []uint64{5, 6, 7})
				a8 := newEthTx(ethAlice, 8)
				f.addEth(batch, ethAlice, 5, "batch")
				f.addEth(a8, ethAlice, 8, "a8")
				return []sdk.Tx{batch, a8}
			},
			wantCascadable: map[string]bool{ethAliceKey: false},
		},
		{
			// The adapter keys the batch on alice only, so bob's nonce 3 is
			// advanced from outside bob's group.
			name: "multi-sender batch eth tx disables the co-sender's group",
			malleate: func(_ *testing.T, f *recheckFixture) []sdk.Tx {
				batch := ethBatch([]common.Address{ethAlice, ethBob}, []uint64{5, 3})
				b4, b6 := newEthTx(ethBob, 4), newEthTx(ethBob, 6)
				f.addEth(batch, ethAlice, 5, "batch")
				f.addEth(b4, ethBob, 4, "b4")
				f.addEth(b6, ethBob, 6, "b6")
				return []sdk.Tx{batch, b4, b6}
			},
			wantCascadable: map[string]bool{ethAliceKey: false, ethBobKey: false},
		},
		{
			// ChooseNonce keys an unordered tx by timeout and its seq is 0, so
			// next to 6, 7, 8 the group would otherwise look clean.
			name: "unordered tx disables its group",
			malleate: func(t *testing.T, f *recheckFixture) []sdk.Tx {
				t.Helper()
				unordered := &ptrTx{id: 1, unordered: true, timeoutTS: time.Now().Add(time.Hour)}
				f.signer.m[unordered] = []sdkmempool.SignerData{sdkmempool.NewSignerData(alice, 0)}
				if err := f.pool.Insert(sdk.Context{}, unordered); err != nil {
					t.Fatal(err)
				}
				return []sdk.Tx{unordered, f.insert(2, alice, 6), f.insert(3, alice, 7), f.insert(4, alice, 8)}
			},
			wantCascadable: map[string]bool{alice.String(): false},
		},
		{
			name: "tx whose encode panics is dropped and disables its group",
			malleate: func(_ *testing.T, f *recheckFixture) []sdk.Tx {
				poison := f.insert(1, alice, 5) // no encCache entry, so grouping encodes it
				f.a.exec.txEncoder = func(sdk.Tx) ([]byte, error) { panicking(encodeSite); return nil, nil }
				return []sdk.Tx{poison, f.add(2, "alice", 6, "alice-6")}
			},
			wantCascadable: map[string]bool{alice.String(): false},
			check: func(t *testing.T, f *recheckFixture, groups []recheckGroup, txs []sdk.Tx) {
				t.Helper()
				if poolHas(f.pool, txs[0]) {
					t.Fatal("a tx whose encode panics must be evicted")
				}
				if len(groups[0].txs) != 1 || groups[0].txs[0].tx != txs[1] {
					t.Fatal("only the sibling must be left to recheck")
				}
			},
		},
		{
			name: "tx whose encode fails is dropped and disables its group",
			malleate: func(_ *testing.T, f *recheckFixture) []sdk.Tx {
				f.a.exec.txEncoder = func(sdk.Tx) ([]byte, error) { return nil, errors.New("encode") }
				return []sdk.Tx{f.insert(1, alice, 5), f.add(2, "alice", 6, "alice-6")}
			},
			wantCascadable: map[string]bool{alice.String(): false},
		},
		{
			name: "duplicate seq disables its group and keeps pool order",
			malleate: func(_ *testing.T, f *recheckFixture) []sdk.Tx {
				return []sdk.Tx{f.add(1, "alice", 5, "alice-5a"), f.add(2, "alice", 5, "alice-5b")}
			},
			wantCascadable: map[string]bool{alice.String(): false},
			check: func(t *testing.T, _ *recheckFixture, groups []recheckGroup, txs []sdk.Tx) {
				t.Helper()
				if groups[0].txs[0].tx != txs[0] || groups[0].txs[1].tx != txs[1] {
					t.Fatal("stable sort must preserve pool order for equal-seq txs")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := newRecheckFixture()
			txs := tc.malleate(t, f)
			groups := f.a.sched.groupCandidates(txs)
			if len(groups) != len(tc.wantCascadable) {
				t.Fatalf("expected %d groups, got %d", len(tc.wantCascadable), len(groups))
			}
			for _, g := range groups {
				if want := tc.wantCascadable[g.key]; g.cascadable != want {
					t.Fatalf("group %s: cascadable=%v, want %v", g.key, g.cascadable, want)
				}
			}
			if tc.check != nil {
				tc.check(t, f, groups, txs)
			}
		})
	}
}

func TestSigners_BatchEthTxIncludesCoSenders(t *testing.T) {
	alice, bob := common.Address{0xa1}, common.Address{0xb0}
	aliceKey := sdk.AccAddress(alice.Bytes()).String()
	bobKey := sdk.AccAddress(bob.Bytes()).String()

	testCases := []struct {
		name string
		tx   sdk.Tx
		want []string
	}{
		{"single eth msg", newEthTx(alice, 5), []string{aliceKey}},
		{"same-sender batch dedups", ethBatch([]common.Address{alice, alice}, []uint64{5, 6}), []string{aliceKey}},
		{"multi-sender batch adds co-sender", ethBatch([]common.Address{alice, bob}, []uint64{5, 3}), []string{aliceKey, bobKey}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := newRecheckFixture()
			f.addEth(tc.tx, alice, 5, "tx")
			if got := f.a.sched.signers(tc.tx); !slices.Equal(got, tc.want) {
				t.Fatalf("signers = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRecheckTxs_BatchEthTxRecheckedWhenCoSenderStaged(t *testing.T) {
	alice, bob := common.Address{0xa1}, common.Address{0xb0}
	f := newRecheckFixture()
	batch := ethBatch([]common.Address{alice, bob}, []uint64{5, 3})
	f.addEth(batch, alice, 5, "batch")
	aliceOnly := newEthTx(alice, 6)
	f.addEth(aliceOnly, alice, 6, "alice-6")

	// Only bob was in the block: the batch advances bob's nonce too, so it must
	// be rechecked; alice's own single tx must not.
	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress(bob.Bytes()).String(): {}}
	f.a.sched.RecheckTxs()

	if !f.runner.seen["batch"] {
		t.Fatal("a batch naming a staged co-sender must be rechecked")
	}
	if f.runner.seen["alice-6"] {
		t.Fatal("a tx of an unstaged sender must not be rechecked")
	}
}

// Batch A uses nonces 5-7 but is keyed on 5. Stale B at 7 must not read as a
// gap after 5, or C at 8, the next valid nonce, would be cascade-evicted.
func TestRunGroup_BatchEthTxStaleSiblingDoesNotCascade(t *testing.T) {
	alice := common.Address{0xa1}
	f := newRecheckFixture()
	f.runner.signer = f.signer
	f.runner.expectedNonce = map[string]uint64{sdk.AccAddress(alice.Bytes()).String(): 5}
	f.runner.nonceSpan = map[string]uint64{"A": 3}

	a := ethBatch([]common.Address{alice, alice, alice}, []uint64{5, 6, 7})
	b, c := newEthTx(alice, 7), newEthTx(alice, 8)
	f.addEth(a, alice, 5, "A")
	f.addEth(b, alice, 7, "B")
	f.addEth(c, alice, 8, "C")

	groups := f.a.sched.groupCandidates([]sdk.Tx{a, b, c})
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	evicted, cascaded, _ := f.a.sched.runGroup(groups[0], 0)

	if cascaded != 0 {
		t.Fatalf("a stale sibling after a batch must not cascade, got %v", cascaded)
	}
	if evicted != 1 || poolHas(f.pool, b) {
		t.Fatalf("only stale B must be evicted, got evicted=%v", evicted)
	}
	if !f.runner.seen["C"] || !poolHas(f.pool, c) {
		t.Fatal("C is the next expected nonce after the batch and must run and stay pooled")
	}
}

func TestGroupCandidates_SortsBySeqAscending(t *testing.T) {
	f := newRecheckFixture()
	seq5 := f.add(1, "alice", 5, "alice-5")
	seq3 := f.add(2, "alice", 3, "alice-3")
	seq4 := f.add(3, "alice", 4, "alice-4")

	groups := f.a.sched.groupCandidates([]sdk.Tx{seq5, seq3, seq4})

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	got := []uint64{groups[0].txs[0].seq, groups[0].txs[1].seq, groups[0].txs[2].seq}
	if got[0] != 3 || got[1] != 4 || got[2] != 5 {
		t.Fatalf("group must be sorted ascending by seq, got %v", got)
	}
	if !groups[0].cascadable {
		t.Fatal("an out-of-order group must stay cascadable once sorted")
	}
}

func TestRunGroup_LargerThanChunkRunsEveryCandidate(t *testing.T) {
	const total = recheckChunkSize + 50
	f := newRecheckFixture()
	txs, _ := f.addChain("alice", total, nil)

	groups := f.a.sched.groupCandidates(txs)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	evicted, cascaded, _ := f.a.sched.runGroup(groups[0], 0)
	if evicted != 0 || cascaded != 0 {
		t.Fatalf("expected no evictions, got evicted=%v cascaded=%v", evicted, cascaded)
	}
	if got := len(f.runner.calls); got != total {
		t.Fatalf("expected every candidate across chunk boundaries to run, got %d RunTx calls", got)
	}
	for i := 0; i < total; i++ {
		if !f.runner.seen[chainBytes("alice", i)] {
			t.Fatalf("alice-%d must have run", i)
		}
	}
}

func TestRunChunkLocked_HoldsGateOnlyForTheChunk(t *testing.T) {
	const total = recheckChunkSize + 3
	f := newRecheckFixture()
	txs, _ := f.addChain("fay", total, nil)
	var unlockedRuns int
	f.runner.onCall = func([]byte) {
		if f.a.exec.gate.mu.TryLock() {
			f.a.exec.gate.mu.Unlock()
			unlockedRuns++
		}
	}

	groups := f.a.sched.groupCandidates(txs)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	for start := 0; start < total; start += recheckChunkSize {
		end := min(start+recheckChunkSize, total)
		if _, _, _, ok := f.a.sched.runChunkLocked(groups[0], start, end, 0); !ok {
			t.Fatalf("chunk at %d aborted without a Commit", start)
		}
		if !f.a.exec.gate.mu.TryLock() {
			t.Fatalf("chunk at %d returned still holding the gate", start)
		}
		f.a.exec.gate.mu.Unlock()
	}
	if unlockedRuns != 0 {
		t.Fatalf("%d recheck RunTx calls ran without the gate", unlockedRuns)
	}
	if got := len(f.runner.calls); got != total {
		t.Fatalf("expected %d RunTx calls, got %d", total, got)
	}
}

// Each case moves the expected nonce as chunk 2's head runs, as admissions
// between gate holds would, and checks chunk 2 never blind-evicts a valid candidate.
func TestRunGroup_ChunkBoundaryNeverCarriesGapProof(t *testing.T) {
	const n = recheckChunkSize
	const total = n + 3 // chunk 1 = [0, n); chunk 2 = [n, n+3)
	// gapAtBoundary keeps 0..n-2 ascending, then jumps at index n-1 so chunk 1
	// proves a gap at its very last index — its cascade range is empty — and
	// chunk 2 continues ascending past the jump: n+4, n+5, n+6.
	gapAtBoundary := func(i int) uint64 {
		if i < n-1 {
			return uint64(i)
		}
		return uint64(n) + 3 + uint64(i-(n-1))
	}
	contiguous := func(i int) uint64 { return uint64(i) }

	testCases := []struct {
		name  string
		seqOf func(i int) uint64
		// nonceAfterChunk1 is the account's expected nonce once the admissions
		// that landed between the two lock holds have been applied.
		nonceAfterChunk1 uint64
		wantEvicted      int
		wantSurvivors    []int // chunk 2 indexes that must stay pooled and spend their own RunTx
		wantGone         []int
	}{
		{
			name:             "gap filled exactly up to chunk 2's head",
			seqOf:            gapAtBoundary,
			nonceAfterChunk1: uint64(n) + 4,
			wantEvicted:      1,
			wantSurvivors:    []int{n, n + 1, n + 2},
			wantGone:         []int{n - 1},
		},
		{
			// The head is now stale, not gapped: reading its nonce error as
			// "the gap held" would blind-evict the valid tail behind it.
			name:             "gap filled past chunk 2's head",
			seqOf:            gapAtBoundary,
			nonceAfterChunk1: uint64(n) + 5,
			wantEvicted:      2,
			wantSurvivors:    []int{n + 1, n + 2},
			wantGone:         []int{n - 1, n},
		},
		{
			// No gap anywhere; two same-sender admissions overtake chunk 2's
			// first two candidates. The second stale failure sits above chunk
			// 1's last accepted nonce + 1 and would pass the gap rule if that
			// cursor carried over.
			name:             "stale run after the boundary is not a gap",
			seqOf:            contiguous,
			nonceAfterChunk1: uint64(n) + 2,
			wantEvicted:      2,
			wantSurvivors:    []int{n + 2},
			wantGone:         []int{n, n + 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := newRecheckFixture()
			f.runner.signer = f.signer
			dave := sdk.AccAddress("dave").String()
			f.runner.expectedNonce = map[string]uint64{dave: 0}
			bz := func(i int) string { return chainBytes("dave", i) }
			txs, ptrTxs := f.addChain("dave", total, tc.seqOf)
			f.runner.onCall = func(b []byte) {
				if string(b) == bz(n) {
					f.runner.expectedNonce[dave] = tc.nonceAfterChunk1
				}
			}

			groups := f.a.sched.groupCandidates(txs)
			if len(groups) != 1 || !groups[0].cascadable {
				t.Fatalf("expected 1 cascadable group, got %+v", groups)
			}

			evicted, cascaded, _ := f.a.sched.runGroup(groups[0], 0)
			if evicted != tc.wantEvicted {
				t.Fatalf("expected %v direct evictions, got %v", tc.wantEvicted, evicted)
			}
			if cascaded != 0 {
				t.Fatalf("no gap is provable after the boundary, so nothing may cascade; got %v", cascaded)
			}
			for _, i := range tc.wantSurvivors {
				if !f.runner.seen[bz(i)] {
					t.Fatalf("candidate %d must have spent a RunTx, not been blind-evicted", i)
				}
				if !poolHas(f.pool, ptrTxs[i]) {
					t.Fatalf("candidate %d is valid and must not be evicted", i)
				}
			}
			for _, i := range tc.wantGone {
				if poolHas(f.pool, ptrTxs[i]) {
					t.Fatalf("candidate %d failed recheck and must be evicted", i)
				}
			}
		})
	}
}

func TestRunGroup_CommitBetweenChunksRestagesRest(t *testing.T) {
	const total = recheckChunkSize + 3
	f := newRecheckFixture()
	f.runner.signer = f.signer
	erin := sdk.AccAddress("erin").String()
	f.runner.expectedNonce = map[string]uint64{erin: 0}
	bz := func(i int) string { return chainBytes("erin", i) }
	txs, ptrTxs := f.addChain("erin", total, nil)
	f.runner.onCall = func(b []byte) {
		if string(b) == bz(recheckChunkSize-1) {
			f.a.exec.gate.commits++ // a Commit ran; recheck holds the gate here
		}
	}

	groups := f.a.sched.groupCandidates(txs)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	evicted, cascaded, restaged := f.a.sched.runGroup(groups[0], 0)
	if evicted != 0 || cascaded != 0 {
		t.Fatalf("a Commit between chunks must not evict anything, got evicted=%v cascaded=%v", evicted, cascaded)
	}
	if want := total - recheckChunkSize; restaged != want {
		t.Fatalf("expected %d candidates reported as restaged, got %d", want, restaged)
	}
	for i := recheckChunkSize; i < total; i++ {
		if f.runner.seen[bz(i)] {
			t.Fatalf("candidate %d ran against the reset checkState", i)
		}
		if !poolHas(f.pool, ptrTxs[i]) {
			t.Fatalf("candidate %d is valid and must stay pooled", i)
		}
	}
	if _, ok := f.a.sched.recheckSenders[erin]; !ok {
		t.Fatal("the sender must be re-staged so the next cycle rechecks its whole chain")
	}
}

func TestRecheckTxs_CommitBetweenGroupsRestagesRest(t *testing.T) {
	f := newRecheckFixture()
	f.runner.signer = f.signer
	bob, carl := sdk.AccAddress("bob").String(), sdk.AccAddress("carl").String()
	f.runner.expectedNonce = map[string]uint64{bob: 0, carl: 5}

	f.add(1, "bob", 0, "bob-0") // bob's group runs first
	f.add(2, "bob", 1, "bob-1")
	c5 := f.add(3, "carl", 5, "carl-5")
	c6 := f.add(4, "carl", 6, carlSeq6Bytes)
	c7 := f.add(5, "carl", 7, "carl-7")
	f.a.sched.lastCommittedHeight = 10
	f.a.sched.recheckSenders = map[string]struct{}{bob: {}, carl: {}}

	// Block 11 commits carl-5 right after bob's group and resets checkState. It
	// stages no sender, so only the stop's re-stage brings carl back.
	f.runner.onCall = func(b []byte) {
		if string(b) != "bob-1" {
			return
		}
		_ = f.pool.Remove(c5)
		f.runner.expectedNonce[carl] = 6
		f.a.exec.gate.commits++
		f.a.sched.stagingMu.Lock()
		f.a.sched.lastCommittedHeight = 11
		f.a.sched.stagingMu.Unlock()
	}

	f.a.sched.RecheckTxs()
	if f.runner.seen[carlSeq6Bytes] {
		t.Fatalf("carl's group must not run after the Commit: calls=%v", f.runner.calls)
	}
	f.a.sched.RecheckTxs() // the cycle that Commit triggers
	if !f.runner.seen[carlSeq6Bytes] {
		t.Fatalf("carl must be re-staged and rechecked: calls=%v", f.runner.calls)
	}
	if !poolHas(f.pool, c6) || !poolHas(f.pool, c7) {
		t.Fatalf("valid carl txs evicted by a second recheck on the same checkState: calls=%v", f.runner.calls)
	}
}

func TestRunRecheck_CommitBeforeFirstChunkRunsNothing(t *testing.T) {
	f := newRecheckFixture()
	f.add(1, "bob", 0, "bob-0")
	f.add(2, "carl", 0, "carl-0")
	f.a.exec.gate.commits++ // a Commit ran after the cycle pinned 0

	f.a.sched.runRecheck(f.a.sched.groupCandidates(PoolSnapshot(t.Context(), f.pool)), 0)

	if len(f.runner.calls) != 0 {
		t.Fatalf("no candidate may run past the pinned height: calls=%v", f.runner.calls)
	}
	for _, sender := range []string{"bob", "carl"} {
		if _, ok := f.a.sched.recheckSenders[sdk.AccAddress(sender).String()]; !ok {
			t.Fatalf("%s must be re-staged", sender)
		}
	}
}

func TestRecheckTxs_DrainsOnlyUnderTheGate(t *testing.T) {
	f := newRecheckFixture()
	f.add(1, "bob", 0, "bob-0")
	bob := sdk.AccAddress("bob").String()
	f.a.sched.recheckSenders = map[string]struct{}{bob: {}}

	f.a.exec.gate.lock() // stands in for a Commit holding the gate
	done := make(chan struct{})
	go func() {
		f.a.sched.RecheckTxs()
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	f.a.sched.stagingMu.Lock()
	_, staged := f.a.sched.recheckSenders[bob]
	f.a.sched.stagingMu.Unlock()
	f.a.exec.gate.unlock()
	<-done

	if !staged {
		t.Fatal("staging must not drain until the gate is free, or a Commit between drain and pin goes unseen")
	}
	if !f.runner.seen["bob-0"] {
		t.Fatal("the cycle must run once the gate is free")
	}
}

func TestRunGroup_CascadeStopsAtChunkBoundary(t *testing.T) {
	const total = 2*recheckChunkSize + recheckChunkSize/2  // spans 3 chunks
	const gapIndex = recheckChunkSize + recheckChunkSize/2 // inside chunk 2
	const chunk3Head = 2 * recheckChunkSize
	f := newRecheckFixture()
	f.runner.signer = f.signer // real nonce tracking: the gap must hold on its own, not via failErrs
	f.runner.expectedNonce = map[string]uint64{sdk.AccAddress("carl").String(): 0}
	bz := func(i int) string { return chainBytes("carl", i) }
	txs, ptrTxs := f.addChain("carl", total, func(i int) uint64 {
		if i >= gapIndex {
			return uint64(i) + 2 // opens a gap at gapIndex and keeps ascending order past it
		}
		return uint64(i)
	})

	groups := f.a.sched.groupCandidates(txs)
	if len(groups) != 1 || !groups[0].cascadable {
		t.Fatalf("expected 1 cascadable group, got %+v", groups)
	}

	evicted, cascaded, _ := f.a.sched.runGroup(groups[0], 0)
	// Direct evictions: the gapped candidate, plus every chunk-3 candidate,
	// each of which fails its own RunTx.
	if want := 1 + total - chunk3Head; evicted != want {
		t.Fatalf("expected %v direct evictions, got %v", want, evicted)
	}
	if want := chunk3Head - gapIndex - 1; cascaded != want {
		t.Fatalf("expected %v cascade-evicted siblings (rest of chunk 2 only), got %v", want, cascaded)
	}
	for i := gapIndex + 1; i < chunk3Head; i++ {
		if f.runner.seen[bz(i)] {
			t.Fatalf("chunk-2 sibling at index %d must be cascade-evicted without a RunTx", i)
		}
	}
	for i := chunk3Head; i < total; i++ {
		if !f.runner.seen[bz(i)] {
			t.Fatalf("chunk-3 candidate at index %d must spend its own RunTx: the gap proof does not cross the lock release", i)
		}
	}
	for i := gapIndex; i < total; i++ {
		if poolHas(f.pool, ptrTxs[i]) {
			t.Fatalf("candidate at index %d is behind the gap and must be evicted", i)
		}
	}
	if !f.runner.seen[bz(gapIndex-1)] {
		t.Fatal("the last successful candidate before the gap must have run")
	}
}

func TestSigners_NilSignerNoPanic(t *testing.T) {
	s := &recheckScheduler{}
	if got := s.signers(&ptrTx{id: 1}); got != nil {
		t.Fatalf("expected nil signers with a nil extractor, got %v", got)
	}
}

func TestRecheckTxs_DeferredCarryWithReplacedHeadDoesNotEvictTail(t *testing.T) {
	const batch = 3
	f := newRecheckFixture()
	f.runner.signer = f.signer
	f.runner.expectedNonce = map[string]uint64{sdk.AccAddress("alice").String(): 5}

	bob := f.add(1, "bob", 0, "bob-0")
	aliceSeqs := []uint64{5, 6, 7, 8, 9}
	alice := make([]*ptrTx, len(aliceSeqs))
	for i, seq := range aliceSeqs {
		alice[i] = f.add(10+i, "alice", seq, "alice-"+strconv.FormatUint(seq, 10))
	}

	f.a.sched.maxRecheckBatch = batch
	f.a.sched.recheckSenders = map[string]struct{}{
		sdk.AccAddress("bob").String():   {},
		sdk.AccAddress("alice").String(): {},
	}

	// Cycle 1: bob's group (1 tx) fits under the cap; alice's group (5 txs)
	// overflows and must defer whole.
	f.a.sched.RecheckTxs()
	if !poolHas(f.pool, bob) {
		t.Fatal("precondition: bob's tx must survive cycle 1")
	}
	if len(f.a.sched.deferred) != len(aliceSeqs) {
		t.Fatalf("precondition: alice's whole group must defer, got %d", len(f.a.sched.deferred))
	}

	// alice fee-bumps her head tx: same (sender, nonce) key, new tx identity.
	// PriorityNonceMempool.Insert replaces the deferred pointer's pool entry.
	bumped := f.add(99, "alice", aliceSeqs[0], "alice-5-bumped")
	if poolHas(f.pool, alice[0]) {
		t.Fatal("precondition: fee bump must replace the original nonce-5 entry")
	}

	// No block touches alice between cycles; her real nonce stays 5.
	// Cycle 2: recheckSenders is drained empty going in; only capRecheckGroups'
	// re-staging from cycle 1 covers alice here.
	f.a.sched.RecheckTxs()

	if !poolHas(f.pool, bumped) {
		t.Fatal("the fee-bumped replacement at nonce 5 must survive recheck")
	}
	for i, seq := range aliceSeqs[1:] {
		if !poolHas(f.pool, alice[i+1]) {
			t.Fatalf("alice's tail tx at seq %d must not be evicted after a head fee bump", seq)
		}
	}
}

// Documented residual: Remove matches by (sender, nonce), so evicting a stale
// candidate drops a fee bump that replaced it after the snapshot.
func TestEvict_KeyBasedRemovalDropsReplacementNotStaleTx(t *testing.T) {
	f := newRecheckFixture()
	stale := f.add(1, "alice", 0, "alice-0")

	// A fee bump lands at the same (sender, nonce) key before eviction runs.
	replacement := f.insert(2, sdk.AccAddress("alice"), 0)
	if poolHas(f.pool, stale) {
		t.Fatal("precondition: fee bump must replace the original nonce-0 entry")
	}

	f.a.sched.evict(stale)

	if poolHas(f.pool, replacement) {
		t.Fatal("key-based Remove must drop whatever occupies (alice, 0) now, i.e. the replacement")
	}
}

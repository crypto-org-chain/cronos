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
	// failErrs returns a specific error per tx bytes, without removing from the
	// pool, so tests can drive runRecheck's nonce-gap classification.
	failErrs map[string]error
	modes    []sdk.ExecMode
	seen     map[string]bool
	// calls records tx bytes in call order, for grouping assertions.
	calls []string
	// onCall, if set, runs after recording the call but before returning, letting
	// a test bump gen mid-pass to exercise runRecheck's cancellation check.
	onCall func(txBytes []byte)
	// signer + expectedNonce implement per-sender expected-nonce ante
	// semantics: a tx whose seq is above its sender's expected nonce fails
	// wrong-sequence, and a successful recheck advances that sender's expected
	// nonce. Nil signer disables this; other tests drive failBytes/failErrs.
	signer        sdkmempool.SignerExtractionAdapter
	expectedNonce map[string]uint64
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
		r.expectedNonce[sender] = seq + 1
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

// Reproduces the batch cap splitting a signer's nonce chain (pre-fix, the flat
// cap sliced the candidate list before grouping by signer). bob's one-tx group
// fills the cap; alice's five-tx chain must carry forward whole rather than
// being split mid-chain, so it revalidates cleanly from her real nonce (8)
// once a Commit lands between cycles and no valid tx is evicted.
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

	// A Commit lands between cycles: base rebranches off the committed store
	// (alice's chain was only rechecked, never included in a block, so her
	// real nonce is still 8) and gen advances.
	f.a.exec.gen.Add(1)

	// Cycle 2: recheckSenders is empty, but the deferred carry must still run
	// as one atomic group against alice's real nonce.
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

// A generation bump cannot split one chunk: gen only advances under the
// admission mutex, which recheckChunkLocked holds for the whole chunk. Both
// candidates here fit in a single chunk, so the bump — raised from inside
// RunTx, i.e. without the admission mutex — shows the chunk still completes;
// cancellation is a between-chunks decision.
func TestRecheckTxs_GenerationBumpDoesNotSplitAChunk(t *testing.T) {
	f := newRecheckFixture()
	f.add(1, "alice", 0, aliceSeq0Bytes)
	f.add(2, "alice", 1, "alice-1")

	f.runner.onCall = func(txBytes []byte) {
		if string(txBytes) == aliceSeq0Bytes {
			f.a.exec.gen.Add(1)
		}
	}
	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.sched.RecheckTxs()

	if !f.runner.seen[aliceSeq0Bytes] || !f.runner.seen["alice-1"] {
		t.Fatal("both candidates of one signer must run under the same stateMu hold")
	}
}

// TestRunRecheck_AbortRecoversUnreachedSendersWithoutClobberingDeferred covers
// the two-sender abort case explicitly: the unreached sender must land in
// staging (not just its raw tx, which runRecheck never touches), and the
// unreached tx must be appended to deferred (F3) alongside — not in place of
// — an already-set deferred carry from this same cycle's capRecheckGroups.
func TestRunRecheck_AbortRecoversUnreachedSendersWithoutClobberingDeferred(t *testing.T) {
	f := newRecheckFixture()
	aliceTx := f.add(1, "alice", 0, aliceSeq0Bytes)
	bobTx := f.add(2, "bob", 0, "bob-0")
	carryTx := f.add(3, "carol", 0, "carol-carry") // stands in for capRecheckGroups' overflow carry

	f.a.sched.deferred = []sdk.Tx{carryTx}

	f.runner.onCall = func(txBytes []byte) {
		if string(txBytes) == aliceSeq0Bytes {
			f.a.exec.gen.Add(1) // simulate a Commit's refresh landing after the first candidate
		}
	}
	gen := f.a.exec.gen.Load()
	f.a.sched.runRecheck(f.a.sched.groupCandidates([]sdk.Tx{aliceTx, bobTx}), gen)

	if !f.runner.seen[aliceSeq0Bytes] {
		t.Fatal("the candidate validated before the bump must still run")
	}
	if f.runner.seen["bob-0"] {
		t.Fatal("the candidate after the bump must be skipped, not rechecked against a superseded base")
	}
	if _, ok := f.a.sched.recheckSenders[sdk.AccAddress("bob").String()]; !ok {
		t.Fatal("bob must be re-covered in staging after its candidate was skipped")
	}
	if !slices.Equal(f.a.sched.deferred, []sdk.Tx{carryTx, bobTx}) {
		t.Fatalf("expected the abort to append bob's tx after the untouched carry, got %v", f.a.sched.deferred)
	}

	// Next RecheckTxs cycle: bob (re-covered) and the carried carol tx must both
	// get rechecked.
	f.runner.onCall = nil
	f.a.sched.RecheckTxs()

	if !f.runner.seen["bob-0"] {
		t.Fatal("the re-covered sender's tx must be rechecked by the next RecheckTxs cycle")
	}
	if !f.runner.seen["carol-carry"] {
		t.Fatal("the deferred carry must still be rechecked by the next RecheckTxs cycle")
	}
}

const (
	carlSeq5Bytes = "carl-5"
	carlSeq7Bytes = "carl-7"
	carlSeq8Bytes = "carl-8"
)

func TestRunRecheck_GroupsCandidatesBySigner(t *testing.T) {
	f := newRecheckFixture()
	aliceLow := f.add(1, "alice", 0, aliceSeq0Bytes)
	bob := f.add(2, "bob", 0, "bob-0")
	aliceHigh := f.add(3, "alice", 1, "alice-1")

	f.a.sched.runRecheck(f.a.sched.groupCandidates([]sdk.Tx{aliceLow, bob, aliceHigh}), f.a.exec.gen.Load())

	want := []string{aliceSeq0Bytes, "alice-1", "bob-0"}
	if !slices.Equal(f.runner.calls, want) {
		t.Fatalf("candidates must run grouped by signer in first-appearance order: got %v, want %v", f.runner.calls, want)
	}
}

func TestRunRecheck_NonceGapCascadesToHigherSiblings(t *testing.T) {
	f := newRecheckFixture()
	valid := f.add(1, "carl", 5, carlSeq5Bytes)
	gapped := f.add(2, "carl", 7, carlSeq7Bytes)
	higher := f.add(3, "carl", 8, carlSeq8Bytes)
	f.runner.failErrs = map[string]error{carlSeq7Bytes: errorsmod.Wrap(sdkerrors.ErrWrongSequence, "gap")}

	f.a.sched.runRecheck(f.a.sched.groupCandidates([]sdk.Tx{valid, gapped, higher}), f.a.exec.gen.Load())

	if f.runner.seen[carlSeq8Bytes] {
		t.Fatal("a sibling behind a proven nonce gap must be evicted without spending a RunTx")
	}
	if poolHas(f.pool, gapped) || poolHas(f.pool, higher) {
		t.Fatal("the gapped tx and its higher-nonce siblings must be evicted")
	}
	if !poolHas(f.pool, valid) {
		t.Fatal("the tx that passed recheck must stay in the pool")
	}
}

// A wrong-sequence failure with no accepted nonce before it may be a stale nonce
// (already committed), in which case the successor is the account's expected one.
func TestRunRecheck_StaleNonceDoesNotCascade(t *testing.T) {
	f := newRecheckFixture()
	stale := f.add(1, "carl", 5, carlSeq5Bytes)
	next := f.add(2, "carl", 6, "carl-6")
	f.runner.failErrs = map[string]error{carlSeq5Bytes: errorsmod.Wrap(sdkerrors.ErrInvalidSequence, "stale")}

	f.a.sched.runRecheck(f.a.sched.groupCandidates([]sdk.Tx{stale, next}), f.a.exec.gen.Load())

	if !f.runner.seen["carl-6"] {
		t.Fatal("the successor of a stale nonce must still be rechecked")
	}
	if poolHas(f.pool, stale) {
		t.Fatal("the stale tx must be evicted")
	}
	if !poolHas(f.pool, next) {
		t.Fatal("the successor must stay in the pool after passing recheck")
	}
}

func TestRunRecheck_NonNonceFailureDoesNotCascade(t *testing.T) {
	f := newRecheckFixture()
	valid := f.add(1, "carl", 5, carlSeq5Bytes)
	failing := f.add(2, "carl", 7, carlSeq7Bytes)
	higher := f.add(3, "carl", 8, carlSeq8Bytes)
	f.runner.failErrs = map[string]error{carlSeq7Bytes: errorsmod.Wrap(sdkerrors.ErrInsufficientFunds, "no funds")}

	f.a.sched.runRecheck(f.a.sched.groupCandidates([]sdk.Tx{valid, failing, higher}), f.a.exec.gen.Load())

	if !f.runner.seen[carlSeq8Bytes] {
		t.Fatal("only a nonce gap justifies skipping a sibling's RunTx")
	}
}

// groupCandidates sorts a group ascending by seq before it runs, so a
// non-ascending pool/deferred order (5, 9, 7) no longer disables the cascade —
// the group becomes the signer's clean ascending view (5, 7, 9), and every
// candidate still gets its own RunTx up to the real gap.
func TestRunRecheck_NonAscendingPoolOrderSortedBeforeCascade(t *testing.T) {
	f := newRecheckFixture()
	valid := f.add(1, "carl", 5, carlSeq5Bytes)
	gapped := f.add(2, "carl", 9, "carl-9")
	lower := f.add(3, "carl", 7, carlSeq7Bytes)
	f.runner.failErrs = map[string]error{"carl-9": errorsmod.Wrap(sdkerrors.ErrWrongSequence, "gap")}

	groups := f.a.sched.groupCandidates([]sdk.Tx{valid, gapped, lower})
	if len(groups) != 1 || !groups[0].cascadable {
		t.Fatalf("sorted group must be cascadable, got groups=%+v", groups)
	}

	f.a.sched.runRecheck(f.a.sched.groupCandidates([]sdk.Tx{valid, gapped, lower}), f.a.exec.gen.Load())

	if !f.runner.seen[carlSeq7Bytes] {
		t.Fatal("seq 7 sits between the valid and gapped candidates in the sorted group and must still run")
	}
	if !poolHas(f.pool, valid) || !poolHas(f.pool, lower) {
		t.Fatal("the two candidates that pass recheck must stay in the pool")
	}
	if poolHas(f.pool, gapped) {
		t.Fatal("the failing candidate must be evicted")
	}
}

// A multi-signer candidate makes its own group's nonce view incomplete: the
// group is keyed on the first signer, so the co-signers' nonces it also
// advances are invisible there.
func TestGroupCandidates_MultiSignerDisablesCascade(t *testing.T) {
	f := newRecheckFixture()
	single := f.insert(1, sdk.AccAddress("alice"), 3)
	multi := f.insert(2, sdk.AccAddress("alice"), 5, sdk.AccAddress("bob"))

	groups := f.a.sched.groupCandidates([]sdk.Tx{single, multi})

	if len(groups) != 1 {
		t.Fatalf("expected both txs in one group keyed on alice, got %d groups", len(groups))
	}
	if groups[0].cascadable {
		t.Fatal("a multi-signer candidate in the group must disable cascade")
	}
}

// The multi-signer tx is keyed on bob, so alice's own group [3, 5] looks like a
// clean ascending view with a gap at 4 — but the bob-keyed tx also carries
// alice at nonce 4 and would fill it. Every signer a multi-signer tx names must
// lose cascade, not just the group that tx lands in.
func TestGroupCandidates_MultiSignerDisablesCascadeInCoSignerGroup(t *testing.T) {
	f := newRecheckFixture()
	alice3 := f.insert(1, sdk.AccAddress("alice"), 3)
	alice5 := f.insert(2, sdk.AccAddress("alice"), 5)
	bobMulti := f.insert(3, sdk.AccAddress("bob"), 4, sdk.AccAddress("alice"))

	groups := f.a.sched.groupCandidates([]sdk.Tx{alice3, alice5, bobMulti})

	if len(groups) != 2 {
		t.Fatalf("expected an alice group and a bob group, got %d", len(groups))
	}
	for _, g := range groups {
		if g.cascadable {
			t.Fatalf("group %q must not cascade: the bob-keyed multi-signer tx names alice too", g.key)
		}
	}
}

// F2: an unordered tx keys its SignerData.Sequence at 0 (ChooseNonce orders it
// by timeout, not sequence), so a group holding it alongside ordered seqs
// 6, 7, 8 has no duplicate seq and would otherwise look like a clean
// ascending-nonce view. groupCandidates must disable cascade for it directly,
// since the seq it carries can't be reasoned about by the gap rule.
func TestGroupCandidates_UnorderedTxDisablesCascade(t *testing.T) {
	f := newRecheckFixture()
	unordered := &ptrTx{id: 1, unordered: true, timeoutTS: time.Now().Add(time.Hour)}
	f.signer.m[unordered] = []sdkmempool.SignerData{sdkmempool.NewSignerData(sdk.AccAddress("alice"), 0)}
	if err := f.pool.Insert(sdk.Context{}, unordered); err != nil {
		t.Fatal(err)
	}
	seq6 := f.insert(2, sdk.AccAddress("alice"), 6)
	seq7 := f.insert(3, sdk.AccAddress("alice"), 7)
	seq8 := f.insert(4, sdk.AccAddress("alice"), 8)

	groups := f.a.sched.groupCandidates([]sdk.Tx{unordered, seq6, seq7, seq8})

	if len(groups) != 1 {
		t.Fatalf("expected 1 group keyed on alice, got %d", len(groups))
	}
	if groups[0].cascadable {
		t.Fatal("an unordered tx in the group must disable cascade")
	}
}

// Deferred front-loading can hand groupCandidates an out-of-nonce-order group
// (e.g. alice-5 ahead of alice-3 and alice-4). Without sorting, alice-5 would
// run first and fail wrong-sequence even though it becomes valid two txs later.
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
}

// The seq <= previous-seq check still needs to catch duplicates once sorting
// is in play, and the sort must be stable so tied seqs keep pool order.
func TestGroupCandidates_DuplicateSeqDisablesCascadeStableOrder(t *testing.T) {
	f := newRecheckFixture()
	first := f.add(1, "alice", 5, "alice-5a")
	second := f.add(2, "alice", 5, "alice-5b") // duplicate seq

	groups := f.a.sched.groupCandidates([]sdk.Tx{first, second})

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g := groups[0]
	if g.cascadable {
		t.Fatal("a duplicate seq within a group must disable cascade")
	}
	if g.txs[0].tx != first || g.txs[1].tx != second {
		t.Fatal("stable sort must preserve pool order for equal-seq txs")
	}
}

// A group larger than recheckChunkSize must still run every candidate: the
// chunking in runGroup bounds one mutex hold, not how much of the group
// eventually gets rechecked.
func TestRunGroup_LargerThanChunkRunsEveryCandidate(t *testing.T) {
	const total = recheckChunkSize + 50
	f := newRecheckFixture()
	txs := make([]sdk.Tx, total)
	for i := 0; i < total; i++ {
		txs[i] = f.add(i+1, "alice", uint64(i), "alice-"+strconv.Itoa(i))
	}

	groups := f.a.sched.groupCandidates(txs)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	evicted, cascaded, unreachedFrom := f.a.sched.runGroup(groups[0], f.a.exec.gen.Load())
	if unreachedFrom != -1 {
		t.Fatalf("expected the whole group reached, got unreachedFrom=%d", unreachedFrom)
	}
	if evicted != 0 || cascaded != 0 {
		t.Fatalf("expected no evictions, got evicted=%v cascaded=%v", evicted, cascaded)
	}
	if got := len(f.runner.calls); got != total {
		t.Fatalf("expected every candidate across chunk boundaries to run, got %d RunTx calls", got)
	}
	for i := 0; i < total; i++ {
		if !f.runner.seen["alice-"+strconv.Itoa(i)] {
			t.Fatalf("alice-%d must have run", i)
		}
	}
}

// F1 regression: a gap proven at the very last index of a chunk leaves the
// cascade range for that chunk empty (g.txs[i+1:end] has nothing in it), so
// nothing was actually evicted under the lock hold that proved the gap. If a
// same-sender admission fills the gap before the next chunk's turn,
// cascadeChunkLocked must discover that with a RunTx on the next chunk's own
// head rather than blind-evicting a nonce that is now valid.
func TestRunGroup_CascadeChunkHeadRunTxWhenGapProvenAtChunkBoundary(t *testing.T) {
	const n = recheckChunkSize
	const total = n + 2 // chunk 1 = [0, n); chunk 2 = [n, n+2)
	f := newRecheckFixture()
	f.runner.signer = f.signer
	dave := sdk.AccAddress("dave").String()
	f.runner.expectedNonce = map[string]uint64{dave: 0}

	seqOf := func(i int) uint64 {
		switch {
		case i < n-1:
			return uint64(i) // 0..n-2: ascending, all valid
		case i == n-1:
			return uint64(n) + 3 // last of chunk 1: opens a gap (skips n-1, n, n+1, n+2)
		default:
			return uint64(n) + 3 + uint64(i-(n-1)) // chunk 2: continues ascending past the gap
		}
	}
	bz := func(i int) string { return "dave-" + strconv.Itoa(i) }
	txs := make([]sdk.Tx, total)
	ptrTxs := make([]*ptrTx, total)
	for i := 0; i < total; i++ {
		ptrTxs[i] = f.add(i+1, "dave", seqOf(i), bz(i))
		txs[i] = ptrTxs[i]
	}

	// A same-sender admission lands between chunk 1's lock release and chunk
	// 2's cascadeChunkLocked call, filling every nonce the gap skipped — by
	// the time chunk 2 runs, the account's expected nonce matches chunk 2's
	// head exactly.
	f.runner.onCall = func(b []byte) {
		if string(b) == bz(n-1) {
			f.runner.expectedNonce[dave] = seqOf(n)
		}
	}

	groups := f.a.sched.groupCandidates(txs)
	if len(groups) != 1 || !groups[0].cascadable {
		t.Fatalf("expected 1 cascadable group, got %+v", groups)
	}

	evicted, cascaded, unreachedFrom := f.a.sched.runGroup(groups[0], f.a.exec.gen.Load())
	if unreachedFrom != -1 {
		t.Fatalf("expected the whole group reached, got unreachedFrom=%d", unreachedFrom)
	}
	if evicted != 1 {
		t.Fatalf("expected exactly 1 eviction (the originally gapped tx), got %v", evicted)
	}
	if cascaded != 0 {
		t.Fatalf("expected no blind cascade eviction once the gap closed, got %v", cascaded)
	}
	for i := n; i < total; i++ {
		if !f.runner.seen[bz(i)] {
			t.Fatalf("candidate %d must have spent a RunTx, not been blind-evicted", i)
		}
		if !poolHas(f.pool, ptrTxs[i]) {
			t.Fatalf("candidate %d is now valid and must not be evicted", i)
		}
	}
	if poolHas(f.pool, ptrTxs[n-1]) {
		t.Fatal("the originally gapped tx must still be evicted")
	}
}

// A gen bump landing exactly at a chunk boundary must abort the group there:
// the completed chunk stays rechecked, and the untouched tail's sender is
// re-staged so the next cycle covers it — mirroring the same-generation
// recovery runRecheck already does between groups.
func TestRunRecheck_GenBumpAtChunkBoundaryAbortsAndRestagesSender(t *testing.T) {
	const total = recheckChunkSize + 50
	f := newRecheckFixture()
	txs := make([]sdk.Tx, total)
	ptrTxs := make([]*ptrTx, total)
	for i := 0; i < total; i++ {
		ptrTxs[i] = f.add(i+1, "alice", uint64(i), "alice-"+strconv.Itoa(i))
		txs[i] = ptrTxs[i]
	}
	lastOfFirstChunk := "alice-" + strconv.Itoa(recheckChunkSize-1)
	f.runner.onCall = func(txBytes []byte) {
		if string(txBytes) == lastOfFirstChunk {
			f.a.exec.gen.Add(1) // simulate a Commit landing right as the first chunk finishes
		}
	}

	gen := f.a.exec.gen.Load()
	groups := f.a.sched.groupCandidates(txs)
	f.a.sched.runRecheck(groups, gen)

	if got := len(f.runner.calls); got != recheckChunkSize {
		t.Fatalf("expected exactly the first chunk (%d) to run, got %d", recheckChunkSize, got)
	}
	for i := recheckChunkSize; i < total; i++ {
		if f.runner.seen["alice-"+strconv.Itoa(i)] {
			t.Fatalf("candidate %d in the aborted second chunk must not have run", i)
		}
		if !poolHas(f.pool, ptrTxs[i]) {
			t.Fatalf("candidate %d must remain in the pool after the abort", i)
		}
	}
	if _, ok := f.a.sched.recheckSenders[sdk.AccAddress("alice").String()]; !ok {
		t.Fatal("alice must be re-staged after the aborted chunk so the next cycle covers her unreached tail")
	}
}

// A nonce gap discovered in a later chunk must still cascade-evict every
// higher-nonce sibling, including ones that live in a chunk beyond the one
// where the gap was found — except each further chunk's own head now spends
// a RunTx (F1 fix) to confirm the gap actually survived the lock release at
// that boundary, so it isn't the same blind cascade past the first chunk.
func TestRecheckGroup_CascadeEvictsAcrossChunkBoundary(t *testing.T) {
	const total = 3*recheckChunkSize - 88 // spans 3 chunks; boundaries at 256, 512
	const gapIndex = 400                  // inside chunk 2 ([256, 512))
	const chunk3Head = 2 * recheckChunkSize
	f := newRecheckFixture()
	f.runner.signer = f.signer // real nonce tracking: the gap must hold on its own, not via failErrs
	f.runner.expectedNonce = map[string]uint64{sdk.AccAddress("carl").String(): 0}
	txs := make([]sdk.Tx, total)
	ptrTxs := make([]*ptrTx, total)
	bz := func(i int) string { return "carl-" + strconv.Itoa(i) }
	for i := 0; i < total; i++ {
		seq := uint64(i)
		if i >= gapIndex {
			seq += 2 // opens a gap at gapIndex and keeps ascending order past it
		}
		ptrTxs[i] = f.add(i+1, "carl", seq, bz(i))
		txs[i] = ptrTxs[i]
	}

	groups := f.a.sched.groupCandidates(txs)
	if len(groups) != 1 || !groups[0].cascadable {
		t.Fatalf("expected 1 cascadable group, got %+v", groups)
	}

	evicted, cascaded, unreachedFrom := f.a.sched.runGroup(groups[0], f.a.exec.gen.Load())
	if unreachedFrom != -1 {
		t.Fatalf("expected the whole group reached (run or cascade-evicted), got unreachedFrom=%d", unreachedFrom)
	}
	// Two real RunTx-driven evictions: the gapped candidate itself, and chunk
	// 3's head re-checking whether the gap survived its own chunk boundary.
	if evicted != 2 {
		t.Fatalf("expected 2 direct evictions (the gapped tx and the next chunk's head), got %v", evicted)
	}
	if want := float32(total - gapIndex - 2); cascaded != want {
		t.Fatalf("expected %v cascade-evicted siblings, got %v", want, cascaded)
	}
	for i := gapIndex + 1; i < total; i++ {
		if i == chunk3Head {
			continue
		}
		if f.runner.seen[bz(i)] {
			t.Fatalf("sibling at index %d must be cascade-evicted without a RunTx", i)
		}
		if poolHas(f.pool, ptrTxs[i]) {
			t.Fatalf("sibling at index %d must be evicted from the pool", i)
		}
	}
	if !f.runner.seen[bz(chunk3Head)] {
		t.Fatal("the next chunk's own head must spend a RunTx to check whether the gap survived to this chunk")
	}
	if poolHas(f.pool, ptrTxs[chunk3Head]) {
		t.Fatal("the next chunk's head must still be evicted since the gap held")
	}
	if !f.runner.seen[bz(gapIndex-1)] {
		t.Fatal("the last successful candidate before the gap must have run")
	}
}

// F3: gen is read right before runRecheck, not right after drainStaging, so a
// Commit landing during the O(pool) scan/grouping no longer wastes the whole
// pass at group 0. Bumping gen as PoolSnapshot starts (before selectTxs runs)
// simulates that landing point.
func TestRecheckTxs_GenBumpBeforeScanStillRunsCandidates(t *testing.T) {
	signer := fakeSigner{m: map[sdk.Tx][]sdkmempool.SignerData{}}
	tx := &ptrTx{id: 1}
	signer.m[tx] = []sdkmempool.SignerData{sdkmempool.NewSignerData(sdk.AccAddress("alice"), 0)}
	pool := &scanHookMempool{txs: []sdk.Tx{tx}}
	runner := &recheckRunner{pool: pool, failBytes: map[string]bool{}, seen: map[string]bool{}}
	txEncoder := func(sdk.Tx) ([]byte, error) { return []byte("alice-0"), nil }
	a := newManager(runner, NewEncoderCache(0, 0), txEncoder, func([]byte) (sdk.Tx, error) { return nil, errors.New("unused") })
	a.sched.mpool = pool
	a.sched.signer = signer
	a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	pool.onScan = func() { a.exec.gen.Add(1) }

	a.sched.RecheckTxs()

	if !runner.seen["alice-0"] {
		t.Fatal("a gen bump before the scan starts must not abort the pass before it runs anything")
	}
}

// scanHookMempool runs onScan when the pool size is first queried (the start
// of PoolSnapshot), so a test can simulate a Commit's gen bump landing right
// as the O(pool) scan begins.
type scanHookMempool struct {
	txs    []sdk.Tx
	onScan func()
}

func (m *scanHookMempool) Insert(context.Context, sdk.Tx) error                 { return nil }
func (m *scanHookMempool) Select(context.Context, [][]byte) sdkmempool.Iterator { return nil }
func (m *scanHookMempool) CountTx() int {
	if m.onScan != nil {
		m.onScan()
	}
	return len(m.txs)
}

func (m *scanHookMempool) Remove(tx sdk.Tx) error {
	for i, t := range m.txs {
		if t == tx {
			m.txs = append(m.txs[:i], m.txs[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *scanHookMempool) RemoveWithReason(_ context.Context, tx sdk.Tx, _ sdkmempool.RemoveReason) error {
	return m.Remove(tx)
}

func (m *scanHookMempool) SelectBy(_ context.Context, _ [][]byte, cb func(sdk.Tx) bool) {
	for _, tx := range m.txs {
		if !cb(tx) {
			return
		}
	}
}

// firstSigner has a nil guard on s.signer; signers() must agree so an abort
// path (recoverSenders -> signers) can't panic when the scheduler was never
// wired with a signer extractor.
func TestSigners_NilSignerNoPanic(t *testing.T) {
	s := &recheckScheduler{}
	if got := s.signers(&ptrTx{id: 1}); got != nil {
		t.Fatalf("expected nil signers with a nil extractor, got %v", got)
	}
}

// F1 regression: the deferred carry from capRecheckGroups is tx-identity-keyed
// (deferredLive), so it alone cannot survive a fee bump replacing the head of
// a deferred group at the same (sender, nonce) key. capRecheckGroups must
// also merge the deferred groups' senders into recheckSenders, so the next
// cycle's selectTxs re-picks alice's whole live queue by sender instead of
// relying on the stale deferred pointer. Without that, the surviving tail
// (seq 6-9) would be regrouped alone, fail wrong-sequence against a base
// still expecting nonce 5, and be evicted in full.
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

	// No block touches alice between cycles; her real nonce stays 5. A Commit
	// still lands (gen advances) but doesn't change the fake runner's nonce
	// view, mirroring "alice's chain was only rechecked, never included".
	f.a.exec.gen.Add(1)

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

// F2 (documented residual, not fixed in code): PriorityNonceMempool.Remove
// resolves by (sender, nonce) key, not tx identity, so evicting a stale
// recheck candidate drops whatever currently occupies that key. If an
// admission lands between the snapshot and the eviction and replaces the slot
// (e.g. a fee bump), the freshly admitted replacement is what gets dropped,
// not the stale tx the pass was actually rechecking.
func TestEvict_KeyBasedRemovalDropsReplacementNotStaleTx(t *testing.T) {
	f := newRecheckFixture()
	stale := f.add(1, "alice", 0, "alice-0")

	// A fee bump lands at the same (sender, nonce) key before eviction runs.
	replacement := f.insert(2, sdk.AccAddress("alice"), 0)
	if poolHas(f.pool, stale) {
		t.Fatal("precondition: fee bump must replace the original nonce-0 entry")
	}

	f.a.sched.evict(stale, "", 0, false)

	if poolHas(f.pool, replacement) {
		t.Fatal("key-based Remove must drop whatever occupies (alice, 0) now, i.e. the replacement")
	}
}

// evictionRecorder is a fake eviction hook recording every (sender, nonce)
// it's invoked with, for asserting the scheduler notifies eviction even when
// it never spends a RunTx on the evicted tx (cascade and TTL evictions).
type evictionRecorder struct {
	mu    sync.Mutex
	calls []struct {
		sender string
		nonce  uint64
	}
}

func (r *evictionRecorder) hook(sender string, nonce uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, struct {
		sender string
		nonce  uint64
	}{sender, nonce})
}

func (r *evictionRecorder) has(sender string, nonce uint64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.calls {
		if c.sender == sender && c.nonce == nonce {
			return true
		}
	}
	return false
}

// F1: a cascade-evicted sibling never spends a RunTx, so the eviction hook is
// the only signal available for dropping its App-level ante state (e.g.
// ethermint's per-tx nonce cache).
func TestEvictionHook_InvokedOnCascadeEviction(t *testing.T) {
	f := newRecheckFixture()
	valid := f.add(1, "carl", 5, carlSeq5Bytes)
	gapped := f.add(2, "carl", 7, carlSeq7Bytes)
	higher := f.add(3, "carl", 8, carlSeq8Bytes)
	f.runner.failErrs = map[string]error{carlSeq7Bytes: errorsmod.Wrap(sdkerrors.ErrWrongSequence, "gap")}

	rec := &evictionRecorder{}
	f.a.sched.evictionHook = rec.hook

	f.a.sched.runRecheck(f.a.sched.groupCandidates([]sdk.Tx{valid, gapped, higher}), f.a.exec.gen.Load())

	carl := sdk.AccAddress("carl").String()
	if !rec.has(carl, 7) {
		t.Fatal("eviction hook must fire for the gapped candidate's own eviction")
	}
	if !rec.has(carl, 8) {
		t.Fatal("eviction hook must fire for a cascade-evicted sibling, which never spends a RunTx")
	}
	if rec.has(carl, 5) {
		t.Fatal("eviction hook must not fire for a candidate that passed recheck")
	}
}

// F1: a TTL eviction never spends a RunTx either, so it needs the same hook.
func TestEvictionHook_InvokedOnTTLEviction(t *testing.T) {
	f := newRecheckFixture()
	f.a.sched.ttlNumBlocks = 5
	aged := f.add(1, "alice", 3, "alice-3")

	f.a.sched.lastCommittedHeight = 10
	f.a.sched.RecheckTxs() // first sighting: records arrival, tx survives

	rec := &evictionRecorder{}
	f.a.sched.evictionHook = rec.hook

	f.a.sched.lastCommittedHeight = 15 // 15-10 == ttl -> evicted
	f.a.sched.RecheckTxs()

	if poolHas(f.pool, aged) {
		t.Fatal("precondition: TTL-aged tx must be evicted")
	}
	if !rec.has(sdk.AccAddress("alice").String(), 3) {
		t.Fatal("eviction hook must fire for a TTL eviction, which never spends a RunTx")
	}
}

// F2: cascadeChunkLocked's chunk head can fail for a reason other than a
// nonce error (e.g. insufficient funds), which carries no information about
// whether the previous chunk's proven gap survived. Blindly cascading the
// rest of the chunk in that case could evict a candidate that is actually the
// account's next expected nonce, so a non-nonce head failure must fall
// through to a per-candidate recheck instead.
func TestRunGroup_CascadeChunkNonNonceHeadFailureFallsThroughToPerCandidateRecheck(t *testing.T) {
	const n = recheckChunkSize
	const total = n + 2 // chunk 1 = [0, n); chunk 2 = [n, n+2)
	f := newRecheckFixture()

	seqOf := func(i int) uint64 {
		switch {
		case i < n-1:
			return uint64(i)
		case i == n-1:
			return uint64(n) + 3 // opens the gap chunk 1 proves
		default:
			return uint64(n) + 3 + uint64(i-(n-1)) // chunk 2 continues ascending past the gap
		}
	}
	bz := func(i int) string { return "dave-" + strconv.Itoa(i) }
	txs := make([]sdk.Tx, total)
	ptrTxs := make([]*ptrTx, total)
	for i := 0; i < total; i++ {
		ptrTxs[i] = f.add(i+1, "dave", seqOf(i), bz(i))
		txs[i] = ptrTxs[i]
	}
	f.runner.failErrs = map[string]error{
		bz(n - 1): errorsmod.Wrap(sdkerrors.ErrWrongSequence, "gap"),          // chunk 1 proves the gap
		bz(n):     errorsmod.Wrap(sdkerrors.ErrInsufficientFunds, "no funds"), // chunk 2's head fails, but not on a nonce error
	}

	groups := f.a.sched.groupCandidates(txs)
	if len(groups) != 1 || !groups[0].cascadable {
		t.Fatalf("expected 1 cascadable group, got %+v", groups)
	}

	evicted, cascaded, unreachedFrom := f.a.sched.runGroup(groups[0], f.a.exec.gen.Load())
	if unreachedFrom != -1 {
		t.Fatalf("expected the whole group reached, got unreachedFrom=%d", unreachedFrom)
	}
	if evicted != 2 {
		t.Fatalf("expected 2 direct evictions (the gapped tx and chunk 2's failing head), got %v", evicted)
	}
	if cascaded != 0 {
		t.Fatalf("a non-nonce head failure must not blind-cascade the rest of the chunk, got %v", cascaded)
	}
	if !f.runner.seen[bz(n+1)] {
		t.Fatal("the candidate after a non-nonce head failure must still spend its own RunTx, not be blind-evicted")
	}
	if !poolHas(f.pool, ptrTxs[n+1]) {
		t.Fatal("that candidate passed recheck and must survive")
	}
	if poolHas(f.pool, ptrTxs[n-1]) || poolHas(f.pool, ptrTxs[n]) {
		t.Fatal("the originally gapped tx and chunk 2's failing head must both be evicted")
	}
}

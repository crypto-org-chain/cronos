package mempool

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
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

// recheckRunner records the ExecMode and bytes of each RunTx call. It mirrors
// BaseApp.RunTx(ExecModeReCheck): on ante failure it removes the tx from the
// pool, which is what RecheckTxs relies on (it only evicts encCache itself).
type recheckRunner struct {
	mu        sync.Mutex
	pool      sdkmempool.Mempool
	failBytes map[string]bool
	modes     []sdk.ExecMode
	seen      map[string]bool
}

func (r *recheckRunner) RunTx(mode sdk.ExecMode, txBytes []byte, tx sdk.Tx, _ int, _ storetypes.MultiStore, _ map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modes = append(r.modes, mode)
	r.seen[string(txBytes)] = true
	if r.failBytes[string(txBytes)] {
		_ = r.pool.Remove(tx) // baseapp removes on recheck failure
		return sdk.GasInfo{}, nil, nil, errors.New("ante failed on recheck")
	}
	return sdk.GasInfo{}, &sdk.Result{}, nil, nil
}

// recheckFixture builds a real PriorityNonceMempool + admitter wired for recheck.
type recheckFixture struct {
	a      *Admitter
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
	enc := NewEncoderCache(0)
	fail := make(map[string]bool, len(failBytes))
	for _, b := range failBytes {
		fail[b] = true
	}
	runner := &recheckRunner{pool: pool, failBytes: fail, seen: map[string]bool{}}
	// Per-tx encoder so the encCache-miss fallback yields deterministic bytes.
	txEncoder := func(tx sdk.Tx) ([]byte, error) { return []byte("enc-" + strconv.Itoa(tx.(*ptrTx).id)), nil }
	decoder := func([]byte) (sdk.Tx, error) { return nil, errors.New("unused") }
	a := newAdmitter(runner, enc, txEncoder, decoder)
	a.mpool = pool
	a.signer = signer
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

	f.a.pending = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.RecheckTxs()

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

func TestRecheckTxs_EmptyPendingNoOp(t *testing.T) {
	f := newRecheckFixture()
	f.add(1, "alice", 0, "alice-0")

	f.a.RecheckTxs() // pending nil

	if len(f.runner.modes) != 0 {
		t.Fatalf("no RunTx expected with empty pending, got %d calls", len(f.runner.modes))
	}
}

func TestRecheckTxs_DrainsPending(t *testing.T) {
	f := newRecheckFixture()
	f.add(1, "alice", 0, "alice-0")
	f.a.pending = map[string]struct{}{sdk.AccAddress("alice").String(): {}}

	f.a.RecheckTxs()
	first := len(f.runner.modes)
	f.a.RecheckTxs() // pending consumed; second run is a no-op

	if len(f.runner.modes) != first {
		t.Fatal("pending must be drained after one RecheckTxs")
	}
}

// Timeout sweep evicts an expired tx even when its sender wasn't touched by the
// last block (no pending entry, so the ante-recheck path never sees it).
func TestRecheckTxs_EvictsExpiredUntouchedSender(t *testing.T) {
	f := newRecheckFixture()
	expired := f.addTimeout(1, "carol", 0, "carol-0", 5)

	f.a.lastCommittedHeight = 5 // next block = 6 > timeoutHeight 5 → never valid again
	f.a.RecheckTxs()            // pending nil: only the timeout sweep runs

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

	f.a.lastCommittedHeight = 5
	f.a.RecheckTxs()

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

	f.a.pending = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.lastCommittedHeight = 5
	f.a.RecheckTxs()

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
// timeout sweep fires on the next RecheckTxs. The fixture's decoder is nil, so
// staging returns after recording height — exercising height independently.
func TestStageRecheckSenders_StagesHeightForSweep(t *testing.T) {
	f := newRecheckFixture()
	expired := f.addTimeout(1, "carol", 0, "carol-0", 5)

	f.a.StageRecheckSenders(5, nil) // decoder nil: stages height, leaves pending nil
	f.a.RecheckTxs()

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
	a := newAdmitter(&stubRunner{}, nil, noopEncoder, decoder)
	a.signer = signer

	a.StageRecheckSenders(10, [][]byte{[]byte("a")})
	a.StageRecheckSenders(11, [][]byte{[]byte("b")}) // no drain between: must keep alice

	if _, ok := a.pending[sdk.AccAddress("alice").String()]; !ok {
		t.Fatal("block-10 sender lost after staging block 11 without a recheck drain")
	}
	if _, ok := a.pending[sdk.AccAddress("bob").String()]; !ok {
		t.Fatal("block-11 sender missing")
	}
	if a.lastCommittedHeight != 11 {
		t.Fatalf("height must advance to 11, got %d", a.lastCommittedHeight)
	}
}

func TestStageRecheckSenders_NoDepsNoPanic(t *testing.T) {
	a := newAdmitter(&stubRunner{}, nil, noopEncoder, nil)
	a.StageRecheckSenders(0, [][]byte{[]byte("x")}) // decoder/signer nil → no-op
	a.RecheckTxs()                                  // mpool nil → no-op
}

// A tx with no encCache entry must still be rechecked via the txEncoder fallback.
func TestRecheckTxs_EncoderFallbackOnCacheMiss(t *testing.T) {
	f := newRecheckFixture("enc-1") // encoder yields "enc-<id>"; fail id 1
	stale := f.insert(1, sdk.AccAddress("alice"), 0)
	if _, ok := f.enc.Get(stale); ok {
		t.Fatal("precondition: tx must not be in encCache")
	}
	f.a.pending = map[string]struct{}{sdk.AccAddress("alice").String(): {}}

	f.a.RecheckTxs()

	if !f.runner.seen["enc-1"] {
		t.Fatal("cache-miss tx must be rechecked using encoder-produced bytes")
	}
	if poolHas(f.pool, stale) {
		t.Fatal("stale cache-miss tx must be removed")
	}
}

// A multi-signer tx must be rechecked when ANY of its signers is in pending,
// even though the pool keys it by the first signer only.
func TestRecheckTxs_MultiSignerMatchesAnySigner(t *testing.T) {
	f := newRecheckFixture("enc-1")
	// pool key = alice (first signer); pending names only the second signer, bob.
	stale := f.insert(1, sdk.AccAddress("alice"), 0, sdk.AccAddress("bob"))
	f.a.pending = map[string]struct{}{sdk.AccAddress("bob").String(): {}}

	f.a.RecheckTxs()

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

// RecheckTxs must not run more than maxRecheckBatch RunTx calls in one cycle.
func TestRecheckTxs_BatchCapLimitsCandidates(t *testing.T) {
	const total = 5
	const batch = 2
	f := newRecheckFixture()
	for i := 0; i < total; i++ {
		f.add(i+1, "alice", uint64(i), "alice-"+strconv.Itoa(i))
	}
	f.a.maxRecheckBatch = batch
	f.a.pending = map[string]struct{}{sdk.AccAddress("alice").String(): {}}

	f.a.RecheckTxs()

	if got := len(f.runner.modes); got != batch {
		t.Fatalf("expected %d RunTx calls with batch cap, got %d", batch, got)
	}
}

// Overflow past the batch cap must carry forward and drain over later cycles —
// front-loaded so the priority-ordered tail isn't re-deferred forever — with
// every tx rechecked exactly once.
func TestRecheckTxs_BatchCapCarriesOverflow(t *testing.T) {
	const total = 5
	const batch = 2
	f := newRecheckFixture()
	for i := 0; i < total; i++ {
		f.add(i+1, "alice", uint64(i), "alice-"+strconv.Itoa(i))
	}
	f.a.maxRecheckBatch = batch
	f.a.pending = map[string]struct{}{sdk.AccAddress("alice").String(): {}}

	// Cycle 1 touches alice; cycles 2-3 have empty pending but must still drain
	// the carried overflow.
	f.a.RecheckTxs()
	f.a.RecheckTxs()
	f.a.RecheckTxs()

	if got := len(f.runner.modes); got != total {
		t.Fatalf("expected all %d txs rechecked across cycles, got %d", total, got)
	}
	for i := 0; i < total; i++ {
		if !f.runner.seen["alice-"+strconv.Itoa(i)] {
			t.Fatalf("alice-%d was never rechecked (starved past the cap)", i)
		}
	}
	if f.a.deferred != nil {
		t.Fatalf("deferred queue must be drained, still holds %d", len(f.a.deferred))
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
	f.a.pending = map[string]struct{}{sdk.AccAddress("alice").String(): {}}

	f.a.RecheckTxs()

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

	// Three blocks each touch alice only; carol is never in pending.
	for i := 0; i < 3; i++ {
		f.a.pending = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
		f.a.RecheckTxs()
	}

	if !poolHas(f.pool, idle) {
		t.Fatal("untouched sender's tx must remain in the pool")
	}
	if f.runner.seen["carol-0"] {
		t.Fatal("untouched sender's tx must never be rechecked")
	}
}

// Known limitation: the timeout sweep evicts an expired middle nonce, but the
// surviving higher nonce of the same (untouched) sender is left in the pool with
// a nonce gap. Recheck won't catch it — the sender isn't in pending — so the
// higher-nonce tx can still be reaped into a proposal and fail the AnteHandler at
// FinalizeBlock. This test pins that behavior so a future fix is a deliberate change.
func TestRecheckTxs_NonceGapAfterTimeoutEvictionSurvives(t *testing.T) {
	f := newRecheckFixture()
	expired := f.addTimeout(1, "carol", 0, "carol-0", 5) // nonce 0, times out at height 5
	gapped := f.addTimeout(2, "carol", 1, "carol-1", 0)  // nonce 1, no timeout

	f.a.lastCommittedHeight = 5 // sweep evicts nonce 0; carol not in pending
	f.a.RecheckTxs()

	if poolHas(f.pool, expired) {
		t.Fatal("expired middle nonce must be swept")
	}
	if !poolHas(f.pool, gapped) {
		t.Fatal("higher nonce survives the sweep, leaving a nonce gap recheck won't catch")
	}
	if f.runner.seen["carol-1"] {
		t.Fatal("untouched sender's surviving tx must not be rechecked")
	}
}

// Doing it inside the callback would pin mp.mtx (and run RunTx's Remove under
// it) across the whole scan, blocking admission/reap on the commit path.
func TestRecheckTxs_SignerExtractionOutsidePoolLock(t *testing.T) {
	pool := &lockTrackingMempool{}
	signer := &lockObservingSigner{m: map[sdk.Tx][]sdkmempool.SignerData{}, pool: pool}
	enc := NewEncoderCache(0)
	runner := &recheckRunner{pool: pool, failBytes: map[string]bool{}, seen: map[string]bool{}}
	txEncoder := func(tx sdk.Tx) ([]byte, error) { return []byte("enc-" + strconv.Itoa(tx.(*ptrTx).id)), nil }
	a := newAdmitter(runner, enc, txEncoder, func([]byte) (sdk.Tx, error) { return nil, errors.New("unused") })
	a.mpool = pool
	a.signer = signer

	tx := &ptrTx{id: 1}
	signer.m[tx] = []sdkmempool.SignerData{sdkmempool.NewSignerData(sdk.AccAddress("alice"), 0)}
	_ = pool.Insert(context.Background(), tx)
	a.pending = map[string]struct{}{sdk.AccAddress("alice").String(): {}}

	a.RecheckTxs()

	if signer.sawLocked {
		t.Fatal("signer extraction ran inside SelectBy (under the pool lock)")
	}
	if !runner.seen["enc-1"] {
		t.Fatal("candidate from a touched sender must still be rechecked")
	}
}

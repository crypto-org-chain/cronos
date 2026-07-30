package mempool

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"

	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// recheckScheduler is the recheck half of the app mempool: it stages the senders
// each block touched, picks which pending txs to re-validate, and evicts the ones
// the new state invalidated.
type recheckScheduler struct {
	exec   *txExec
	mpool  sdkmempool.Mempool
	signer sdkmempool.SignerExtractionAdapter
	// maxRecheckBatch softly caps RunTx(ReCheck) calls per Commit cycle: it splits
	// only at group boundaries, so a signer's whole nonce chain always runs
	// together even if that group alone exceeds the cap. 0 = unlimited.
	maxRecheckBatch int
	// stagingMu guards the staging fields (recheckSenders, deferred, lastCommittedHeight).
	// Separate from the admission mutex so FinalizeBlock staging never blocks behind a recheck batch.
	stagingMu sync.Mutex
	// recheckSenders accumulates senders of committed blocks awaiting recheck; merged
	// (not overwritten) across blocks so an un-drained block's senders aren't lost.
	recheckSenders map[string]struct{}
	// deferred is an ordering hint only: it front-loads capRecheckGroups'
	// overflow ahead of fresh candidates next cycle. The actual guarantee that
	// those senders get re-picked comes from capRecheckGroups also merging
	// their senders into recheckSenders — deferred alone would miss a carried
	// tx whose pool identity changed, e.g. a fee bump replacing it at the same
	// nonce.
	deferred            []sdk.Tx
	lastCommittedHeight int64
	// arrival maps each pooled tx to the height RecheckTxs first observed it, for
	// ttlNumBlocks eviction. Rebuilt from the snapshot each cycle; recheckMu keeps it single-writer.
	arrival map[sdk.Tx]int64
	// ttlNumBlocks evicts txs older than this many blocks by arrival height; 0 = off.
	ttlNumBlocks int64

	recheckMu sync.Mutex // serializes RecheckTxs; always acquired before the admission mutex and stagingMu, never after
	// Zero-value (trigger nil) when built via the newManager() test constructor;
	// TriggerRecheck then runs RecheckTxs inline instead of async.
	worker recheckWorker
	// recheckDisabled mirrors mempool.recheck=false: skips all rechecking,
	// including TTL/expiry eviction
	recheckDisabled bool
	// evictionHook, if set, is notified once per (sender, nonce) named by an
	// evicted tx — every signer for a multi-signer tx, not just the group's key
	// signer — so App-level state keyed on the same pair (e.g. ethermint's ante
	// nonce cache) can be dropped along with it. Nil-safe: a nil hook is a no-op.
	evictionHook func(sender string, nonce uint64)
}

// recheckDecodingEnabled reports whether sender decoding/bookkeeping should run.
func (s *recheckScheduler) recheckDecodingEnabled() bool {
	return !s.recheckDisabled && s.signer != nil && s.exec.decoder != nil
}

// stageSkippedSenders merges the senders of proposal-gate-rejected txs into
// recheckSenders without touching lastCommittedHeight
func (s *recheckScheduler) stageSkippedSenders(txs [][]byte) {
	if !s.recheckDecodingEnabled() || len(txs) == 0 {
		return
	}
	senders := make(map[string]struct{}, len(txs))
	for _, bz := range txs {
		tx, err := s.exec.decoder(bz)
		if err != nil {
			continue
		}
		for _, sg := range s.signers(tx) {
			senders[sg] = struct{}{}
		}
	}
	if len(senders) == 0 {
		return
	}
	s.stagingMu.Lock()
	s.mergeRecheckSenders(senders)
	s.stagingMu.Unlock()
}

func (s *recheckScheduler) mergeRecheckSenders(senders map[string]struct{}) {
	// mergeRecheckSenders folds senders into recheckSenders without overwriting, so a
	// block whose Commit skipped RecheckTxs doesn't lose its staged senders.
	if s.recheckSenders == nil {
		s.recheckSenders = senders
	} else {
		for sg := range senders {
			s.recheckSenders[sg] = struct{}{}
		}
	}
}

// stageRecheckSenders records the senders of the just-committed block's txs so
// RecheckTxs can re-validate only their remaining pending txs, and stages the
// committed height.
func (s *recheckScheduler) stageRecheckSenders(height int64, txs [][]byte) {
	// Decode + extract signers unlocked (the expensive part), then publish height
	// and recheckSenders in one critical section so a reader never sees a torn update.
	var senders map[string]struct{}
	if s.recheckDecodingEnabled() {
		senders = make(map[string]struct{}, len(txs))
		for _, bz := range txs {
			tx, err := s.exec.decoder(bz)
			if err != nil {
				continue // non-sdk txs (e.g. vote extensions) have no mempool entry
			}
			for _, sg := range s.signers(tx) {
				senders[sg] = struct{}{}
			}
		}
	}

	s.stagingMu.Lock()
	s.lastCommittedHeight = height
	s.mergeRecheckSenders(senders)
	s.stagingMu.Unlock()
}

// triggerRecheck schedules an async recheck.
// Call only from the consensus path (App.Commit).
func (s *recheckScheduler) triggerRecheck() {
	if s.worker.trigger == nil {
		s.RecheckTxs()
		return
	}
	s.worker.recheck()
}

// RecheckTxs evicts pool txs invalidated by the last block.
func (s *recheckScheduler) RecheckTxs() {
	if s.mpool == nil || s.recheckDisabled {
		return
	}
	s.recheckMu.Lock() // lock order: see the recheckMu field comment
	defer s.recheckMu.Unlock()
	recheckSenders, height, deferred := s.drainStaging()
	// Before the first block (height 0) with no senders/carry there's nothing to scan.
	if len(recheckSenders) == 0 && len(deferred) == 0 && height == 0 {
		return
	}

	snapshot := PoolSnapshot(context.Background(), s.mpool)
	candidates := s.selectTxs(snapshot, recheckSenders, height, deferred)
	groups := s.capRecheckGroups(s.groupCandidates(candidates))
	// Read gen only now: it must cover the RunTx phase below, not the O(pool)
	// scan/grouping above, or a Commit landing during the scan would abort the
	// whole pass before a single group runs.
	gen := s.exec.gen.Load()
	s.runRecheck(groups, gen)

	telemetry.SetGauge(float32(s.mpool.CountTx()), "cronos", "mempool", "pool", "size")
}

// drainStaging atomically takes and clears the staged senders, height, and carry.
func (s *recheckScheduler) drainStaging() (recheckSenders map[string]struct{}, height int64, deferred []sdk.Tx) {
	s.stagingMu.Lock()
	defer s.stagingMu.Unlock()
	recheckSenders, height, deferred = s.recheckSenders, s.lastCommittedHeight, s.deferred
	s.recheckSenders = nil
	s.deferred = nil
	return recheckSenders, height, deferred
}

// selectTxs scans the pool to retrieve txs for recheck. Caller (RecheckTxs)
// only invokes this when recheck is enabled.
func (s *recheckScheduler) selectTxs(snapshot []sdk.Tx, recheckSenders map[string]struct{}, height int64, deferred []sdk.Tx) []sdk.Tx {
	// deferredLive: carried-over tx -> still in pool. Sized to the small carry; nil if none.
	var deferredLive map[sdk.Tx]bool
	if len(deferred) > 0 {
		deferredLive = make(map[sdk.Tx]bool, len(deferred))
		for _, tx := range deferred {
			deferredLive[tx] = false
		}
	}

	var (
		expiredEvicted float32
		ttlEvicted     float32
	)
	// Rebuild arrival from this cycle's snapshot so txs gone from the pool fall out.
	var newArrival map[sdk.Tx]int64
	if s.ttlNumBlocks > 0 {
		newArrival = make(map[sdk.Tx]int64, len(snapshot))
	}

	// Pass 1: evictions. Collect senders of evicted txs so their remaining pool txs
	// (e.g. higher-nonce siblings) are rechecked — they become invalid after the gap.
	var evictedSet map[sdk.Tx]struct{} // nil until first eviction; nil-map read is safe
	now := time.Now()
	for _, tx := range snapshot {
		if txTimedout(tx, height, now) {
			evictedSet, recheckSenders = s.evictForRecheck(tx, evictedSet, recheckSenders)
			expiredEvicted++
			continue
		}
		if s.ttlNumBlocks > 0 {
			arrived, expired := txTTLExpired(s.arrival, tx, height, s.ttlNumBlocks)
			if expired {
				evictedSet, recheckSenders = s.evictForRecheck(tx, evictedSet, recheckSenders)
				ttlEvicted++
				continue
			}
			newArrival[tx] = arrived
		}
	}
	s.arrival = newArrival
	if expiredEvicted > 0 {
		telemetry.IncrCounter(expiredEvicted, "cronos", "mempool", "recheck", "expired")
	}
	if ttlEvicted > 0 {
		telemetry.IncrCounter(ttlEvicted, "cronos", "mempool", "recheck", "ttl_expired")
	}

	// Pass 2: candidate selection over surviving (non-evicted) txs.
	var candidates []sdk.Tx
	for _, tx := range snapshot {
		if _, wasEvicted := evictedSet[tx]; wasEvicted {
			continue
		}
		if deferredLive != nil {
			if _, isDeferred := deferredLive[tx]; isDeferred {
				deferredLive[tx] = true
			}
		}
		if len(recheckSenders) == 0 {
			continue
		}
		for _, sg := range s.signers(tx) {
			if _, ok := recheckSenders[sg]; ok {
				candidates = append(candidates, tx)
				break
			}
		}
	}

	if len(deferred) == 0 {
		return candidates
	}
	// Front-load surviving deferred ahead of fresh candidates: the snapshot is
	// priority-ordered, so otherwise capRecheckGroups re-takes the same prefix and starves the tail.
	ordered := make([]sdk.Tx, 0, len(deferred)+len(candidates))
	for _, tx := range deferred {
		if deferredLive[tx] {
			ordered = append(ordered, tx) // skip txs included/evicted since carry
		}
	}
	for _, tx := range candidates {
		if _, isDeferred := deferredLive[tx]; isDeferred {
			continue // this tx is already in the deferred carry; avoid double recheck
		}
		ordered = append(ordered, tx)
	}
	return ordered
}

// evictForRecheck evicts tx and folds its signers into recheckSenders, allocating
// evictedSet/recheckSenders lazily so a no-eviction cycle stays alloc-free.
func (s *recheckScheduler) evictForRecheck(tx sdk.Tx, evictedSet map[sdk.Tx]struct{}, recheckSenders map[string]struct{}) (map[sdk.Tx]struct{}, map[string]struct{}) {
	// firstSigner already does the GetSigners lookup this needs for the eviction
	// hook; reuse it for the single-signer case below instead of calling
	// s.signers (a second GetSigners) just to get the same one key back.
	key, seq, known, multiSigner := s.firstSigner(tx)
	s.evict(tx, key, seq, known, multiSigner)
	if evictedSet == nil {
		evictedSet = make(map[sdk.Tx]struct{})
	}
	evictedSet[tx] = struct{}{}
	var sigs []string
	if multiSigner {
		sigs = s.signers(tx)
	} else if known {
		sigs = []string{key}
	}
	if len(sigs) > 0 && recheckSenders == nil {
		recheckSenders = make(map[string]struct{})
	}
	for _, sg := range sigs {
		recheckSenders[sg] = struct{}{}
	}
	return evictedSet, recheckSenders
}

// capRecheckGroups bounds RunTx(ReCheck) calls per cycle without ever
// splitting a signer's group: the first group always runs in full regardless
// of size, and once the running total would exceed maxRecheckBatch the
// remaining groups carry forward whole into deferred. Their senders are also
// merged into recheckSenders (the recoverSenders path), because deferred is
// keyed on tx identity: a fee bump replacing a carried tx at the same nonce
// would otherwise vanish from deferredLive next cycle and take its whole live
// tail down as a false wrong-sequence failure. Re-selecting by sender instead
// picks up whatever the pool holds for that (sender, nonce) key now.
func (s *recheckScheduler) capRecheckGroups(groups []recheckGroup) []recheckGroup {
	if s.maxRecheckBatch <= 0 {
		return groups
	}
	count := 0
	for i, g := range groups {
		if count > 0 && count+len(g.txs) > s.maxRecheckBatch {
			carry := unreachedTxs(groups[i:])
			s.stagingMu.Lock()
			s.deferred = carry
			s.stagingMu.Unlock()
			s.recoverSenders(carry)
			return groups[:i]
		}
		count += len(g.txs)
	}
	return groups
}

// recheckCandidate carries the signer nonce alongside the tx: telling a nonce
// gap from a merely stale nonce is what makes cascade eviction safe.
type recheckCandidate struct {
	tx  sdk.Tx
	bz  []byte
	seq uint64
	// multiSigner mirrors firstSigner's flag from group build time, so evict
	// knows to fire the hook for every named signer without a second GetSigners
	// call on the hot recheck path.
	multiSigner bool
}

// recheckGroup holds one signer's candidates sorted ascending by seq.
// cascadable is false when the group is not that signer's contiguous
// ascending-nonce view — an unknown signer, a signer named by a multi-signer tx
// (that tx is grouped elsewhere, so it can fill a nonce this group can't see), a
// duplicate seq, an unordered tx (keyed by timeout, not sequence), or a tx
// dropped on encode error — because the cascade rule reasons about the next
// expected nonce.
type recheckGroup struct {
	key string
	txs []recheckCandidate
	// known reports whether key identifies a real signer, set once at group
	// creation and never flipped back — unlike cascadable, which also turns
	// false for reasons unrelated to identity (multi-signer, unordered,
	// duplicate seq). The eviction hook needs known, not cascadable.
	known      bool
	cascadable bool
}

// runRecheck re-validates candidates via RunTx(ReCheck), one signer group at a
// time so a sender's nonce chain advances atomically with respect to other
// senders' admissions. The pass is abandoned once gen advances mid-flight: the
// remaining candidates would be validated against a base a concurrent Commit
// has already superseded. drainStaging already cleared recheckSenders for this
// cycle, so the unreached candidates' senders are re-merged into staging here —
// otherwise a sender that isn't touched again by a later block would never be
// rechecked until TTL. They're also appended to deferred, so selectTxs front-
// loads them ahead of the priority-ordered snapshot's same old prefix next
// cycle, same as capRecheckGroups' overflow carry.
func (s *recheckScheduler) runRecheck(groups []recheckGroup, gen uint64) {
	var evicted, cascaded, superseded float32
	for i, g := range groups {
		if len(g.txs) == 0 {
			continue
		}
		e, c, unreachedFrom := s.runGroup(g, gen)
		evicted += e
		cascaded += c
		if unreachedFrom != -1 {
			unreached := make([]sdk.Tx, 0, len(g.txs)-unreachedFrom)
			for _, cand := range g.txs[unreachedFrom:] {
				unreached = append(unreached, cand.tx)
			}
			unreached = append(unreached, unreachedTxs(groups[i+1:])...)
			superseded += float32(len(unreached))
			s.recoverSenders(unreached)
			s.appendDeferred(unreached)
			break
		}
	}
	if evicted > 0 {
		telemetry.IncrCounter(evicted, "cronos", "mempool", "recheck", "evicted")
	}
	if cascaded > 0 {
		telemetry.IncrCounter(cascaded, "cronos", "mempool", "recheck", "cascade_evicted")
	}
	if superseded > 0 {
		telemetry.IncrCounter(superseded, "cronos", "mempool", "recheck", "superseded")
	}
}

// groupCandidates buckets candidates by first signer — the one the mempool
// orders by — keeping first-appearance order across groups. Encoding happens
// here, outside the admission mutex, to keep the per-group hold to RunTx.
// Within a group, candidates are sorted ascending by seq: deferred front-
// loading can hand candidates out of nonce order, and running them out of
// order would fail wrong-sequence against a nonce that a later candidate in
// the same group would have satisfied.
func (s *recheckScheduler) groupCandidates(candidates []sdk.Tx) []recheckGroup {
	var groups []recheckGroup
	index := make(map[string]int)
	// Every signer named by a multi-signer tx: that tx is grouped under its first
	// signer only, so it can advance a co-signer's nonce from outside that
	// co-signer's group, making a gap there unprovable.
	var coSigned map[string]struct{}
	for _, tx := range candidates {
		key, seq, known, multiSigner := s.firstSigner(tx)
		gi, seen := index[key]
		if !seen {
			groups = append(groups, recheckGroup{key: key, known: known, cascadable: known})
			gi = len(groups) - 1
			index[key] = gi
		}
		g := &groups[gi]
		if multiSigner {
			if coSigned == nil {
				coSigned = make(map[string]struct{})
			}
			for _, sg := range s.signers(tx) {
				coSigned[sg] = struct{}{}
			}
		}
		if unordered, ok := tx.(sdk.TxWithUnordered); ok && unordered.GetUnordered() {
			g.cascadable = false // unordered txs key by timeout, not sequence: seq here is meaningless for the gap rule
		}
		bz, _, err := EncodeTx(s.exec.encCache, s.exec.txEncoder, tx)
		if err != nil {
			g.cascadable = false
			continue
		}
		g.txs = append(g.txs, recheckCandidate{tx: tx, bz: bz, seq: seq, multiSigner: multiSigner})
	}
	for i := range groups {
		g := &groups[i]
		if _, ok := coSigned[g.key]; ok {
			g.cascadable = false
		}
		slices.SortStableFunc(g.txs, func(a, b recheckCandidate) int {
			return cmp.Compare(a.seq, b.seq)
		})
		// A duplicate seq can only appear as adjacent equal entries once sorted;
		// it still means the group isn't a clean ascending-nonce view.
		for j := 1; j < len(g.txs); j++ {
			if g.txs[j].seq <= g.txs[j-1].seq {
				g.cascadable = false
				break
			}
		}
	}
	return groups
}

// recheckChunkSize bounds how many candidates one signer group runs under a
// single hold of exec.mu. Without this, a group's size is bounded only by one
// sender's pool depth, and App.Commit — which blocks on the same mutex — would
// stall behind an arbitrarily deep queue.
const recheckChunkSize = 256

// nonceCursor is the account's next-expected-nonce view, carried across a
// group's chunks so cascade detection in a later chunk can still reason about
// a candidate accepted in an earlier one.
type nonceCursor struct {
	last uint64
	ok   bool
}

// runGroup re-validates one signer's candidates in bounded chunks, so a deep
// queue for one sender can't hold the admission mutex indefinitely. Nonce
// contiguity holds within and across chunks via the returned cursor; an
// admission of the same sender landing between chunks is the same residual
// interleaving the design doc already accepts between groups. unreachedFrom
// is -1 once every candidate has either run or been cascade-evicted; otherwise
// it is the index where the aborting chunk would have started, leaving the
// group untouched from there on. On a nonce gap the remaining higher-nonce
// siblings in the same chunk are evicted without spending a RunTx on each,
// since that eviction runs under the same lock hold as the gap proof. Each
// later chunk's own head is still verified with its own RunTx before any
// blind eviction there — the lock is released between chunks, so an admission
// of the same sender can legitimately fill the gap in the meantime. Any
// non-gap failure evicts only the failing tx, since a later sibling may still
// be the account's next expected nonce.
func (s *recheckScheduler) runGroup(g recheckGroup, gen uint64) (evicted, cascaded float32, unreachedFrom int) {
	cursor := nonceCursor{}
	gapFound := false
	for start := 0; start < len(g.txs); start += recheckChunkSize {
		end := min(start+recheckChunkSize, len(g.txs))
		var (
			e, c float32
			next nonceCursor
			gap  bool
			ok   bool
		)
		if gapFound {
			e, c, next, gap, ok = s.cascadeChunkLocked(g, start, end, gen)
		} else {
			e, c, next, gap, ok = s.recheckChunkLocked(g, start, end, gen, cursor)
		}
		evicted += e
		cascaded += c
		if !ok {
			return evicted, cascaded, start
		}
		cursor = next
		gapFound = gap
	}
	return evicted, cascaded, -1
}

// runCandidatesLocked runs g.txs[start:end] against the current base,
// evicting any candidate that fails and cascade-evicting the rest of the
// range once a nonce gap is proven. Precondition: caller holds exec.mu and
// has already confirmed gen is current.
func (s *recheckScheduler) runCandidatesLocked(g recheckGroup, start, end int, cursor nonceCursor) (evicted, cascaded float32, next nonceCursor, gapFound bool) {
	for i := start; i < end; i++ {
		c := g.txs[i]
		_, _, _, err := s.exec.runTxLocked(sdk.ExecModeReCheck, c.bz, c.tx)
		if err == nil {
			cursor = nonceCursor{last: c.seq, ok: true}
			continue
		}
		s.evict(c.tx, g.key, c.seq, g.known, c.multiSigner)
		evicted++
		// A gap is only provable relative to a nonce this pass just accepted;
		// without one the failure may be a stale nonce, whose successor is valid.
		if g.cascadable && cursor.ok && c.seq > cursor.last+1 && isNonceErr(err) {
			for _, rest := range g.txs[i+1 : end] {
				s.evict(rest.tx, g.key, rest.seq, g.known, rest.multiSigner)
				cascaded++
			}
			return evicted, cascaded, cursor, true
		}
	}
	return evicted, cascaded, cursor, false
}

// recheckChunkLocked runs g.txs[start:end] under one hold of exec.mu. Returns
// ok=false if gen advanced before the chunk started, meaning nothing in
// [start, len(g.txs)) ran. gapFound reports a proven nonce gap discovered in
// this chunk: the cascade for the rest of this chunk already ran here, under
// the same lock as the admissions it must stay atomic with respect to.
func (s *recheckScheduler) recheckChunkLocked(g recheckGroup, start, end int, gen uint64, cursor nonceCursor) (evicted, cascaded float32, next nonceCursor, gapFound, ok bool) {
	s.exec.mu.Lock()
	defer s.exec.mu.Unlock()
	// gen only advances under the same mutex, so it cannot change once this chunk starts.
	if s.exec.gen.Load() != gen {
		return 0, 0, cursor, false, false
	}
	evicted, cascaded, next, gapFound = s.runCandidatesLocked(g, start, end, cursor)
	return evicted, cascaded, next, gapFound, true
}

// cascadeChunkLocked handles a chunk that starts after a gap was proven in an
// earlier chunk. The gap proof only covers evictions made under that earlier
// chunk's own lock hold; the lock is released between chunks, so an admission
// of the same sender can land in the gap before this chunk's turn and
// legitimately fill it. This chunk's own head is therefore verified with a
// RunTx before anything is blind-evicted: if it succeeds, the gap didn't
// survive to this chunk, and the remainder falls back to normal
// recheckChunkLocked semantics, seeded from the head's now-accepted nonce. If
// it fails on a nonce error, the gap held, and the head plus the rest of the
// chunk are cascade-evicted without a RunTx on the rest, same as before. Any
// other failure (e.g. insufficient funds) carries no information about
// whether the gap survived — the EVM ante checks balance/gas before nonce, so
// a funds failure at the head says nothing about the account's true nonce
// state — so that case falls through to recheckChunkLocked's normal per-tx
// semantics for the rest of the chunk instead of assuming the gap held.
func (s *recheckScheduler) cascadeChunkLocked(g recheckGroup, start, end int, gen uint64) (evicted, cascaded float32, next nonceCursor, gapFound, ok bool) {
	s.exec.mu.Lock()
	defer s.exec.mu.Unlock()
	if s.exec.gen.Load() != gen {
		return 0, 0, nonceCursor{}, true, false
	}

	head := g.txs[start]
	_, _, _, err := s.exec.runTxLocked(sdk.ExecModeReCheck, head.bz, head.tx)
	if err == nil {
		evicted, cascaded, next, gapFound = s.runCandidatesLocked(g, start+1, end, nonceCursor{last: head.seq, ok: true})
		return evicted, cascaded, next, gapFound, true
	}

	s.evict(head.tx, g.key, head.seq, g.known, head.multiSigner)
	if !isNonceErr(err) {
		// No cursor context to carry over: we don't know the account's true
		// nonce state, so run the rest of the chunk one RunTx at a time instead
		// of assuming the gap held.
		evicted, cascaded, next, gapFound = s.runCandidatesLocked(g, start+1, end, nonceCursor{})
		return evicted + 1, cascaded, next, gapFound, true
	}

	for _, rest := range g.txs[start+1 : end] {
		s.evict(rest.tx, g.key, rest.seq, g.known, rest.multiSigner)
		cascaded++
	}
	return 1, cascaded, nonceCursor{}, true, true
}

// isNonceErr matches both ante paths: cosmos sig verification reports
// ErrWrongSequence, the EVM nonce check reports ErrInvalidSequence.
func isNonceErr(err error) bool {
	return errorsmod.IsOf(err, sdkerrors.ErrWrongSequence, sdkerrors.ErrInvalidSequence)
}

func unreachedTxs(groups []recheckGroup) []sdk.Tx {
	var txs []sdk.Tx
	for _, g := range groups {
		for _, c := range g.txs {
			txs = append(txs, c.tx)
		}
	}
	return txs
}

// recoverSenders folds txs' senders back into staged recheckSenders without
// touching deferred, which capRecheckGroups may have already set this cycle. A
// candidate whose signer can't be extracted is silently dropped here, so it
// waits for TTL eviction instead of being re-covered.
func (s *recheckScheduler) recoverSenders(txs []sdk.Tx) {
	if len(txs) == 0 {
		return
	}
	senders := make(map[string]struct{})
	for _, tx := range txs {
		for _, sg := range s.signers(tx) {
			senders[sg] = struct{}{}
		}
	}
	if len(senders) == 0 {
		return
	}
	s.stagingMu.Lock()
	s.mergeRecheckSenders(senders)
	s.stagingMu.Unlock()
}

// appendDeferred appends txs to the deferred carry under stagingMu. Append,
// not overwrite: capRecheckGroups may have already set deferred to its own
// overflow carry earlier this same cycle, and that must survive alongside an
// abort's unreached tail.
func (s *recheckScheduler) appendDeferred(txs []sdk.Tx) {
	if len(txs) == 0 {
		return
	}
	s.stagingMu.Lock()
	s.deferred = append(s.deferred, txs...)
	s.stagingMu.Unlock()
}

// txTimedout reports whether tx should be evicted by its own declared timeout:
func txTimedout(tx sdk.Tx, height int64, now time.Time) bool {
	if t, ok := tx.(sdk.TxWithTimeoutHeight); ok {
		th := t.GetTimeoutHeight()
		if th > 0 && uint64(height) >= th {
			return true
		}
	}
	if t, ok := tx.(sdk.TxWithTimeoutTimeStamp); ok {
		ts := t.GetTimeoutTimeStamp()
		if !ts.IsZero() && !now.Before(ts) {
			return true
		}
	}
	return false
}

// txTTLExpired reports whether tx has aged past ttlNumBlocks since first seen.
func txTTLExpired(arrival map[sdk.Tx]int64, tx sdk.Tx, height, ttlNumBlocks int64) (int64, bool) {
	arrived, ok := arrival[tx]
	if !ok {
		arrived = height
	}
	return arrived, height-arrived >= ttlNumBlocks
}

// evict removes tx from the pool and encoder cache together, so the cache never
// outlives its pool entry, then notifies evictionHook (if set) once per signer
// named by tx. A multi-signer tx caches App-level ante state per signer it
// names (e.g. ethermint stages one nonce-cache entry per msg), so the hook
// must fire once per named signer, not just the group's key signer — hence
// the extra GetSigners call here rather than reusing sender/nonce, but only
// on this eviction path, not the hot recheck pass.
func (s *recheckScheduler) evict(tx sdk.Tx, sender string, nonce uint64, known, multiSigner bool) {
	_ = s.mpool.Remove(tx)
	s.exec.encCache.Evict(tx)
	if s.evictionHook == nil {
		return
	}
	if !multiSigner {
		if known {
			s.evictionHook(sender, nonce)
		}
		return
	}
	for _, sg := range s.allSigners(tx) {
		s.evictionHook(sg.Signer.String(), sg.Sequence)
	}
}

// firstSigner returns the signer the mempool orders by, with its nonce, and
// whether tx has more than one signer. An unknown signer only costs the
// cascade optimization, not the recheck itself. A multi-signer tx must also
// disable the cascade: a secondary signer's nonce isn't visible to the group
// keyed on the first signer, so a gap in that group may really be filled by
// a multi-signer tx grouped elsewhere.
func (s *recheckScheduler) firstSigner(tx sdk.Tx) (key string, seq uint64, known, multiSigner bool) {
	sigs := s.allSigners(tx)
	if len(sigs) == 0 {
		return "", 0, false, false
	}
	return sigs[0].Signer.String(), sigs[0].Sequence, true, len(sigs) > 1
}

func (s *recheckScheduler) signers(tx sdk.Tx) []string {
	sigs := s.allSigners(tx)
	if len(sigs) == 0 {
		return nil
	}
	keys := make([]string, len(sigs))
	for i, sg := range sigs {
		keys[i] = sg.Signer.String()
	}
	return keys
}

// allSigners returns every signer GetSigners names for tx, nil-safe on both a
// nil signer extractor and a lookup error.
func (s *recheckScheduler) allSigners(tx sdk.Tx) []sdkmempool.SignerData {
	if s.signer == nil {
		return nil
	}
	sigs, err := s.signer.GetSigners(tx)
	if err != nil {
		return nil
	}
	return sigs
}

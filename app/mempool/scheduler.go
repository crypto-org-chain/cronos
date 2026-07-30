package mempool

import (
	"context"
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
	// maxRecheckBatch caps RunTx(ReCheck) calls per Commit cycle; 0 = unlimited.
	maxRecheckBatch int
	// stagingMu guards the staging fields (recheckSenders, deferred, lastCommittedHeight).
	// Separate from the admission mutex so FinalizeBlock staging never blocks behind a recheck batch.
	stagingMu sync.Mutex
	// recheckSenders accumulates senders of committed blocks awaiting recheck; merged
	// (not overwritten) across blocks so an un-drained block's senders aren't lost.
	recheckSenders map[string]struct{}
	// deferred carries candidates past maxRecheckBatch to the next cycle, so a
	// deep per-sender queue eventually drains instead of being silently dropped.
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
	gen := s.exec.gen.Load()
	// Before the first block (height 0) with no senders/carry there's nothing to scan.
	if len(recheckSenders) == 0 && len(deferred) == 0 && height == 0 {
		return
	}

	snapshot := PoolSnapshot(context.Background(), s.mpool)
	candidates := s.capRecheckTxs(s.selectTxs(snapshot, recheckSenders, height, deferred))
	s.runRecheck(candidates, gen)

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
	// priority-ordered, so otherwise capRecheckTxs re-takes the same prefix and starves the tail.
	ordered := make([]sdk.Tx, 0, len(deferred)+len(candidates))
	for _, tx := range deferred {
		if deferredLive[tx] {
			ordered = append(ordered, tx) // skip txs included/evicted since carry
		}
	}
	for _, tx := range candidates {
		if _, isDeferred := deferredLive[tx]; isDeferred {
			continue // sender re-touched this cycle; avoid double recheck
		}
		ordered = append(ordered, tx)
	}
	return ordered
}

// evictForRecheck evicts tx and folds its signers into recheckSenders, allocating
// evictedSet/recheckSenders lazily so a no-eviction cycle stays alloc-free.
func (s *recheckScheduler) evictForRecheck(tx sdk.Tx, evictedSet map[sdk.Tx]struct{}, recheckSenders map[string]struct{}) (map[sdk.Tx]struct{}, map[string]struct{}) {
	s.evict(tx)
	if evictedSet == nil {
		evictedSet = make(map[sdk.Tx]struct{})
	}
	evictedSet[tx] = struct{}{}
	sigs := s.signers(tx)
	if len(sigs) > 0 && recheckSenders == nil {
		recheckSenders = make(map[string]struct{})
	}
	for _, sg := range sigs {
		recheckSenders[sg] = struct{}{}
	}
	return evictedSet, recheckSenders
}

// capRecheckTxs bounds RunTx(ReCheck) per cycle; overflow carries forward.
func (s *recheckScheduler) capRecheckTxs(candidates []sdk.Tx) []sdk.Tx {
	if s.maxRecheckBatch <= 0 || len(candidates) <= s.maxRecheckBatch {
		return candidates
	}
	carried := make([]sdk.Tx, len(candidates)-s.maxRecheckBatch)
	copy(carried, candidates[s.maxRecheckBatch:])
	s.stagingMu.Lock()
	s.deferred = carried
	s.stagingMu.Unlock()
	return candidates[:s.maxRecheckBatch]
}

// recheckCandidate carries the signer nonce alongside the tx: telling a nonce
// gap from a merely stale nonce is what makes cascade eviction safe.
type recheckCandidate struct {
	tx  sdk.Tx
	bz  []byte
	seq uint64
}

// recheckGroup holds one signer's candidates in pool order. cascadable is false
// when the group is not that signer's contiguous ascending-nonce view — an
// unknown signer, a repeated or out-of-order nonce, or a tx dropped on encode
// error — because the cascade rule reasons about the next expected nonce.
type recheckGroup struct {
	txs        []recheckCandidate
	cascadable bool
}

// runRecheck re-validates candidates via RunTx(ReCheck), one signer group at a
// time so a sender's nonce chain advances atomically with respect to other
// senders' admissions. The pass is abandoned once gen advances mid-flight: the
// remaining candidates would be validated against a base a concurrent Commit
// has already superseded. drainStaging already cleared recheckSenders for this
// cycle, so the unreached candidates' senders are re-merged into staging here —
// otherwise a sender that isn't touched again by a later block would never be
// rechecked until TTL.
func (s *recheckScheduler) runRecheck(candidates []sdk.Tx, gen uint64) {
	var evicted, cascaded, superseded float32
	groups := s.groupCandidates(candidates)
	for i, g := range groups {
		if len(g.txs) == 0 {
			continue
		}
		e, c, aborted := s.recheckGroup(g, gen)
		evicted += e
		cascaded += c
		if aborted {
			unreached := unreachedTxs(groups[i:])
			superseded = float32(len(unreached))
			s.recoverSenders(unreached)
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
// orders by — keeping first-appearance order across groups and pool order
// within one, so the front-loaded deferred prefix still runs first. Encoding
// happens here, outside the admission mutex, to keep the per-group hold to RunTx.
func (s *recheckScheduler) groupCandidates(candidates []sdk.Tx) []recheckGroup {
	groups := make([]recheckGroup, 0, len(candidates))
	index := make(map[string]int, len(candidates))
	for _, tx := range candidates {
		key, seq, known := s.firstSigner(tx)
		gi, seen := index[key]
		if !seen {
			groups = append(groups, recheckGroup{cascadable: known})
			gi = len(groups) - 1
			index[key] = gi
		}
		g := &groups[gi]
		bz, _, err := EncodeTx(s.exec.encCache, s.exec.txEncoder, tx)
		if err != nil {
			g.cascadable = false
			continue
		}
		if n := len(g.txs); n > 0 && seq <= g.txs[n-1].seq {
			g.cascadable = false
		}
		g.txs = append(g.txs, recheckCandidate{tx: tx, bz: bz, seq: seq})
	}
	return groups
}

// recheckGroup re-validates one signer's candidates under a single hold of the
// admission mutex. Reports aborted when gen advanced before the group started,
// leaving the group untouched. On a nonce gap the remaining higher-nonce siblings
// are evicted without spending a RunTx on each: nothing can fill the gap while
// they sit in the pool. Any other failure evicts only the failing tx, since a
// later sibling may still be the account's next expected nonce.
func (s *recheckScheduler) recheckGroup(g recheckGroup, gen uint64) (evicted, cascaded float32, aborted bool) {
	s.exec.mu.Lock()
	defer s.exec.mu.Unlock()
	// gen only advances under the same mutex, so it cannot change once this group starts.
	if s.exec.gen.Load() != gen {
		return 0, 0, true
	}

	var lastOK uint64
	haveOK := false
	for i, c := range g.txs {
		_, _, _, err := s.exec.runTxLocked(sdk.ExecModeReCheck, c.bz, c.tx)
		if err == nil {
			lastOK, haveOK = c.seq, true
			continue
		}
		s.evict(c.tx)
		evicted++
		// A gap is only provable relative to a nonce this pass just accepted;
		// without one the failure may be a stale nonce, whose successor is valid.
		if g.cascadable && haveOK && c.seq > lastOK+1 && isNonceErr(err) {
			for _, rest := range g.txs[i+1:] {
				s.evict(rest.tx)
				cascaded++
			}
			return evicted, cascaded, false
		}
	}
	return evicted, cascaded, false
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
// touching deferred, which capRecheckTxs may have already set this cycle.
func (s *recheckScheduler) recoverSenders(txs []sdk.Tx) {
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
// outlives its pool entry.
func (s *recheckScheduler) evict(tx sdk.Tx) {
	_ = s.mpool.Remove(tx)
	s.exec.encCache.Evict(tx)
}

// firstSigner returns the signer the mempool orders by, with its nonce. An
// unknown signer only costs the cascade optimization, not the recheck itself.
func (s *recheckScheduler) firstSigner(tx sdk.Tx) (key string, seq uint64, known bool) {
	if s.signer == nil {
		return "", 0, false
	}
	sigs, err := s.signer.GetSigners(tx)
	if err != nil || len(sigs) == 0 {
		return "", 0, false
	}
	return sigs[0].Signer.String(), sigs[0].Sequence, true
}

func (s *recheckScheduler) signers(tx sdk.Tx) []string {
	sigs, err := s.signer.GetSigners(tx)
	if err != nil {
		return nil
	}
	keys := make([]string, len(sigs))
	for i, sg := range sigs {
		keys[i] = sg.Signer.String()
	}
	return keys
}

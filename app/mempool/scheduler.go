package mempool

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"

	antecache "github.com/evmos/ethermint/ante/cache"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

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
	// anteCache is ethermint's per-(sender, nonce) admission cache; evict drops
	// a tx's entries so a cascade/TTL eviction that never spends a RunTx cannot
	// leave a stale entry that lets a resubmit skip nonce verification.
	anteCache *antecache.AnteCache
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
	s.exec.pending.invalidate()

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

// RecheckTxs evicts pool txs invalidated by the last block. A Commit landing
// mid-pass is harmless: the remaining chunks simply run against the newer
// checkState, and each chunk re-proves any nonce gap under its own lock hold.
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
	s.runRecheck(s.capRecheckGroups(s.groupCandidates(candidates)))

	s.exec.pending.invalidate()
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
}

// recheckGroup holds one signer's candidates sorted ascending by seq.
// cascadable is false when the group is not that signer's contiguous
// ascending-nonce view — an unknown signer, a signer named by a multi-signer tx
// (that tx is grouped elsewhere, so it can fill a nonce this group can't see), a
// duplicate seq, an unordered tx (keyed by timeout, not sequence), or a tx
// dropped on encode error — because the cascade rule reasons about the next
// expected nonce.
type recheckGroup struct {
	key        string
	txs        []recheckCandidate
	cascadable bool
}

// runRecheck re-validates candidates via RunTx(ReCheck), one signer group at a
// time so a sender's nonce chain advances atomically with respect to other
// senders' admissions.
func (s *recheckScheduler) runRecheck(groups []recheckGroup) {
	var evicted, cascaded float32
	for _, g := range groups {
		e, c := s.runGroup(g)
		evicted += e
		cascaded += c
	}
	if evicted > 0 {
		telemetry.IncrCounter(evicted, "cronos", "mempool", "recheck", "evicted")
	}
	if cascaded > 0 {
		telemetry.IncrCounter(cascaded, "cronos", "mempool", "recheck", "cascade_evicted")
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
			groups = append(groups, recheckGroup{key: key, cascadable: known})
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
		g.txs = append(g.txs, recheckCandidate{tx: tx, bz: bz, seq: seq})
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

// runGroup re-validates one signer's candidates in bounded chunks, so a deep
// queue for one sender can't hold the admission mutex indefinitely. An
// admission of the same sender landing between chunks is the same residual
// interleaving the design doc already accepts between groups.
func (s *recheckScheduler) runGroup(g recheckGroup) (evicted, cascaded float32) {
	for start := 0; start < len(g.txs); start += recheckChunkSize {
		e, c := s.runChunkLocked(g, start, min(start+recheckChunkSize, len(g.txs)))
		evicted += e
		cascaded += c
	}
	return evicted, cascaded
}

// runChunkLocked runs g.txs[start:end] under one hold of exec.mu, evicting each
// candidate that fails. On a proven nonce gap the rest of the chunk is
// cascade-evicted without a RunTx each: siblings above a gap can't become
// valid until the gap is filled, and nothing can fill it while the mutex is
// held. A gap is only provable against a nonce this same lock hold accepted
// (so lastOK+1 is the account's next expected nonce), and only when the
// failing nonce is strictly above that. Nothing carries across chunks: once
// the mutex is released, a same-sender admission or a Commit can move the
// account's nonce, so a nonce error at the next chunk's head may mean stale
// rather than gap, and cascading on it would evict valid siblings. Without an
// accepted nonce a failure may also be a stale nonce, whose successor is valid.
func (s *recheckScheduler) runChunkLocked(g recheckGroup, start, end int) (evicted, cascaded float32) {
	s.exec.mu.Lock()
	defer s.exec.mu.Unlock()
	var lastOK uint64
	accepted := false
	for i := start; i < end; i++ {
		c := g.txs[i]
		_, _, _, err := s.exec.runTxLocked(sdk.ExecModeReCheck, c.bz, c.tx)
		if err == nil {
			lastOK, accepted = c.seq, true
			continue
		}
		s.evict(c.tx)
		evicted++
		if g.cascadable && accepted && c.seq > lastOK+1 && isNonceErr(err) {
			for _, rest := range g.txs[i+1 : end] {
				s.evict(rest.tx)
				cascaded++
			}
			return evicted, cascaded
		}
	}
	return evicted, cascaded
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

// evict removes tx from the pool, encoder cache, and ante cache together, so
// no cache outlives its pool entry.
func (s *recheckScheduler) evict(tx sdk.Tx) {
	_ = s.mpool.Remove(tx)
	s.exec.encCache.Evict(tx)
	s.evictAnteCache(tx)
}

// evictAnteCache mirrors the ante's own staging: ethermint caches one
// (from, nonce) entry per MsgEthereumTx, so a multi-msg tx drops one per msg.
func (s *recheckScheduler) evictAnteCache(tx sdk.Tx) {
	if s.anteCache == nil || tx == nil {
		return
	}
	for _, msg := range tx.GetMsgs() {
		ethTx, ok := msg.(*evmtypes.MsgEthereumTx)
		if !ok {
			continue
		}
		asTx := ethTx.AsTransaction()
		if asTx == nil {
			continue
		}
		s.anteCache.Delete(ethTx.GetFrom().String(), asTx.Nonce())
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

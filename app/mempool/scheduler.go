package mempool

import (
	"context"
	"sync"
	"time"

	antecache "github.com/evmos/ethermint/ante/cache"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// recheckScheduler is the recheck half of the app mempool: it stages the senders
// each block touched, picks which pending txs to re-validate, and evicts the ones
// the new state invalidated.
type recheckScheduler struct {
	exec      *txExec
	mpool     sdkmempool.Mempool
	signer    sdkmempool.SignerExtractionAdapter
	anteCache *antecache.AnteCache
	// maxRecheckBatch caps RunTx(ReCheck) calls per Commit cycle; 0 = unlimited.
	maxRecheckBatch int
	// stagingMu guards the staging fields (recheckSenders, deferred, lastCommittedHeight).
	// Separate from gate so FinalizeBlock staging never blocks behind its RunTx batches.
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

	recheckMu sync.Mutex // serializes RecheckTxs; always acquired before gate and stagingMu, never after
	// Zero-value (trigger nil) when built via the newManager() test constructor;
	// TriggerRecheck then runs RecheckTxs inline instead of async.
	worker recheckWorker
	// recheckDisabled mirrors mempool.recheck=false: skips all rechecking,
	// including TTL/expiry eviction
	recheckDisabled bool
}

// recheckDecodingEnabled reports whether sender decoding/bookkeeping should run.
func (a *recheckScheduler) recheckDecodingEnabled() bool {
	return !a.recheckDisabled && a.signer != nil && a.exec.decoder != nil
}

// stageSkippedSenders merges the senders of proposal-gate-rejected txs into
// recheckSenders without touching lastCommittedHeight
func (a *recheckScheduler) stageSkippedSenders(txs [][]byte) {
	if !a.recheckDecodingEnabled() || len(txs) == 0 {
		return
	}
	senders := make(map[string]struct{}, len(txs))
	for _, bz := range txs {
		tx, err := a.exec.decoder(bz)
		if err != nil {
			continue
		}
		for _, s := range a.signers(tx) {
			senders[s] = struct{}{}
		}
	}
	if len(senders) == 0 {
		return
	}
	a.stagingMu.Lock()
	a.mergeRecheckSenders(senders)
	a.stagingMu.Unlock()
}

func (a *recheckScheduler) mergeRecheckSenders(senders map[string]struct{}) {
	// mergeRecheckSenders folds senders into a.recheckSenders without overwriting, so a
	// block whose Commit skipped RecheckTxs doesn't lose its staged senders.
	if a.recheckSenders == nil {
		a.recheckSenders = senders
	} else {
		for s := range senders {
			a.recheckSenders[s] = struct{}{}
		}
	}
}

// stageRecheckSenders records committed-block senders for RecheckTxs and stages
// the committed height.
func (a *recheckScheduler) stageRecheckSenders(height int64, txs [][]byte) {
	a.exec.pending.invalidate()

	// Decode + extract signers unlocked (the expensive part), then publish height
	// and recheckSenders in one critical section so a reader never sees a torn update.
	var senders map[string]struct{}
	if a.recheckDecodingEnabled() {
		senders = make(map[string]struct{}, len(txs))
		for _, bz := range txs {
			tx, err := a.exec.decoder(bz)
			if err != nil {
				continue // non-sdk txs (e.g. vote extensions) have no mempool entry
			}
			for _, s := range a.signers(tx) {
				senders[s] = struct{}{}
			}
		}
	}

	a.stagingMu.Lock()
	a.lastCommittedHeight = height
	a.mergeRecheckSenders(senders)
	a.stagingMu.Unlock()
}

// triggerRecheck schedules an async recheck.
// Call only from the consensus path (App.Commit).
func (a *recheckScheduler) triggerRecheck() {
	if a.worker.trigger == nil {
		a.RecheckTxs()
		return
	}
	a.worker.recheck()
}

// RecheckTxs evicts pool txs invalidated by the last block.
func (a *recheckScheduler) RecheckTxs() {
	if a.mpool == nil || a.recheckDisabled {
		return
	}
	a.recheckMu.Lock() // lock order: see the recheckMu field comment
	defer a.recheckMu.Unlock()
	recheckSenders, height, deferred := a.drainStaging()
	// Before the first block (height 0) with no senders/carry there's nothing to scan.
	if len(recheckSenders) == 0 && len(deferred) == 0 && height == 0 {
		return
	}
	// A panic here would crash the node and, on the worker, skip close(ready).
	defer func() {
		if r := recover(); r != nil {
			a.restoreStaging(recheckSenders, deferred)
			a.exec.pending.invalidate() // evictions before the panic must reach pending queries
			logRecoveredPanic(a.exec.logger, panicPathRecheck, r)
		}
	}()

	snapshot := PoolSnapshot(context.Background(), a.mpool)
	candidates := a.capRecheckTxs(a.selectTxs(snapshot, recheckSenders, height, deferred))
	a.runRecheck(candidates)

	a.exec.pending.invalidate()
	telemetry.SetGauge(float32(a.mpool.CountTx()), "cronos", "mempool", "pool", "size")
}

// drainStaging atomically takes and clears the staged senders, height, and carry.
func (a *recheckScheduler) drainStaging() (recheckSenders map[string]struct{}, height int64, deferred []sdk.Tx) {
	a.stagingMu.Lock()
	defer a.stagingMu.Unlock()
	recheckSenders, height, deferred = a.recheckSenders, a.lastCommittedHeight, a.deferred
	a.recheckSenders = nil
	a.deferred = nil
	return recheckSenders, height, deferred
}

// restoreStaging puts drained work back after an aborted cycle so the next cycle retries it.
func (a *recheckScheduler) restoreStaging(recheckSenders map[string]struct{}, deferred []sdk.Tx) {
	a.stagingMu.Lock()
	defer a.stagingMu.Unlock()
	a.mergeRecheckSenders(recheckSenders)
	if a.deferred == nil {
		a.deferred = deferred
	}
}

// selectTxs scans the pool to retrieve txs for recheck. Caller (RecheckTxs)
// only invokes this when recheck is enabled.
func (a *recheckScheduler) selectTxs(snapshot []sdk.Tx, recheckSenders map[string]struct{}, height int64, deferred []sdk.Tx) []sdk.Tx {
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
	if a.ttlNumBlocks > 0 {
		newArrival = make(map[sdk.Tx]int64, len(snapshot))
	}

	// Pass 1: evictions. Collect senders of evicted txs so their remaining pool txs
	// (e.g. higher-nonce siblings) are rechecked — they become invalid after the gap.
	var evictedSet map[sdk.Tx]struct{} // nil until first eviction; nil-map read is safe
	now := time.Now()
	for _, tx := range snapshot {
		evictedPoison := a.guardRecheckTx(tx, func() {
			if txTimedout(tx, height, now) {
				evictedSet, recheckSenders = a.evictForRecheck(tx, evictedSet, recheckSenders)
				expiredEvicted++
				return
			}
			if a.ttlNumBlocks > 0 {
				arrived, expired := txTTLExpired(a.arrival, tx, height, a.ttlNumBlocks)
				if expired {
					evictedSet, recheckSenders = a.evictForRecheck(tx, evictedSet, recheckSenders)
					ttlEvicted++
					return
				}
				newArrival[tx] = arrived
			}
		})
		if evictedPoison {
			evictedSet = markEvicted(evictedSet, tx)
		}
	}
	a.arrival = newArrival
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
		var sigs []string
		if a.guardRecheckTx(tx, func() { sigs = a.signers(tx) }) {
			delete(deferredLive, tx) // evicted, so don't recheck it as carry
			continue
		}
		for _, s := range sigs {
			if _, ok := recheckSenders[s]; ok {
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
func (a *recheckScheduler) evictForRecheck(tx sdk.Tx, evictedSet map[sdk.Tx]struct{}, recheckSenders map[string]struct{}) (map[sdk.Tx]struct{}, map[string]struct{}) {
	a.evict(tx)
	evictedSet = markEvicted(evictedSet, tx)
	sigs := a.signers(tx)
	if len(sigs) > 0 && recheckSenders == nil {
		recheckSenders = make(map[string]struct{})
	}
	for _, s := range sigs {
		recheckSenders[s] = struct{}{}
	}
	return evictedSet, recheckSenders
}

func markEvicted(evictedSet map[sdk.Tx]struct{}, tx sdk.Tx) map[sdk.Tx]struct{} {
	if evictedSet == nil {
		evictedSet = make(map[sdk.Tx]struct{})
	}
	evictedSet[tx] = struct{}{}
	return evictedSet
}

// guardRecheckTx runs step and evicts tx if step panics, so one poison tx can't stall
// every cycle's eviction. Reports whether step panicked.
func (a *recheckScheduler) guardRecheckTx(tx sdk.Tx, step func()) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			logRecoveredPanic(a.exec.logger, panicPathRecheckTx, r)
			a.evictPoisonTx(tx)
			panicked = true
		}
	}()
	step()
	return false
}

// Eviction calls the same tx methods that may have panicked; if it panics too, the tx
// stays pooled and is guarded again next cycle.
func (a *recheckScheduler) evictPoisonTx(tx sdk.Tx) {
	defer func() {
		if r := recover(); r != nil {
			logRecoveredPanic(a.exec.logger, panicPathRecheckTx, r, "evict_failed", true)
		}
	}()
	a.evict(tx)
}

// capRecheckTxs bounds RunTx(ReCheck) per cycle; overflow carries forward.
func (a *recheckScheduler) capRecheckTxs(candidates []sdk.Tx) []sdk.Tx {
	if a.maxRecheckBatch <= 0 || len(candidates) <= a.maxRecheckBatch {
		return candidates
	}
	carried := make([]sdk.Tx, len(candidates)-a.maxRecheckBatch)
	copy(carried, candidates[a.maxRecheckBatch:])
	a.stagingMu.Lock()
	a.deferred = carried
	a.stagingMu.Unlock()
	return candidates[:a.maxRecheckBatch]
}

// runRecheck re-validates candidates via RunTx(ReCheck)
func (a *recheckScheduler) runRecheck(candidates []sdk.Tx) {
	var evicted float32
	for _, tx := range candidates {
		a.guardRecheckTx(tx, func() {
			bz, _, err := EncodeTx(a.exec.encCache, a.exec.txEncoder, tx)
			if err != nil {
				return
			}
			if err = a.recheckTx(bz, tx); err != nil {
				a.evict(tx)
				evicted++
			}
		})
	}
	if evicted > 0 {
		telemetry.IncrCounter(evicted, "cronos", "mempool", "recheck", "evicted")
	}
}

// Deferred unlock: a recovered RunTx panic must not leave the gate held.
func (a *recheckScheduler) recheckTx(bz []byte, tx sdk.Tx) error {
	a.exec.gate.lock()
	defer a.exec.gate.unlock()
	_, _, _, err := a.exec.runTxLocked(sdk.ExecModeReCheck, bz, tx)
	return err
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
func (a *recheckScheduler) evict(tx sdk.Tx) {
	_ = a.mpool.Remove(tx)
	a.exec.encCache.Evict(tx)
	a.evictAnteCache(tx)
}

func (a *recheckScheduler) evictAnteCache(tx sdk.Tx) {
	if a.anteCache == nil || tx == nil {
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
		a.anteCache.Delete(ethTx.GetFrom().String(), asTx.Nonce())
	}
}

func (a *recheckScheduler) signers(tx sdk.Tx) []string {
	sigs, err := a.signer.GetSigners(tx)
	if err != nil {
		return nil
	}
	keys := make([]string, len(sigs))
	for i, s := range sigs {
		keys[i] = s.Signer.String()
	}
	return keys
}

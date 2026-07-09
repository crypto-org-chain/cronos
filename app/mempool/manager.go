package mempool

import (
	"context"
	"sync"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"

	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/baseapp"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

type txRunner interface {
	RunTx(mode sdk.ExecMode, txBytes []byte, tx sdk.Tx, txIndex int, txMultiStore storetypes.MultiStore, incarnationCache map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error)
}

var _ txRunner = (*baseapp.BaseApp)(nil)

// Manager owns the app-side mempool for mempool.type=app: tx admission
// (Insert/CheckTx) plus per-block recheck and TTL eviction.
type Manager struct {
	// mu guards BaseApp.checkState: every RunTx (admission Check, RPC CheckTx,
	// the ReCheck batch) plus Commit's checkState reset (via AdmissionMutex).
	// AppMempool.Lock() is a no-op, so mu replaces the mempool lock BaseApp
	// normally relies on. Held only around RunTx, never the lock-free pool scan.
	mu        sync.Mutex
	runner    txRunner
	encCache  *EncoderCache
	txEncoder sdk.TxEncoder
	trace     bool
	// preVerify runs cheap verification lock-free before the tx admission mutex; set to nil for skip.
	preVerify func([]byte) error

	mpool   sdkmempool.Mempool
	signer  sdkmempool.SignerExtractionAdapter
	decoder sdk.TxDecoder
	// maxRecheckBatch caps RunTx(ReCheck) calls per Commit cycle; 0 = unlimited.
	maxRecheckBatch int
	// stagingMu guards the staging fields (recheckSenders, deferred, lastCommittedHeight).
	// Separate from mu so FinalizeBlock staging never blocks behind mu's RunTx batches.
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

	recheckMu sync.Mutex // serializes RecheckTxs; always acquired before mu and stagingMu, never after
	// Zero-value (trigger nil) when built via the newManager() test constructor;
	// TriggerRecheck then runs RecheckTxs inline instead of async.
	worker recheckWorker
}

// NewManager builds the Manager for mempool.type=app; register it via
// BaseApp.SetInsertTxHandler before Seal.
func NewManager(app *baseapp.BaseApp, encCache *EncoderCache, txEncoder sdk.TxEncoder, mpool sdkmempool.Mempool, signer sdkmempool.SignerExtractionAdapter, decoder sdk.TxDecoder, recheckBatchSize int, ttlNumBlocks int64) *Manager {
	a := newManager(app, encCache, txEncoder, decoder)
	a.trace = app.Trace()
	a.mpool = mpool
	a.signer = signer
	a.maxRecheckBatch = recheckBatchSize
	a.ttlNumBlocks = ttlNumBlocks
	// newManager (tests) leaves worker zero and runs RecheckTxs synchronously.
	a.worker = newRecheckWorker(a.RecheckTxs)
	a.worker.start()
	return a
}

func newManager(runner txRunner, encCache *EncoderCache, txEncoder sdk.TxEncoder, decoder sdk.TxDecoder) *Manager {
	if encCache != nil {
		if decoder == nil {
			panic("mempool: encCache requires decoder != nil")
		}
		if txEncoder == nil {
			panic("mempool: encCache requires txEncoder != nil for canonical bytes")
		}
	}
	return &Manager{
		runner:    runner,
		encCache:  encCache,
		txEncoder: txEncoder,
		decoder:   decoder,
	}
}

// StageSkippedSenders merges the senders of proposal-gate-rejected txs into
// recheckSenders without touching lastCommittedHeight (which StageRecheckSenders owns).
// Called from the PrepareProposal wrapper so stranded senders are re-validated at the
// next RecheckTxs cycle (~1 block) instead of waiting for the TTL.
func (a *Manager) StageSkippedSenders(txs [][]byte) {
	if a.signer == nil || a.decoder == nil || len(txs) == 0 {
		return
	}
	senders := make(map[string]struct{}, len(txs))
	for _, bz := range txs {
		tx, err := a.decoder(bz)
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

// mergeRecheckSenders folds senders into a.recheckSenders without overwriting, so a
// block whose Commit skipped RecheckTxs doesn't lose its staged senders. Caller holds stagingMu.
func (a *Manager) mergeRecheckSenders(senders map[string]struct{}) {
	if a.recheckSenders == nil {
		a.recheckSenders = senders
	} else {
		for s := range senders {
			a.recheckSenders[s] = struct{}{}
		}
	}
}

// AdmissionMutex exposes mu so App.Commit can serialize its checkState reset
// against lock-free admission. The pointer is stable (Manager is heap-allocated).
func (a *Manager) AdmissionMutex() *sync.Mutex {
	return &a.mu
}

// SetPreVerify wires the lock-free pre-verification hook for mempool.type=app.
func (a *Manager) SetPreVerify(fn func([]byte) error) {
	a.preVerify = fn
}

// InsertTxHandler validates peer-relayed txs via RunTx(ExecModeCheck) before
// admitting them. Admitted txs register canonical bytes so ReapTxsHandler can
// skip proto.Marshal; re-encoding keeps non-minimal peer bytes out of the cache.
func (a *Manager) InsertTxHandler() sdk.InsertTxHandler {
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		if a.preVerify != nil {
			if err := a.preVerify(req.Tx); err != nil {
				_, code, _ := errorsmod.ABCIInfo(err, false)
				return &abci.ResponseInsertTx{Code: code}, nil
			}
		}
		var tx sdk.Tx
		if a.encCache != nil {
			var err error
			if tx, err = a.decoder(req.Tx); err != nil {
				_, code, _ := errorsmod.ABCIInfo(sdkerrors.ErrTxDecode.Wrap(err.Error()), false)
				return &abci.ResponseInsertTx{Code: code}, nil
			}
		}

		a.mu.Lock()
		defer a.mu.Unlock()

		_, _, _, err := a.runner.RunTx(sdk.ExecModeCheck, req.Tx, tx, -1, nil, nil)
		if err != nil {
			if errorsmod.IsOf(err, sdkmempool.ErrMempoolTxMaxCapacity) {
				return &abci.ResponseInsertTx{Code: abci.CodeTypeRetry}, nil
			}
			_, code, _ := errorsmod.ABCIInfo(err, false)
			return &abci.ResponseInsertTx{Code: code}, nil
		}

		a.cacheTx(tx, req.Tx)
		return &abci.ResponseInsertTx{Code: abci.CodeTypeOK}, nil
	}
}

// cacheTx registers the already-decoded tx under its canonical bytes (raw
// req.Tx bytes on encode error). No-op without a cache.
func (a *Manager) cacheTx(tx sdk.Tx, raw []byte) {
	if a.encCache == nil {
		return
	}
	bz := raw
	if canonical, err := a.txEncoder(tx); err == nil {
		bz = canonical
	}
	a.encCache.Set(tx, bz)
}

// CheckTxHandler runs RPC CheckTx.The runTx closure comes from BaseApp bound to
// the exec mode; its panics are recovered inside BaseApp.RunTx.
func (a *Manager) CheckTxHandler() sdk.CheckTxHandler {
	return func(runTx sdk.RunTx, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
		// Decode before locking: proto unmarshal is CPU-intensive; decoder and
		// DecodeCache have their own locks. Bad txs return without acquiring mu.
		var tx sdk.Tx
		if a.encCache != nil {
			var err error
			if tx, err = a.decoder(req.Tx); err != nil {
				return sdkerrors.ResponseCheckTxWithEvents(sdkerrors.ErrTxDecode.Wrap(err.Error()), 0, 0, nil, a.trace), nil
			}
		}

		a.mu.Lock()
		defer a.mu.Unlock()

		gasInfo, result, anteEvents, err := runTx(req.Tx, tx)
		if err != nil {
			return sdkerrors.ResponseCheckTxWithEvents(err, gasInfo.GasWanted, gasInfo.GasUsed, anteEvents, a.trace), nil
		}

		a.cacheTx(tx, req.Tx)

		// No MarkEventsToIndex (unlike default CheckTx): that flag only feeds
		// the tx indexer on FinalizeBlock results, not CheckTx.
		return &abci.ResponseCheckTx{
			GasWanted: int64(gasInfo.GasWanted),
			GasUsed:   int64(gasInfo.GasUsed),
			Log:       result.Log,
			Data:      result.Data,
			Events:    result.Events,
		}, nil
	}
}

// StageRecheckSenders records the senders of the just-committed block's txs so
// RecheckTxs can re-validate only their remaining pending txs, and stages the
// committed height for TimeoutHeight eviction.
func (a *Manager) StageRecheckSenders(height int64, txs [][]byte) {
	// Decode + extract signers unlocked (the expensive part), then publish height
	// and recheckSenders in one critical section so a reader never sees a torn update.
	var senders map[string]struct{}
	if a.signer != nil && a.decoder != nil {
		senders = make(map[string]struct{}, len(txs))
		for _, bz := range txs {
			tx, err := a.decoder(bz)
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

// TriggerRecheck schedules an async recheck without blocking; repeated calls
// coalesce into one run. Falls back to running RecheckTxs inline when there's
// no worker (the test path).
func (a *Manager) TriggerRecheck() {
	if a.worker.trigger == nil {
		a.RecheckTxs()
		return
	}
	a.worker.recheck()
}

// Close stops the recheck worker. Idempotent; call before store teardown.
func (a *Manager) Close() {
	a.worker.stop()
}

// WaitForRecheck blocks until any in-progress recheck completes. ctx cancellation
// unblocks it so a stuck worker cannot stall block production.
func (a *Manager) WaitForRecheck(ctx context.Context) {
	if a.worker.trigger == nil {
		return // sync path
	}
	a.worker.wait(ctx)
}

// WaitForRecheckTimedOut works like WaitForRecheck but bounded by timeout,
// and reports whether the timeout was hit.
func (a *Manager) WaitForRecheckTimedOut(ctx context.Context, timeout time.Duration) bool {
	if a.worker.trigger == nil {
		return false // sync path always completes inline
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return a.worker.wait(waitCtx)
}

// RecheckTxs evicts pool txs invalidated by the last block: timed-out txs (any
// sender) and txs of block-touched senders that now fail ReCheck. Capped per
// cycle; overflow carries to the next.
func (a *Manager) RecheckTxs() {
	if a.mpool == nil {
		return
	}
	a.recheckMu.Lock() // only one recheck runs at a time; see the recheckMu field comment for lock ordering
	defer a.recheckMu.Unlock()
	recheckSenders, height, deferred := a.drainStaging()
	// Before the first block (height 0) with no senders/carry there's nothing to scan.
	if len(recheckSenders) == 0 && len(deferred) == 0 && height == 0 {
		return
	}

	snapshot := PoolSnapshot(context.Background(), a.mpool)
	candidates := a.selectTxs(snapshot, recheckSenders, height, deferred)
	candidates = a.capRecheckTxs(candidates)
	a.runRecheck(candidates)
	telemetry.SetGauge(float32(a.mpool.CountTx()), "cronos", "mempool", "pool", "size")
}

// drainStaging atomically takes and clears the staged senders, height, and carry.
func (a *Manager) drainStaging() (recheckSenders map[string]struct{}, height int64, deferred []sdk.Tx) {
	a.stagingMu.Lock()
	defer a.stagingMu.Unlock()
	recheckSenders, height, deferred = a.recheckSenders, a.lastCommittedHeight, a.deferred
	a.recheckSenders = nil
	a.deferred = nil
	return recheckSenders, height, deferred
}

// selectTxs scans the pool to retrieve txs for recheck.
func (a *Manager) selectTxs(snapshot []sdk.Tx, recheckSenders map[string]struct{}, height int64, deferred []sdk.Tx) []sdk.Tx {
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
		if txTimedout(tx, height, now) {
			evictedSet, recheckSenders = a.evictForRecheck(tx, evictedSet, recheckSenders)
			expiredEvicted++
			continue
		}
		if a.ttlNumBlocks > 0 {
			arrived, expired := txTTLExpired(a.arrival, tx, height, a.ttlNumBlocks)
			if expired {
				evictedSet, recheckSenders = a.evictForRecheck(tx, evictedSet, recheckSenders)
				ttlEvicted++
				continue
			}
			newArrival[tx] = arrived
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
		for _, s := range a.signers(tx) {
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
func (a *Manager) evictForRecheck(tx sdk.Tx, evictedSet map[sdk.Tx]struct{}, recheckSenders map[string]struct{}) (map[sdk.Tx]struct{}, map[string]struct{}) {
	a.evict(tx)
	if evictedSet == nil {
		evictedSet = make(map[sdk.Tx]struct{})
	}
	evictedSet[tx] = struct{}{}
	sigs := a.signers(tx)
	if len(sigs) > 0 && recheckSenders == nil {
		recheckSenders = make(map[string]struct{})
	}
	for _, s := range sigs {
		recheckSenders[s] = struct{}{}
	}
	return evictedSet, recheckSenders
}

// capRecheckTxs bounds RunTx(ReCheck) per cycle; overflow carries forward.
func (a *Manager) capRecheckTxs(candidates []sdk.Tx) []sdk.Tx {
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

// runRecheck re-validates candidates via RunTx(ReCheck), evicting failures from both
// the pool and encoder cache. mu is locked per tx, not across the batch, so admission
// interleaves; EncodeTx/evict need no lock (mpool and encCache are self-synced).
func (a *Manager) runRecheck(candidates []sdk.Tx) {
	var evicted float32
	for _, tx := range candidates {
		bz, _, err := EncodeTx(a.encCache, a.txEncoder, tx)
		if err != nil {
			continue
		}
		a.mu.Lock()
		_, _, _, err = a.runner.RunTx(sdk.ExecModeReCheck, bz, tx, -1, nil, nil)
		a.mu.Unlock()
		if err != nil {
			a.evict(tx)
			evicted++
		}
	}
	if evicted > 0 {
		telemetry.IncrCounter(evicted, "cronos", "mempool", "recheck", "evicted")
	}
}

// txTimedout reports whether tx should be evicted by its own declared timeout:
// TxWithTimeoutHeight passed or TxWithTimeoutTimeStamp reached.
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
// Returns arrived (for the caller's newArrival map) and whether the tx is expired.
func txTTLExpired(arrival map[sdk.Tx]int64, tx sdk.Tx, height, ttlNumBlocks int64) (int64, bool) {
	arrived, ok := arrival[tx]
	if !ok {
		arrived = height
	}
	return arrived, height-arrived >= ttlNumBlocks
}

// evict removes tx from the pool and encoder cache together, so the cache never
// outlives its pool entry.
func (a *Manager) evict(tx sdk.Tx) {
	_ = a.mpool.Remove(tx)
	a.encCache.Evict(tx)
}

func (a *Manager) signers(tx sdk.Tx) []string {
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

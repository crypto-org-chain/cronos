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

// Admitter owns the app-side mempool admission path for mempool.type=app.
type Admitter struct {
	// mu guards BaseApp.checkState: every RunTx (admission Check, RPC CheckTx,
	// the ReCheck batch) plus Commit's checkState reset (via AdmissionMutex).
	// AppMempool.Lock() is a no-op, so mu replaces the mempool lock BaseApp
	// normally relies on. Held only around RunTx, never the lock-free pool scan.
	mu        sync.Mutex
	runner    txRunner
	encCache  *EncoderCache
	txEncoder sdk.TxEncoder
	trace     bool

	mpool   sdkmempool.Mempool
	signer  sdkmempool.SignerExtractionAdapter
	decoder sdk.TxDecoder
	// maxRecheckBatch caps RunTx(ReCheck) calls per Commit cycle; 0 = unlimited.
	maxRecheckBatch int
	// pendingMu guards the staging fields (pending, deferred, lastCommittedHeight).
	// Separate from mu so FinalizeBlock staging never blocks behind mu's RunTx batches.
	pendingMu sync.Mutex
	// pending accumulates senders of committed blocks awaiting recheck; merged
	// (not overwritten) across blocks so an un-drained block's senders aren't lost.
	pending map[string]struct{}
	// deferred carries candidates past maxRecheckBatch to the next cycle, so a
	// deep per-sender queue eventually drains instead of being silently dropped.
	deferred            []sdk.Tx
	lastCommittedHeight int64
	// arrival maps each pooled tx to the height RecheckTxs first observed it, for
	// ttlNumBlocks eviction. Rebuilt from the snapshot each cycle (stale entries
	// drop out) and touched only by RecheckTxs (serial per Commit), so it needs no lock.
	arrival map[sdk.Tx]int64
	// ttlNumBlocks evicts txs older than this many blocks by arrival height; 0 = off.
	ttlNumBlocks int64
}

// NewAdmitter builds the Admitter for mempool.type=app; register it via
// BaseApp.SetInsertTxHandler before Seal.
func NewAdmitter(app *baseapp.BaseApp, encCache *EncoderCache, txEncoder sdk.TxEncoder, mpool sdkmempool.Mempool, signer sdkmempool.SignerExtractionAdapter, decoder sdk.TxDecoder, recheckBatchSize int, ttlNumBlocks int64) *Admitter {
	a := newAdmitter(app, encCache, txEncoder, decoder)
	a.trace = app.Trace()
	a.mpool = mpool
	a.signer = signer
	a.maxRecheckBatch = recheckBatchSize
	a.ttlNumBlocks = ttlNumBlocks
	return a
}

func newAdmitter(runner txRunner, encCache *EncoderCache, txEncoder sdk.TxEncoder, decoder sdk.TxDecoder) *Admitter {
	if encCache != nil {
		if decoder == nil {
			panic("mempool: encCache requires decoder != nil")
		}
		if txEncoder == nil {
			panic("mempool: encCache requires txEncoder != nil for canonical bytes")
		}
	}
	return &Admitter{
		runner:    runner,
		encCache:  encCache,
		txEncoder: txEncoder,
		decoder:   decoder,
	}
}

// AdmissionMutex exposes mu so App.Commit can serialize its checkState reset
// against lock-free admission. The pointer is stable (Admitter is heap-allocated).
func (a *Admitter) AdmissionMutex() *sync.Mutex {
	return &a.mu
}

// InsertTxHandler validates peer-relayed txs via RunTx(ExecModeCheck) before
// admitting them. Flood protection relies on CometBFT peer limits, not this
// handler. Admitted txs register canonical bytes so ReapTxsHandler can skip
// proto.Marshal; re-encoding keeps non-minimal peer bytes out of the cache.
func (a *Admitter) InsertTxHandler() sdk.InsertTxHandler {
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		a.mu.Lock()
		defer a.mu.Unlock()

		_, _, _, err := a.runner.RunTx(sdk.ExecModeCheck, req.Tx, nil, -1, nil, nil)
		if err != nil {
			if errorsmod.IsOf(err, sdkmempool.ErrMempoolTxMaxCapacity) {
				return &abci.ResponseInsertTx{Code: abci.CodeTypeRetry}, nil
			}
			_, code, _ := errorsmod.ABCIInfo(err, false)
			return &abci.ResponseInsertTx{Code: code}, nil
		}

		a.cacheTx(req.Tx)
		return &abci.ResponseInsertTx{Code: abci.CodeTypeOK}, nil
	}
}

// cacheTx registers tx in the encoder cache.
func (a *Admitter) cacheTx(raw []byte) {
	if a.encCache == nil {
		return
	}
	tx, err := a.decoder(raw)
	if err != nil {
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
func (a *Admitter) CheckTxHandler() sdk.CheckTxHandler {
	return func(runTx sdk.RunTx, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
		a.mu.Lock()
		defer a.mu.Unlock()

		gasInfo, result, anteEvents, err := runTx(req.Tx, nil)
		if err != nil {
			return sdkerrors.ResponseCheckTxWithEvents(err, gasInfo.GasWanted, gasInfo.GasUsed, anteEvents, a.trace), nil
		}

		a.cacheTx(req.Tx)

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
func (a *Admitter) StageRecheckSenders(height int64, txs [][]byte) {
	// Decode + extract signers unlocked (the expensive part), then publish height
	// and pending in one critical section so a reader never sees a torn update.
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

	a.pendingMu.Lock()
	a.lastCommittedHeight = height
	if a.pending == nil {
		a.pending = senders
	} else {
		// Merge, don't overwrite: a prior block whose Commit skipped RecheckTxs
		// (e.g. Commit error) still has senders staged here that must not be lost.
		for s := range senders {
			a.pending[s] = struct{}{}
		}
	}
	a.pendingMu.Unlock()
}

// RecheckTxs evicts pool txs invalidated by the last block: those whose
// TimeoutHeight has passed (any sender), and those of senders touched by the
// block that now fail the AnteHandler in ReCheck mode. RunTx(ReCheck) work is
// capped per cycle; overflow is carried (front-loaded) to the next cycle rather
// than dropped, so a deep per-sender queue still drains over time.
func (a *Admitter) RecheckTxs() {
	if a.mpool == nil {
		return
	}
	pending, height, deferred := a.drainStaging()
	// Nothing to do before the first committed block (height 0) with no pending
	// senders and no carryover. In steady state height > 0, so the sweep always scans.
	if len(pending) == 0 && len(deferred) == 0 && height == 0 {
		return
	}

	snapshot := PoolSnapshot(context.Background(), a.mpool)
	candidates := a.selectCandidates(snapshot, pending, height, deferred)
	candidates = a.capBatch(candidates)
	a.runRecheck(candidates)
}

// drainStaging atomically takes and clears the staged senders, committed height,
// and the prior cycle's carried-over candidates.
func (a *Admitter) drainStaging() (pending map[string]struct{}, height int64, deferred []sdk.Tx) {
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()
	pending, height, deferred = a.pending, a.lastCommittedHeight, a.deferred
	a.pending = nil
	a.deferred = nil
	return
}

// selectCandidates scans the snapshot once: evicting txs past their timeout or
// TTL, rebuilding the arrival map, and collecting txs whose senders the last
// block touched. Carried-over (deferred) candidates still in the pool are
// front-loaded ahead of fresh ones: the snapshot is priority-ordered, so without
// this the per-cycle cap would re-take the same prefix every cycle and the tail
// would starve.
func (a *Admitter) selectCandidates(snapshot []sdk.Tx, pending map[string]struct{}, height int64, deferred []sdk.Tx) []sdk.Tx {
	// deferredLive maps each carried-over tx to whether it's still in the pool.
	// Sized to the (small) carryover, not the whole snapshot; nil with no carryover.
	var deferredLive map[sdk.Tx]bool
	if len(deferred) > 0 {
		deferredLive = make(map[sdk.Tx]bool, len(deferred))
		for _, tx := range deferred {
			deferredLive[tx] = false
		}
	}

	var (
		candidates     []sdk.Tx
		expiredEvicted float32
		ttlEvicted     float32
	)
	// Rebuild arrival from this cycle's snapshot so txs gone from the pool fall out.
	var newArrival map[sdk.Tx]int64
	if a.ttlNumBlocks > 0 {
		newArrival = make(map[sdk.Tx]int64, len(snapshot))
	}
	now := time.Now()
	for _, tx := range snapshot {
		if txTimedout(tx, height, now) {
			a.evict(tx)
			expiredEvicted++
			continue
		}
		if a.ttlNumBlocks > 0 {
			arrived, expired := txTTLExpired(a.arrival, tx, height, a.ttlNumBlocks)
			if expired {
				a.evict(tx)
				ttlEvicted++
				continue
			}
			newArrival[tx] = arrived
		}
		if deferredLive != nil {
			if _, isDeferred := deferredLive[tx]; isDeferred {
				deferredLive[tx] = true
			}
		}
		if len(pending) == 0 {
			continue
		}
		for _, s := range a.signers(tx) {
			if _, ok := pending[s]; ok {
				candidates = append(candidates, tx)
				break
			}
		}
	}
	a.arrival = newArrival
	if expiredEvicted > 0 {
		telemetry.IncrCounter(expiredEvicted, "cronos", "mempool", "recheck", "expired")
	}
	if ttlEvicted > 0 {
		telemetry.IncrCounter(ttlEvicted, "cronos", "mempool", "recheck", "ttl_expired")
	}

	if len(deferred) == 0 {
		return candidates
	}
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

// capBatch bounds RunTx(ReCheck) per cycle, carrying the overflow to the next
// cycle (front-loaded there) rather than dropping it.
func (a *Admitter) capBatch(candidates []sdk.Tx) []sdk.Tx {
	if a.maxRecheckBatch <= 0 || len(candidates) <= a.maxRecheckBatch {
		return candidates
	}
	carried := make([]sdk.Tx, len(candidates)-a.maxRecheckBatch)
	copy(carried, candidates[a.maxRecheckBatch:])
	a.pendingMu.Lock()
	a.deferred = carried
	a.pendingMu.Unlock()
	return candidates[:a.maxRecheckBatch]
}

// runRecheck re-validates candidates via RunTx(ReCheck), evicting those that now
// fail the AnteHandler. RunTx mutates checkState, so it serializes against
// admission and the post-Commit reset for the batch only.
func (a *Admitter) runRecheck(candidates []sdk.Tx) {
	if len(candidates) == 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	var evicted float32
	for _, tx := range candidates {
		bz, _, err := EncodeTx(a.encCache, a.txEncoder, tx)
		if err != nil {
			continue
		}
		if _, _, _, err := a.runner.RunTx(sdk.ExecModeReCheck, bz, tx, -1, nil, nil); err != nil {
			a.encCache.Evict(tx)
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
func (a *Admitter) evict(tx sdk.Tx) {
	_ = a.mpool.Remove(tx)
	a.encCache.Evict(tx)
}

func (a *Admitter) signers(tx sdk.Tx) []string {
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

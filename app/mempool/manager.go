package mempool

import (
	"context"
	"sync"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/baseapp"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// evictBatchSize bounds how many expired txs one admission-lock hold evicts.
const evictBatchSize = 1024

type txRunner interface {
	RunTx(mode sdk.ExecMode, txBytes []byte, tx sdk.Tx, txIndex int, txMultiStore storetypes.MultiStore, incarnationCache map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error)
}

var _ txRunner = (*baseapp.BaseApp)(nil)

const (
	mempoolFullLog    = "mempool is full"
	rpcTxLimitFullLog = "too many in-flight RPC txs"
)

// admitSource decides whether an admission counts against rpcTxLimit.
type admitSource int

const (
	fromGossip admitSource = iota
	fromRPC
)

// Manager owns the app-side mempool for mempool.type=app
type Manager struct {
	// gate guards BaseApp.checkState, replacing the mempool lock BaseApp relies on
	// (AppMempool.Lock() is a no-op). Held around RunTx and evict, never the lock-free pool scan.
	// Taken before the pool lock, never under it.
	gate admissionGate
	// rpcTxLimit is applied to RPC admissions only; gossip is already bounded by
	// p2p receive concurrency.
	rpcTxLimit inflightLimit

	runner    txRunner
	encCache  *EncoderCache
	txEncoder sdk.TxEncoder
	// encodeMu serializes the pre-RunTx encode, which writes empty body/auth-info bytes into a
	// shared decoded tx. Pooled txs passed ante, so other encodes only read. Never held with gate.
	encodeMu sync.Mutex
	trace    bool
	logger   log.Logger
	// preVerify runs cheap verification lock-free before the tx admission mutex; set to nil for skip.
	preVerify func([]byte) error

	mpool   sdkmempool.Mempool
	signer  sdkmempool.SignerExtractionAdapter
	decoder sdk.TxDecoder
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
	// pendingTxCache avoids re-walking the pool on every PendingTxs() call.
	pendingTxCache pendingTxCache
}

// NewManager builds the Manager for mempool.type=app.
func NewManager(app *baseapp.BaseApp, logger log.Logger, encCache *EncoderCache, txEncoder sdk.TxEncoder, mpool sdkmempool.Mempool, signer sdkmempool.SignerExtractionAdapter, decoder sdk.TxDecoder, recheckBatchSize int, ttlNumBlocks int64, recheckDisabled, pendingCacheEnabled bool) *Manager {
	a := newManager(app, encCache, txEncoder, decoder)
	a.trace = app.Trace()
	if logger != nil { // a nil logger would re-panic inside the recover handlers
		a.logger = logger
	}
	a.mpool = mpool
	if _, ok := mpool.(sdkmempool.TxLookupMempool); !ok {
		a.logger.Warn("app mempool can't look up pooled txs; a stale tx failing recheck can remove its replacement")
	}
	a.signer = signer
	a.maxRecheckBatch = recheckBatchSize
	a.ttlNumBlocks = ttlNumBlocks
	a.recheckDisabled = recheckDisabled
	a.pendingTxCache.enabled = pendingCacheEnabled
	recheckEnabledGauge := float32(0)
	if !recheckDisabled {
		recheckEnabledGauge = 1
	}
	telemetry.SetGauge(recheckEnabledGauge, "cronos", "mempool", "recheck", "enabled")
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
		logger:    log.NewNopLogger(),
	}
}

// recheckDecodingEnabled reports whether sender decoding/bookkeeping should run.
func (a *Manager) recheckDecodingEnabled() bool {
	return !a.recheckDisabled && a.signer != nil && a.decoder != nil
}

// StageSkippedSenders merges the senders of proposal-gate-rejected txs into
// recheckSenders without touching lastCommittedHeight
func (a *Manager) StageSkippedSenders(txs [][]byte) {
	if !a.recheckDecodingEnabled() || len(txs) == 0 {
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

func (a *Manager) mergeRecheckSenders(senders map[string]struct{}) {
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

// LockForCommit is for App.Commit only; see admissionGate.lockForCommit.
func (a *Manager) LockForCommit() (unlock func()) {
	return a.gate.lockForCommit()
}

// SetRPCMaxInflight caps RPC txs being admitted at once; excess get CodeTypeRetry. <= 0 = unlimited.
func (a *Manager) SetRPCMaxInflight(n int) {
	a.rpcTxLimit.max.Store(int64(n))
}

// RPCMaxInflight returns the RPC admission cap; <= 0 = unlimited.
func (a *Manager) RPCMaxInflight() int {
	return int(a.rpcTxLimit.max.Load())
}

// acquireRPCSlot takes an RPC admission slot and returns its release; when none
// is free it counts a shed and returns ok=false.
func (a *Manager) acquireRPCSlot() (release func(), ok bool) {
	if !a.rpcTxLimit.tryAcquire() {
		telemetry.IncrCounter(1, "cronos", "mempool", "rpc", "shed")
		return nil, false
	}
	return a.rpcTxLimit.release, true
}

// SetPreVerify sets the pre-verification hook.
func (a *Manager) SetPreVerify(fn func([]byte) error) {
	a.preVerify = fn
}

// InsertTxHandler validates peer-relayed txs via RunTx(ExecModeCheck) before
// admitting them.
func (a *Manager) InsertTxHandler() sdk.InsertTxHandler {
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		code, _, _ := a.admit(req.Tx, fromGossip)
		return &abci.ResponseInsertTx{Code: code}, nil
	}
}

// InsertTx is the direct RPC insert; error is always nil (failures surface as ABCI codes).
func (a *Manager) InsertTx(txBytes []byte) (*sdk.TxResponse, error) {
	code, codespace, rawLog := a.admit(txBytes, fromRPC)
	return &sdk.TxResponse{Code: code, Codespace: codespace, RawLog: rawLog}, nil
}

// PendingTxs returns a snapshot of pooled txs, safe for the caller to mutate.
func (a *Manager) PendingTxs() []sdk.Tx {
	if a.mpool == nil {
		return nil
	}
	return a.pendingTxCache.get(func() []sdk.Tx {
		return UnorderedPoolSnapshot(context.Background(), a.mpool)
	})
}

func (a *Manager) CountTx() int {
	if a.mpool == nil {
		return 0
	}
	return a.mpool.CountTx()
}

// RecheckDisabled reports whether mempool recheck is disabled
func (a *Manager) RecheckDisabled() bool {
	return a.recheckDisabled
}

// admit is the shared admission path: preVerify + decodeAndEncode unlocked (bad txs skip
// the gate), then RunTx(ExecModeCheck) + encCache.Set under it. Over-capacity and,
// for RPC txs, a full rpcTxLimit map to CodeTypeRetry. tx stays nil when
// encCache is nil; BaseApp.RunTx accepts nil sdk.Tx (uses txBytes).
func (a *Manager) admit(txBytes []byte, source admitSource) (code uint32, codespace, rawLog string) {
	defer func() {
		if r := recover(); r != nil {
			codespace, code, rawLog = errorsmod.ABCIInfo(recoveredPanic(a.logger, panicPathAdmit, r, "tx", txHash(txBytes)), false)
		}
	}()

	if source == fromRPC {
		release, ok := a.acquireRPCSlot()
		if !ok {
			return abci.CodeTypeRetry, "", rpcTxLimitFullLog
		}
		defer release()
	}

	if a.preVerify != nil {
		if err := a.preVerify(txBytes); err != nil {
			cs, c, l := errorsmod.ABCIInfo(err, false)
			return c, cs, l
		}
	}

	tx, canonical, err := a.decodeAndEncode(txBytes)
	if err != nil {
		cs, c, l := errorsmod.ABCIInfo(err, false)
		return c, cs, l
	}

	a.gate.lock()
	defer a.gate.unlock()

	if _, _, _, err = a.runner.RunTx(sdk.ExecModeCheck, txBytes, tx, -1, nil, nil); err != nil {
		if errorsmod.IsOf(err, sdkmempool.ErrMempoolTxMaxCapacity) {
			return abci.CodeTypeRetry, "", mempoolFullLog
		}
		cs, c, l := errorsmod.ABCIInfo(err, false)
		return c, cs, l
	}

	a.encCache.Set(tx, canonical)
	a.pendingTxCache.invalidate()
	return abci.CodeTypeOK, "", ""
}

// Encoding before RunTx keeps a tx whose encoding panics out of ante and the pool; nils without a cache, raw bytes on encode error.
func (a *Manager) decodeAndEncode(raw []byte) (sdk.Tx, []byte, error) {
	if a.encCache == nil {
		return nil, nil, nil
	}
	tx, err := a.decoder(raw)
	if err != nil {
		return nil, nil, sdkerrors.ErrTxDecode.Wrap(err.Error())
	}
	if bz, err := a.encode(tx); err == nil {
		return tx, bz, nil
	}
	return tx, raw, nil
}

// Deferred unlock: a recovered encoder panic must not leave encodeMu held.
func (a *Manager) encode(tx sdk.Tx) ([]byte, error) {
	a.encodeMu.Lock()
	defer a.encodeMu.Unlock()
	return a.txEncoder(tx)
}

// CheckTxHandler runs RPC CheckTx.
func (a *Manager) CheckTxHandler() sdk.CheckTxHandler {
	return func(runTx sdk.RunTx, req *abci.RequestCheckTx) (resp *abci.ResponseCheckTx, _ error) {
		defer func() {
			if r := recover(); r != nil {
				resp = sdkerrors.ResponseCheckTxWithEvents(recoveredPanic(a.logger, panicPathCheckTx, r, "tx", txHash(req.Tx)), 0, 0, nil, a.trace)
			}
		}()

		release, ok := a.acquireRPCSlot()
		if !ok {
			return &abci.ResponseCheckTx{Code: abci.CodeTypeRetry, Log: rpcTxLimitFullLog}, nil
		}
		defer release()

		// Decode and encode before locking
		tx, canonical, err := a.decodeAndEncode(req.Tx)
		if err != nil {
			return sdkerrors.ResponseCheckTxWithEvents(err, 0, 0, nil, a.trace), nil
		}

		a.gate.lock()
		defer a.gate.unlock()

		gasInfo, result, anteEvents, err := runTx(req.Tx, tx)
		if err != nil {
			return sdkerrors.ResponseCheckTxWithEvents(err, gasInfo.GasWanted, gasInfo.GasUsed, anteEvents, a.trace), nil
		}

		a.encCache.Set(tx, canonical)
		a.pendingTxCache.invalidate()

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

// StageRecheckSenders records committed-block senders for RecheckTxs and stages
// the committed height.
func (a *Manager) StageRecheckSenders(height int64, txs [][]byte) {
	a.pendingTxCache.invalidate()

	// Decode + extract signers unlocked (the expensive part), then publish height
	// and recheckSenders in one critical section so a reader never sees a torn update.
	var senders map[string]struct{}
	if a.recheckDecodingEnabled() {
		senders = make(map[string]struct{}, len(txs))
		for _, bz := range txs {
			tx, err := a.decoder(bz)
			if err != nil {
				continue // non-sdk txs (e.g. vote extensions) have no mempool entry
			}
			for _, s := range a.stagedSenders(tx) {
				senders[s] = struct{}{}
			}
		}
	}

	a.stagingMu.Lock()
	a.lastCommittedHeight = height
	a.mergeRecheckSenders(senders)
	a.stagingMu.Unlock()
}

// TriggerRecheck schedules an async recheck.
// Call only from the consensus path (App.Commit).
func (a *Manager) TriggerRecheck() {
	if a.worker.trigger == nil {
		a.RecheckTxs()
		return
	}
	a.worker.recheck()
}

// Close stops the recheck worker.
func (a *Manager) Close() {
	a.worker.stop()
}

// WaitForRecheck blocks until the pending recheck finishes;
func (a *Manager) WaitForRecheck(ctx context.Context) {
	if a.worker.trigger == nil {
		return
	}
	a.worker.wait(ctx)
}

// WaitForRecheckTimedOut is WaitForRecheck bounded by timeout, reporting whether the
// timeout was hit.
func (a *Manager) WaitForRecheckTimedOut(ctx context.Context, timeout time.Duration) bool {
	if a.worker.trigger == nil {
		return false
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return a.worker.wait(waitCtx)
}

// RecheckTxs evicts pool txs invalidated by the last block.
func (a *Manager) RecheckTxs() {
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
			a.pendingTxCache.invalidate() // evictions before the panic must reach pending queries
			logRecoveredPanic(a.logger, panicPathRecheck, r)
		}
	}()

	snapshot := PoolSnapshot(context.Background(), a.mpool)
	candidates := a.capRecheckTxs(a.selectTxs(snapshot, recheckSenders, height, deferred))
	a.runRecheck(candidates)

	a.pendingTxCache.invalidate()
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

// restoreStaging puts drained work back after an aborted cycle so the next cycle retries it.
func (a *Manager) restoreStaging(recheckSenders map[string]struct{}, deferred []sdk.Tx) {
	a.stagingMu.Lock()
	defer a.stagingMu.Unlock()
	a.mergeRecheckSenders(recheckSenders)
	if a.deferred == nil {
		a.deferred = deferred
	}
}

// selectTxs scans the pool to retrieve txs for recheck. Caller (RecheckTxs)
// only invokes this when recheck is enabled.
func (a *Manager) selectTxs(snapshot []sdk.Tx, recheckSenders map[string]struct{}, height int64, deferred []sdk.Tx) []sdk.Tx {
	// deferredLive: carried-over tx -> still in pool, plus replacements scheduled in a carried tx's slot.
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

	// Pass 1: evictions. Recheck every expired tx's senders: its later nonces now have a gap, and a
	// replaced one's replacement still needs a recheck.
	var evictedSet map[sdk.Tx]struct{} // nil until first eviction; nil-map read is safe
	var timedOut, ttlExpired []sdk.Tx
	now := time.Now()
	for _, tx := range snapshot {
		evictedPoison := a.guardRecheckTx(&tx, func() {
			if txTimedout(tx, height, now) {
				timedOut = append(timedOut, tx)
				return
			}
			if a.ttlNumBlocks > 0 {
				arrived, expired := txTTLExpired(a.arrival, tx, height, a.ttlNumBlocks)
				if expired {
					ttlExpired = append(ttlExpired, tx)
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
	// Evict in batches: one gate hold per tx queues behind admissions; one for all stalls them.
	numTimedOut := len(timedOut)
	expired := make([]sdk.Tx, 0, len(timedOut)+len(ttlExpired))
	expired = append(append(expired, timedOut...), ttlExpired...)
	keys := make([]poolKey, min(evictBatchSize, len(expired)))
	// A tx admitted at a freed key already applied its nonce, so deferred txs aren't redirected to it.
	var freedKeys map[poolKey]struct{}
	for start := 0; start < len(expired); start += evictBatchSize {
		batch := expired[start:min(start+evictBatchSize, len(expired))]
		// Keys depend only on the tx, so derive them outside the gate.
		for j, tx := range batch {
			var sigs []string
			if a.guardRecheckTx(&tx, func() { keys[j], sigs = a.keyAndSigners(tx) }) {
				evictedSet = markEvicted(evictedSet, tx)
				continue
			}
			recheckSenders = addSigners(recheckSenders, sigs)
		}
		func() {
			a.gate.lock()
			defer a.gate.unlock()
			for j, tx := range batch {
				if _, poison := evictedSet[tx]; poison {
					continue
				}
				var evicted bool
				if evictedSet, evicted = a.tryEvictForRecheck(tx, keys[j], evictedSet); !evicted {
					continue
				}
				if keys[j].valid() {
					if freedKeys == nil {
						freedKeys = make(map[poolKey]struct{})
					}
					freedKeys[keys[j]] = struct{}{}
				}
				if start+j < numTimedOut {
					expiredEvicted++
				} else {
					ttlEvicted++
				}
			}
		}()
	}
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
		if a.guardRecheckTx(&tx, func() { sigs = a.signers(tx) }) {
			delete(deferredLive, tx) // evicted, so don't recheck it as carry
			evictedSet = markEvicted(evictedSet, tx)
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
			ordered = append(ordered, tx)
			continue
		}
		if _, wasEvicted := evictedSet[tx]; wasEvicted {
			continue // its key is freed, as for freedKeys
		}
		// Recheck a replaced tx's replacement in its slot, so later deferred nonces see the sequence.
		a.encCache.Evict(tx)
		var key poolKey
		if a.guardRecheckTx(&tx, func() { key = a.keyOf(tx) }) {
			continue
		}
		if _, freed := freedKeys[key]; freed {
			continue
		}
		if cur, found, known := a.pooledAtKey(key); known && found && cur != tx {
			if _, dup := deferredLive[cur]; !dup {
				deferredLive[cur] = true
				ordered = append(ordered, cur)
			}
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

// evictForRecheck evicts tx and reports whether it was pooled; a replaced tx stays a candidate so its
// replacement is rechecked. Callers hold the gate.
func (a *Manager) evictForRecheck(tx sdk.Tx, key poolKey, evictedSet map[sdk.Tx]struct{}) (map[sdk.Tx]struct{}, bool) {
	cur, found, known := a.pooledAtKey(key)
	// When the pool can't tell, fall back to removal by key.
	pooled := !known || (found && cur == tx)
	a.evict(tx, pooled)
	if pooled {
		evictedSet = markEvicted(evictedSet, tx)
	}
	return evictedSet, pooled
}

// tryEvictForRecheck is evictForRecheck that logs a panic instead, leaving tx pooled to be guarded
// again next cycle. Callers hold the gate.
func (a *Manager) tryEvictForRecheck(tx sdk.Tx, key poolKey, evictedSet map[sdk.Tx]struct{}) (set map[sdk.Tx]struct{}, evicted bool) {
	set = evictedSet
	defer func() {
		if r := recover(); r != nil {
			logRecoveredPanic(a.logger, panicPathRecheckTx, r, "evict_failed", true)
			evicted = false
		}
	}()
	return a.evictForRecheck(tx, key, evictedSet)
}

// addSigners folds sigs into senders, allocating it lazily.
func addSigners(senders map[string]struct{}, sigs []string) map[string]struct{} {
	if len(sigs) > 0 && senders == nil {
		senders = make(map[string]struct{})
	}
	for _, s := range sigs {
		senders[s] = struct{}{}
	}
	return senders
}

func markEvicted(evictedSet map[sdk.Tx]struct{}, tx sdk.Tx) map[sdk.Tx]struct{} {
	if evictedSet == nil {
		evictedSet = make(map[sdk.Tx]struct{})
	}
	evictedSet[tx] = struct{}{}
	return evictedSet
}

// guardRecheckTx runs step and evicts *tx, which step may redirect, if step panics, so one poison tx
// can't stall every cycle's eviction. Reports whether step panicked. Callers don't hold the gate.
func (a *Manager) guardRecheckTx(tx *sdk.Tx, step func()) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			logRecoveredPanic(a.logger, panicPathRecheckTx, r)
			a.evictPoisonTx(*tx)
			panicked = true
		}
	}()
	step()
	return false
}

// Eviction calls the same tx methods that may have panicked; if it panics too, the tx
// stays pooled and is guarded again next cycle.
func (a *Manager) evictPoisonTx(tx sdk.Tx) {
	defer func() {
		if r := recover(); r != nil {
			logRecoveredPanic(a.logger, panicPathRecheckTx, r, "evict_failed", true)
		}
	}()
	var key poolKey
	func() {
		defer func() { _ = recover() }() // already logged; a zero key falls back to removal by key
		key = a.keyOf(tx)
	}()
	a.gate.lock()
	defer a.gate.unlock()
	a.tryEvictForRecheck(tx, key, nil)
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

// runRecheck re-validates candidates via RunTx(ReCheck)
func (a *Manager) runRecheck(candidates []sdk.Tx) {
	var evicted float32
	// A second recheck of one tx would fail its nonce and evict it. Only a redirect can cause that,
	// so the set is built lazily from the earlier candidates.
	var seen map[sdk.Tx]struct{}
	// A stale tx at an evicted key must neither replay the sequence nor be redirected.
	var evictedKeys map[poolKey]struct{}
	markKey := func(key poolKey) {
		if !key.valid() {
			return
		}
		if evictedKeys == nil {
			evictedKeys = make(map[poolKey]struct{})
		}
		evictedKeys[key] = struct{}{}
	}
	for i, tx := range candidates {
		var key poolKey
		if a.guardRecheckTx(&tx, func() {
			bz, _, err := EncodeTx(a.encCache, a.txEncoder, tx)
			if err != nil {
				return
			}
			key = a.keyOf(tx) // outside the gate; a redirect keeps the key
			a.gate.lock()
			defer a.gate.unlock() // a recovered panic must not leave the gate held
			// A tx replaced since the snapshot is rechecked as its replacement, since the pool removes by
			// key; one removed since still replays the sequence but has nothing to evict.
			pooled := true
			cur, found, known := a.pooledAtKey(key)
			if known && cur != tx {
				a.encCache.Evict(tx)
				pooled = found
				if _, stale := evictedKeys[key]; stale {
					return
				}
				if found {
					if seen == nil {
						seen = make(map[sdk.Tx]struct{}, i+1)
						for _, prev := range candidates[:i] {
							seen[prev] = struct{}{}
						}
					}
					tx = cur
					if bz, _, err = EncodeTx(a.encCache, a.txEncoder, tx); err != nil {
						return
					}
				}
			}
			if seen != nil {
				if _, dup := seen[tx]; dup {
					return
				}
				seen[tx] = struct{}{}
			}
			_, _, _, err = a.runner.RunTx(sdk.ExecModeReCheck, bz, tx, -1, nil, nil)
			if err != nil && pooled {
				// baseapp removes tx only on an ante failure; gate still held, so the key is unchanged.
				a.evict(tx, true)
				evicted++
				markKey(key)
			}
		}) {
			markKey(key)
		}
	}
	if evicted > 0 {
		telemetry.IncrCounter(evicted, "cronos", "mempool", "recheck", "evicted")
	}
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

// evict drops tx from the encoder cache, and from the pool only when pooled, since the pool removes
// by key and a replacement must stay. Callers hold the gate since computing pooled.
func (a *Manager) evict(tx sdk.Tx, pooled bool) {
	if pooled {
		_ = a.mpool.Remove(tx)
	}
	a.encCache.Evict(tx)
}

// pooledAtKey returns the tx pooled at key, compared by identity since pooled txs are decoded
// pointers. known is false when the pool or key can't be queried.
func (a *Manager) pooledAtKey(key poolKey) (cur sdk.Tx, found, known bool) {
	pool, ok := a.mpool.(sdkmempool.TxLookupMempool)
	if !ok || !key.valid() {
		return nil, false, false
	}
	cur, found = pool.GetTx(key.sender, key.nonce)
	return cur, found, true
}

// poolKey is the sender and nonce the pool keys a tx by; the zero value means none.
type poolKey struct {
	sender string
	nonce  uint64
}

func (k poolKey) valid() bool { return k.sender != "" }

func (a *Manager) keyOf(tx sdk.Tx) poolKey {
	sigs := a.signerData(tx)
	if len(sigs) == 0 {
		return poolKey{}
	}
	return poolKeyAt(tx, sigs[0].Sequence, sigs[0].Signer.String())
}

// keyAndSigners returns tx's pool key and signers from one signer extraction.
func (a *Manager) keyAndSigners(tx sdk.Tx) (poolKey, []string) {
	sigs := a.signerData(tx)
	signers := signerStrings(sigs)
	if len(sigs) == 0 {
		return poolKey{}, signers
	}
	return poolKeyAt(tx, sigs[0].Sequence, signers[0]), signers
}

// signerData returns tx's signers, or nil when they can't be extracted.
func (a *Manager) signerData(tx sdk.Tx) []sdkmempool.SignerData {
	if a.signer == nil || tx == nil {
		return nil
	}
	sigs, err := a.signer.GetSigners(tx)
	if err != nil {
		return nil
	}
	return sigs
}

func poolKeyAt(tx sdk.Tx, sequence uint64, sender string) poolKey {
	nonce, err := sdkmempool.ChooseNonce(sequence, tx)
	if err != nil {
		return poolKey{}
	}
	return poolKey{sender, nonce}
}

// stagedSenders adds later eth msg senders: CheckTx rejects multi-sender batches, but a proposer
// can include one that uses another pooled tx's nonce.
func (a *Manager) stagedSenders(tx sdk.Tx) []string {
	keys := a.signers(tx)
	msgs := tx.GetMsgs()
	if len(msgs) < 2 {
		return keys
	}
	for _, msg := range msgs[1:] {
		if ethTx, ok := msg.(*evmtypes.MsgEthereumTx); ok {
			keys = append(keys, ethTx.GetFrom().String())
		}
	}
	return keys
}

func (a *Manager) signers(tx sdk.Tx) []string {
	return signerStrings(a.signerData(tx))
}

func signerStrings(sigs []sdkmempool.SignerData) []string {
	if sigs == nil {
		return nil
	}
	strs := make([]string, len(sigs))
	for i, s := range sigs {
		strs[i] = s.Signer.String()
	}
	return strs
}

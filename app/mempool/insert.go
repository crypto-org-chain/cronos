package mempool

import (
	"sync"

	abci "github.com/cometbft/cometbft/abci/types"

	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/baseapp"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// txRunner is the subset of baseapp.BaseApp used by the Admitter.
// *baseapp.BaseApp satisfies this interface; tests may inject stubs.
type txRunner interface {
	RunTx(mode sdk.ExecMode, txBytes []byte, tx sdk.Tx, txIndex int, txMultiStore storetypes.MultiStore, incarnationCache map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error)
}

// Compile-time check: *baseapp.BaseApp implements txRunner.
var _ txRunner = (*baseapp.BaseApp)(nil)

// Admitter owns the app-side mempool admission path for mempool.type=app.
// Admission runs RunTx against the shared checkState multistore (a Go map that
// is unsafe for concurrent writes), so it is serialized by a single mutex.
type Admitter struct {
	// mu serializes the two ADMISSION paths against each other. CometBFT calls
	// InsertTx (per-peer P2P AppReactor) and CheckTx (per-tx RPC BroadcastTx)
	// concurrently, and AppMempool.CheckTx runs LOCK-FREE (skips the ABCI
	// client lock). Both call RunTx(ExecModeCheck) on the shared checkState, so
	// without this mutex an RPC CheckTx racing a p2p InsertTx panics on the
	// concurrent map write.
	//
	// NOTE: mu does NOT serialize admission against consensus-side checkState
	// mutation (Commit resets checkState; see baseapp Commit). Those calls go
	// through CometBFT's localClient.mtx and never through the Admitter, so mu
	// cannot exclude them. For the CList mempool that gap is closed by
	// mempool.Lock() around Commit, but AppMempool.Lock() is a no-op — so the
	// admission-vs-Commit race is a known, pre-existing limitation of
	// mempool.type=app, not addressed here.
	mu        sync.Mutex
	runner    txRunner
	encCache  *EncoderCache
	txGet     TxGetter
	txEncoder sdk.TxEncoder
	// trace mirrors BaseApp.Trace(); controls whether CheckTx errors include
	// the full stack trace in the response log.
	trace bool
}

// NewAdmitter builds the Admitter for mempool.type=app. It must be registered
// with BaseApp.SetInsertTxHandler (via InsertTxHandler) before Seal.
//
// If encCache is non-nil, txGet and txEncoder must also be non-nil so InsertTx
// can register canonical bytes for the reap fast path.
func NewAdmitter(app *baseapp.BaseApp, txGet TxGetter, encCache *EncoderCache, txEncoder sdk.TxEncoder) *Admitter {
	a := newAdmitter(app, txGet, encCache, txEncoder)
	a.trace = app.Trace()
	return a
}

func newAdmitter(runner txRunner, txGet TxGetter, encCache *EncoderCache, txEncoder sdk.TxEncoder) *Admitter {
	if encCache != nil && txGet == nil {
		panic("mempool: encCache requires txGet != nil")
	}
	if encCache != nil && txEncoder == nil {
		panic("mempool: encCache requires txEncoder != nil for canonical bytes")
	}
	return &Admitter{
		runner:    runner,
		encCache:  encCache,
		txGet:     txGet,
		txEncoder: txEncoder,
	}
}

// AdmissionMutex returns the mutex that serializes admission (CheckTx +
// InsertTx). The App must also acquire it around BaseApp.Commit so the
// consensus-side checkState reset cannot run concurrently with a lock-free
// admission RunTx — the admission-vs-Commit gap that AppMempool.Lock()'s no-op
// leaves open. Returns a stable pointer (Admitter is always heap-allocated).
func (a *Admitter) AdmissionMutex() *sync.Mutex {
	return &a.mu
}

// InsertTxHandler validates peer-relayed txs via RunTx(ExecModeCheck) before
// admitting them to the mempool. Flood protection relies on CometBFT peer
// limits, not this handler. If encCache is non-nil, admitted txs have their
// canonical bytes registered so ReapTxsHandler can skip proto.Marshal;
// re-encoding avoids storing non-minimal peer bytes in the cache.
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

		if a.encCache != nil {
			a.registerCanonical(req.Tx)
		}

		return &abci.ResponseInsertTx{Code: abci.CodeTypeOK}, nil
	}
}

// registerCanonical re-encodes an admitted tx to its canonical bytes and stores
// them in the EncoderCache so ReapTxsHandler can skip proto.Marshal. Re-encoding
// (rather than caching raw) keeps a peer's non-minimal proto bytes out of the
// cache. Falls back to raw bytes if re-encoding errors so reap can still ship
// the tx. Caller holds a.mu and must have checked encCache != nil.
func (a *Admitter) registerCanonical(raw []byte) {
	tx, ok := a.txGet(raw)
	if !ok {
		return
	}
	bz := raw
	if canonical, err := a.txEncoder(tx); err == nil {
		bz = canonical
	}
	a.encCache.Register(tx, bz)
}

// CheckTxHandler serializes RPC-driven CheckTx against InsertTxHandler on the
// same mutex. CometBFT's AppMempool calls CheckTx LOCK-FREE (no ABCI client
// lock) for BroadcastTx* RPC, while peer-relayed txs arrive via InsertTx. Both
// run RunTx(ExecModeCheck) against the shared checkState multistore (a Go map
// unsafe for concurrent writes), so without a shared lock an RPC CheckTx racing
// a p2p InsertTx corrupts checkState and panics. This handler closes only that
// admission-vs-admission race; see the Admitter.mu doc for the residual
// admission-vs-Commit limitation. The runTx closure is supplied by BaseApp
// already bound to the correct exec mode (panics inside it are converted to
// errors by BaseApp.RunTx's own recover, so they never escape past the deferred
// Unlock). On success it also registers the tx's canonical bytes in the
// EncoderCache so RPC-submitted txs (which never traverse InsertTx) still hit
// the reap fast path.
func (a *Admitter) CheckTxHandler() sdk.CheckTxHandler {
	return func(runTx sdk.RunTx, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
		a.mu.Lock()
		defer a.mu.Unlock()

		gasInfo, result, anteEvents, err := runTx(req.Tx, nil)
		if err != nil {
			return sdkerrors.ResponseCheckTxWithEvents(err, gasInfo.GasWanted, gasInfo.GasUsed, anteEvents, a.trace), nil
		}

		if a.encCache != nil {
			a.registerCanonical(req.Tx)
		}

		// Events are passed through without sdk.MarkEventsToIndex (which the
		// default BaseApp.CheckTx applies): that flag only feeds the tx indexer,
		// which operates on FinalizeBlock results, not CheckTx — and indexEvents
		// has no public accessor here. No observable effect for RPC callers.
		return &abci.ResponseCheckTx{
			GasWanted: int64(gasInfo.GasWanted),
			GasUsed:   int64(gasInfo.GasUsed),
			Log:       result.Log,
			Data:      result.Data,
			Events:    result.Events,
		}, nil
	}
}

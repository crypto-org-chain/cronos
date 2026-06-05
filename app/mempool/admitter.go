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
type Admitter struct {
	// mu serializes admission. AppMempool delivers InsertTx (p2p) and CheckTx
	// (RPC) concurrently, and CheckTx runs lock-free, yet both call
	// RunTx(ExecModeCheck) on the shared checkState — unsafe for concurrent
	// access. App.Commit also takes this mutex (via AdmissionMutex) so the
	// checkState reset can't race in-flight admission, since AppMempool.Lock()
	// is a no-op.
	mu        sync.Mutex
	runner    txRunner
	encCache  *EncoderCache
	txGet     TxGetter
	txEncoder sdk.TxEncoder
	// trace mirrors BaseApp.Trace(): include stack traces in CheckTx error logs.
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

// AdmissionMutex exposes the admission mutex so App.Commit can serialize the
// checkState reset against lock-free admission. Pointer is stable (Admitter is
// heap-allocated).
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

		a.registerCanonical(req.Tx)
		return &abci.ResponseInsertTx{Code: abci.CodeTypeOK}, nil
	}
}

// registerCanonical caches an admitted tx's canonical (re-encoded) bytes so
// ReapTxsHandler can skip proto.Marshal; re-encoding keeps non-minimal peer
// bytes out of the cache. Falls back to raw on encode error. No-op when the
// cache is disabled. Caller holds mu.
func (a *Admitter) registerCanonical(raw []byte) {
	if a.encCache == nil {
		return
	}
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

// CheckTxHandler runs RPC CheckTx under the admission mutex (see mu): without it
// a lock-free RPC CheckTx races a p2p InsertTx on checkState. The runTx closure
// comes from BaseApp bound to the exec mode; its panics are recovered inside
// BaseApp.RunTx. On success it registers canonical bytes so RPC txs (which skip
// InsertTx) still hit the reap fast path.
func (a *Admitter) CheckTxHandler() sdk.CheckTxHandler {
	return func(runTx sdk.RunTx, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
		a.mu.Lock()
		defer a.mu.Unlock()

		gasInfo, result, anteEvents, err := runTx(req.Tx, nil)
		if err != nil {
			return sdkerrors.ResponseCheckTxWithEvents(err, gasInfo.GasWanted, gasInfo.GasUsed, anteEvents, a.trace), nil
		}

		a.registerCanonical(req.Tx)

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

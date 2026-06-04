package mempool

import (
	"sync"

	abci "github.com/cometbft/cometbft/abci/types"

	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/baseapp"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
	// mu serializes admission. CometBFT calls InsertTx concurrently
	// (per-peer P2P AppReactor + per-tx RPC BroadcastTx goroutines) and the
	// ABCI client holds no connection lock. RunTx writes the shared checkState
	// multistore, so concurrent admission races and panics without this mutex.
	mu        sync.Mutex
	runner    txRunner
	encCache  *EncoderCache
	txGet     TxGetter
	txEncoder sdk.TxEncoder
}

// NewAdmitter builds the Admitter for mempool.type=app. It must be registered
// with BaseApp.SetInsertTxHandler (via InsertTxHandler) before Seal.
//
// If encCache is non-nil, txGet and txEncoder must also be non-nil so InsertTx
// can register canonical bytes for the reap fast path.
func NewAdmitter(app *baseapp.BaseApp, txGet TxGetter, encCache *EncoderCache, txEncoder sdk.TxEncoder) *Admitter {
	return newAdmitter(app, txGet, encCache, txEncoder)
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
			if tx, ok := a.txGet(req.Tx); ok {
				bz := req.Tx
				if canonical, err := a.txEncoder(tx); err == nil {
					bz = canonical
				}
				a.encCache.Register(tx, bz)
			}
		}

		return &abci.ResponseInsertTx{Code: abci.CodeTypeOK}, nil
	}
}

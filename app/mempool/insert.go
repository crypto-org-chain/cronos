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
	// mu serializes the admission path. CometBFT's AppMempool calls InsertTx
	// concurrently (P2P AppReactor.Receive runs per-peer; the RPC BroadcastTx
	// path launches one goroutine per tx) and the local ABCI client does NOT
	// take the connection lock for InsertTx/CheckTx — it assumes the handler is
	// thread-safe. RunTx branches and writes the shared checkState multistore
	// (cacheTxContext + msCache.Write), so without this lock concurrent
	// ingestion races and can panic with "concurrent map writes". Holding it
	// across the whole RunTx call also makes admission atomic, so a tx that
	// slips past CometBFT's dedup is validated at most once.
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

// InsertTxHandler returns an sdk.InsertTxHandler that validates peer-relayed
// txs via RunTx(ExecModeCheck) before admitting them to the mempool.
//
// DoS note: every gossiped tx costs one RunTx(ExecModeCheck) (a secp256k1
// signature verification). CometBFT's AppMempool dedups by tx.Key() before
// calling InsertTx, but a flood of distinct well-formed txs is bounded only by
// the p2p layer, so rely on CometBFT peer limits / rate limiting — not this
// handler — for gossip-flood protection.
//
// If encCache is non-nil, each successfully-admitted tx has its canonical bytes
// registered so ReapTxsHandler can skip proto.Marshal on the reap hot path.
// Re-encoding from the decoded tx ensures canonical proto bytes are stored even
// if req.Tx arrived with non-minimal encoding.
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

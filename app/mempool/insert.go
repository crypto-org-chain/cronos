package mempool

import (
	"context"
	"sync"

	abci "github.com/cometbft/cometbft/abci/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log/v2"

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

// Admitter owns the app-side mempool admission and recheck paths for
// mempool.type=app. Both paths run RunTx against the shared checkState
// multistore (a Go map that is unsafe for concurrent writes), so they are
// serialized by a single mutex.
type Admitter struct {
	// mu serializes the admission and recheck paths. CometBFT's AppMempool
	// calls InsertTx concurrently (P2P AppReactor.Receive runs per-peer; the
	// RPC BroadcastTx path launches one goroutine per tx) and the local ABCI
	// client does NOT take the connection lock for InsertTx/CheckTx — it
	// assumes the handler is thread-safe. RunTx branches and writes the shared
	// checkState multistore (cacheTxContext + msCache.Write), so without this
	// lock concurrent ingestion races and can panic with "concurrent map
	// writes". Holding it across the whole RunTx call also makes admission
	// atomic, so a tx that slips past CometBFT's dedup is validated at most
	// once.
	mu        sync.Mutex
	runner    txRunner
	mpool     sdkmempool.Mempool
	encCache  *EncoderCache
	txGet     TxGetter
	txEncoder sdk.TxEncoder
	logger    log.Logger
}

// NewAdmitter builds the Admitter for mempool.type=app. It must be registered
// with BaseApp.SetInsertTxHandler (via InsertTxHandler) and
// BaseApp.SetPrepareCheckStater (via Recheck) before Seal.
//
// txEncoder is always required: Recheck needs canonical bytes for txs not in
// encCache. If encCache is non-nil, txGet must also be non-nil so InsertTx can
// register canonical bytes for the reap fast path.
func NewAdmitter(app *baseapp.BaseApp, mpool sdkmempool.Mempool, txGet TxGetter, encCache *EncoderCache, txEncoder sdk.TxEncoder, logger log.Logger) *Admitter {
	return newAdmitter(app, mpool, txGet, encCache, txEncoder, logger)
}

func newAdmitter(runner txRunner, mpool sdkmempool.Mempool, txGet TxGetter, encCache *EncoderCache, txEncoder sdk.TxEncoder, logger log.Logger) *Admitter {
	if mpool == nil {
		panic("mempool: Admitter requires mpool != nil")
	}
	if txEncoder == nil {
		panic("mempool: Admitter requires txEncoder != nil: recheck needs canonical bytes and a nil encoder risks non-canonical bytes in proposals")
	}
	if encCache != nil && txGet == nil {
		panic("mempool: encCache requires txGet != nil")
	}
	return &Admitter{
		runner:    runner,
		mpool:     mpool,
		encCache:  encCache,
		txGet:     txGet,
		txEncoder: txEncoder,
		logger:    logger,
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

// Recheck re-validates every resident mempool tx against the freshly committed
// checkState. Wired as a baseapp PrepareCheckStater, it runs at the end of
// Commit after checkState is reset and while CometBFT holds the mempool lock,
// so no InsertTx is concurrent; it still takes a.mu to serialize the shared
// checkState writes RunTx performs and to stay correct if that lock assumption
// changes.
//
// RunTx(ExecModeReCheck) skips ValidateBasic (stateless, already passed at
// admission) and, on AnteHandler failure, removes the tx from the mempool
// itself (baseapp -> mempool.RemoveWithReason); on success it does NOT
// re-insert. So a resident tx invalidated by another tx's committed state
// change (nonce consumed, balance drained, signing key rotated) is evicted here
// instead of being proposed and failing at execution, wasting block space.
func (a *Admitter) Recheck(sdk.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Snapshot under the pool lock, then run RunTx outside the iteration:
	// RunTx(ExecModeReCheck) calls mempool.Remove on failure, which must not
	// mutate the pool mid-iteration.
	snapshot := make([]sdk.Tx, 0, a.mpool.CountTx())
	sdkmempool.SelectBy(context.Background(), a.mpool, nil, func(tx sdk.Tx) bool {
		snapshot = append(snapshot, tx)
		return true
	})

	for _, tx := range snapshot {
		bz, ok := a.encCache.Bytes(tx)
		if !ok {
			var err error
			if bz, err = a.txEncoder(tx); err != nil {
				if a.logger != nil {
					a.logger.Error("recheck encode failed; skipping tx", "err", err)
				}
				continue
			}
		}
		// Ignore the returned error: on AnteHandler failure baseapp has already
		// removed the tx from the mempool.
		_, _, _, _ = a.runner.RunTx(sdk.ExecModeReCheck, bz, tx, -1, nil, nil)
	}
}

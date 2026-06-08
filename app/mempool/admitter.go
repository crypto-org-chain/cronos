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

type txRunner interface {
	RunTx(mode sdk.ExecMode, txBytes []byte, tx sdk.Tx, txIndex int, txMultiStore storetypes.MultiStore, incarnationCache map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error)
}

var _ txRunner = (*baseapp.BaseApp)(nil)

// Admitter owns the app-side mempool admission path for mempool.type=app.
type Admitter struct {
	mu        sync.Mutex
	runner    txRunner
	encCache  *EncoderCache
	txGet     TxGetter
	txEncoder sdk.TxEncoder
	trace     bool

	preVerify func([]byte) error

	mpool     sdkmempool.Mempool
	signer    sdkmempool.SignerExtractionAdapter
	decoder   sdk.TxDecoder
	pendingMu sync.Mutex
	// pending holds leftover transactions in the mempool after a last committed block
	pending map[string]struct{}
	lastCommittedHeight int64
}

// NewAdmitter builds the Admitter for mempool.type=app; register it via
// BaseApp.SetInsertTxHandler before Seal.
func NewAdmitter(app *baseapp.BaseApp, txGet TxGetter, encCache *EncoderCache, txEncoder sdk.TxEncoder) *Admitter {
	a := newAdmitter(app, txGet, encCache, txEncoder)
	a.trace = app.Trace()
	return a
}

func newAdmitter(runner txRunner, txGet TxGetter, encCache *EncoderCache, txEncoder sdk.TxEncoder) *Admitter {
	if encCache != nil {
		if txGet == nil {
			panic("mempool: encCache requires txGet != nil")
		}
		if txEncoder == nil {
			panic("mempool: encCache requires txEncoder != nil for canonical bytes")
		}
	}
	return &Admitter{
		runner:    runner,
		encCache:  encCache,
		txGet:     txGet,
		txEncoder: txEncoder,
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
		// Pre-verify the EVM signature lock-free: ecrecover dominates admission
		// cost and touches no store. Non-EVM txs and signer-build failures return
		// nil and fall through to the locked RunTx below. (The in-lock re-verify
		// skip lands with the ethermint fork; until then this double-verifies.)
		if a.preVerify != nil {
			if err := a.preVerify(req.Tx); err != nil {
				return reject(err), nil
			}
		}

		a.mu.Lock()
		defer a.mu.Unlock()

		_, _, _, err := a.runner.RunTx(sdk.ExecModeCheck, req.Tx, nil, -1, nil, nil)
		if err != nil {
			if errorsmod.IsOf(err, sdkmempool.ErrMempoolTxMaxCapacity) {
				return &abci.ResponseInsertTx{Code: abci.CodeTypeRetry}, nil
			}
			return reject(err), nil
		}

		a.cacheTx(req.Tx)
		return &abci.ResponseInsertTx{Code: abci.CodeTypeOK}, nil
	}
}

// reject maps a RunTx/pre-verify error to its ABCI code for an InsertTx
// rejection. The ErrMempoolTxMaxCapacity retry case is handled at the call site.
func reject(err error) *abci.ResponseInsertTx {
	_, code, _ := errorsmod.ABCIInfo(err, false)
	return &abci.ResponseInsertTx{Code: code}
}

// cacheTx registers tx in the encoder cache.
func (a *Admitter) cacheTx(raw []byte) {
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
	a.encCache.Set(tx, bz)
}

// CheckTxHandler runs RPC CheckTx under mu so it can't race a p2p InsertTx on
// checkState. The runTx closure comes from BaseApp bound to the exec mode; its
// panics are recovered inside BaseApp.RunTx. On success it registers canonical
// bytes so RPC txs (which skip InsertTx) still hit the reap fast path.
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

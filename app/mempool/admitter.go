package mempool

import (
	"context"
	"sync"

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
	pendingMu       sync.Mutex
	// pending holds leftover transactions in the mempool after a last committed block
	pending             map[string]struct{}
	lastCommittedHeight int64
}

// NewAdmitter builds the Admitter for mempool.type=app; register it via
// BaseApp.SetInsertTxHandler before Seal.
func NewAdmitter(app *baseapp.BaseApp, encCache *EncoderCache, txEncoder sdk.TxEncoder, mpool sdkmempool.Mempool, signer sdkmempool.SignerExtractionAdapter, decoder sdk.TxDecoder) *Admitter {
	a := newAdmitter(app, encCache, txEncoder, decoder)
	a.trace = app.Trace()
	a.mpool = mpool
	a.signer = signer
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

// SetRecheckBatchSize caps RunTx(ReCheck) calls per Commit cycle. 0 = unlimited.
func (a *Admitter) SetRecheckBatchSize(n int) {
	a.maxRecheckBatch = n
}

// StageRecheckSenders records the senders of the just-committed block's txs so
// RecheckTxs can re-validate only their remaining pending txs, and stages the
// committed height for TimeoutHeight eviction.
func (a *Admitter) StageRecheckSenders(height int64, txs [][]byte) {
	a.pendingMu.Lock()
	a.lastCommittedHeight = height
	a.pendingMu.Unlock()

	if a.signer == nil || a.decoder == nil {
		return
	}
	senders := make(map[string]struct{}, len(txs))
	for _, bz := range txs {
		tx, err := a.decoder(bz)
		if err != nil {
			continue // non-sdk txs (e.g. vote extensions) have no mempool entry
		}
		for _, s := range a.signers(tx) {
			senders[s] = struct{}{}
		}
	}
	a.pendingMu.Lock()
	a.pending = senders
	a.pendingMu.Unlock()
}

// RecheckTxs evicts pool txs invalidated by the last block: those whose
// TimeoutHeight has passed (any sender), and those of senders touched by the
// block that now fail the AnteHandler in ReCheck mode.
func (a *Admitter) RecheckTxs() {
	if a.mpool == nil {
		return
	}
	a.pendingMu.Lock()
	pending := a.pending
	height := a.lastCommittedHeight
	a.pending = nil
	a.pendingMu.Unlock()
	// Nothing to do before the first committed block (height 0) with no pending
	// senders. In steady state height > 0, so the sweep always scans.
	if len(pending) == 0 && height == 0 {
		return
	}

	snapshot := PoolSnapshot(context.Background(), a.mpool)

	var (
		candidates     []sdk.Tx
		expiredEvicted float32
	)
	for _, tx := range snapshot {
		if txExpired(tx, height) {
			_ = a.mpool.Remove(tx)
			a.encCache.Evict(tx)
			expiredEvicted++
			continue
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
		if a.maxRecheckBatch > 0 && len(candidates) >= a.maxRecheckBatch {
			break
		}
	}
	if expiredEvicted > 0 {
		telemetry.IncrCounter(expiredEvicted, "cronos", "mempool", "recheck", "expired") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
	}
	if len(candidates) == 0 {
		return
	}

	// RunTx(ReCheck) mutates checkState; serialize against admission and the
	// post-Commit reset for the batch only.
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
		telemetry.IncrCounter(evicted, "cronos", "mempool", "recheck", "evicted") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
	}
}

func txExpired(tx sdk.Tx, committedHeight int64) bool {
	t, ok := tx.(sdk.TxWithTimeoutHeight)
	if !ok {
		return false
	}
	th := t.GetTimeoutHeight()
	return th > 0 && uint64(committedHeight) >= th
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

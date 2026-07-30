package mempool

import (
	"context"
	"sync"
	"time"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// Manager owns the app-side mempool for mempool.type=app. It is a facade over
// the two halves — admission and recheck — which share only txExec.
type Manager struct {
	exec  *txExec
	adm   *admitter
	sched *recheckScheduler
}

// NewManager builds the Manager for mempool.type=app;
func NewManager(app *baseapp.BaseApp, encCache *EncoderCache, txEncoder sdk.TxEncoder, mpool sdkmempool.Mempool, signer sdkmempool.SignerExtractionAdapter, decoder sdk.TxDecoder, recheckBatchSize int, ttlNumBlocks int64, recheckDisabled bool) *Manager {
	a := newManager(app, encCache, txEncoder, decoder)
	a.adm.trace = app.Trace()
	a.sched.mpool = mpool
	a.sched.signer = signer
	a.sched.maxRecheckBatch = recheckBatchSize
	a.sched.ttlNumBlocks = ttlNumBlocks
	a.sched.recheckDisabled = recheckDisabled
	// Left unrefreshed here: NewManager runs inside baseAppOptions, before
	// LoadLatestVersion, so branching now would read an unloaded store. state.base
	// stays nil until App wires the first RefreshMempoolStateLocked call after
	// LoadLatestVersion succeeds; store() falling back to nil (checkState) until
	// then is the correct degradation.
	a.exec.state = &mempoolState{provider: app.CommitMultiStore}
	recheckEnabledGauge := float32(0)
	if !recheckDisabled {
		recheckEnabledGauge = 1
	}
	telemetry.SetGauge(recheckEnabledGauge, "cronos", "mempool", "recheck", "enabled")
	a.sched.worker = newRecheckWorker(a.sched.RecheckTxs)
	a.sched.worker.start()
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
	exec := &txExec{
		runner:    runner,
		encCache:  encCache,
		txEncoder: txEncoder,
		decoder:   decoder,
	}
	return &Manager{
		exec:  exec,
		adm:   &admitter{exec: exec},
		sched: &recheckScheduler{exec: exec},
	}
}

// AdmissionMutex exposes the admission mutex so App.Commit can serialize
// BaseApp.Commit() and the mempoolState refresh against admission and recheck.
func (a *Manager) AdmissionMutex() *sync.Mutex {
	return &a.exec.mu
}

// RefreshMempoolStateLocked rebranches the mempool state off the freshly
// committed store. Precondition: the caller holds AdmissionMutex, which
// App.Commit does across BaseApp.Commit() and this call.
func (a *Manager) RefreshMempoolStateLocked() {
	a.exec.refreshLocked()
}

// SetPreVerify sets the pre-verification hook.
func (a *Manager) SetPreVerify(fn func([]byte) error) {
	a.adm.preVerify = fn
}

func (a *Manager) InsertTxHandler() sdk.InsertTxHandler {
	return a.adm.insertTxHandler()
}

func (a *Manager) CheckTxHandler() sdk.CheckTxHandler {
	return a.adm.checkTxHandler()
}

// InsertTx returns the sync ABCI result; error is always nil (failures surface as ABCI codes).
func (a *Manager) InsertTx(txBytes []byte) (*sdk.TxResponse, error) {
	code, codespace, log := a.adm.admit(txBytes)
	return &sdk.TxResponse{Code: code, Codespace: codespace, RawLog: log}, nil
}

func (a *Manager) PendingTxs() []sdk.Tx {
	if a.sched.mpool == nil {
		return nil
	}
	return PoolSnapshot(context.Background(), a.sched.mpool)
}

func (a *Manager) CountTx() int {
	if a.sched.mpool == nil {
		return 0
	}
	return a.sched.mpool.CountTx()
}

// RecheckDisabled reports whether mempool recheck is disabled
func (a *Manager) RecheckDisabled() bool {
	return a.sched.recheckDisabled
}

func (a *Manager) StageRecheckSenders(height int64, txs [][]byte) {
	a.sched.stageRecheckSenders(height, txs)
}

func (a *Manager) StageSkippedSenders(txs [][]byte) {
	a.sched.stageSkippedSenders(txs)
}

// TriggerRecheck schedules an async recheck.
// Call only from the consensus path (App.Commit).
func (a *Manager) TriggerRecheck() {
	a.sched.triggerRecheck()
}

func (a *Manager) RecheckTxs() {
	a.sched.RecheckTxs()
}

// Close stops the recheck worker.
func (a *Manager) Close() {
	a.sched.worker.stop()
}

// WaitForRecheck blocks until the pending recheck finishes;
func (a *Manager) WaitForRecheck(ctx context.Context) {
	if a.sched.worker.trigger == nil {
		return
	}
	a.sched.worker.wait(ctx)
}

// WaitForRecheckTimedOut is WaitForRecheck bounded by timeout, reporting whether the
// timeout was hit.
func (a *Manager) WaitForRecheckTimedOut(ctx context.Context, timeout time.Duration) bool {
	if a.sched.worker.trigger == nil {
		return false
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return a.sched.worker.wait(waitCtx)
}

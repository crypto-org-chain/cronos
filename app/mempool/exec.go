package mempool

import (
	abci "github.com/cometbft/cometbft/abci/types"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/baseapp"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type txRunner interface {
	RunTx(mode sdk.ExecMode, txBytes []byte, tx sdk.Tx, txIndex int, txMultiStore storetypes.MultiStore, incarnationCache map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error)
}

var (
	_ txRunner = (*baseapp.BaseApp)(nil)
	// committedHeight relies on this to detect a Commit during recheck.
	_ interface{ LastBlockHeight() int64 } = (*baseapp.BaseApp)(nil)
)

// txExec is what admission and recheck share.
type txExec struct {
	// gate serializes every RunTx on checkState, the pending-nonce view admission
	// and recheck share, and replaces the no-op AppMempool.Lock(). App.Commit
	// holds it across BaseApp.Commit() and goes ahead of queued holders. Recheck
	// holds it per chunk, cascade eviction included, never across the pool scan.
	gate      admissionGate
	runner    txRunner
	encCache  *EncoderCache
	txEncoder sdk.TxEncoder
	decoder   sdk.TxDecoder
	// pending avoids re-walking the pool on every PendingTxs() call.
	pending pendingTxCache
	logger  log.Logger
}

// runTxLocked runs tx against checkState. Precondition: the caller holds gate.
func (e *txExec) runTxLocked(mode sdk.ExecMode, bz []byte, tx sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	return e.runner.RunTx(mode, bz, tx, -1, nil, nil)
}

// committedHeight is the last committed height, or 0 if the runner can't report
// it. Stable while gate is held.
func (e *txExec) committedHeight() int64 {
	if h, ok := e.runner.(interface{ LastBlockHeight() int64 }); ok {
		return h.LastBlockHeight()
	}
	return 0
}

package mempool

import (
	"sync"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type txRunner interface {
	RunTx(mode sdk.ExecMode, txBytes []byte, tx sdk.Tx, txIndex int, txMultiStore storetypes.MultiStore, incarnationCache map[string]any) (sdk.GasInfo, *sdk.Result, []abci.Event, error)
}

var _ txRunner = (*baseapp.BaseApp)(nil)

// txExec is what admission and recheck share: the RunTx entry point, the
// codecs both need, and the PendingTxs snapshot cache both invalidate.
type txExec struct {
	// mu serializes every RunTx against baseapp's checkState, the pending-nonce
	// view admission and recheck share: a CheckTx/ReCheckTx ante write-back is
	// what lets a later higher-nonce sibling pass. AppMempool.Lock() is a no-op,
	// so mu also replaces the mempool lock BaseApp normally relies on, and
	// App.Commit holds it across BaseApp.Commit() so the checkState reset never
	// races a RunTx. Held around one recheck chunk (its RunTx calls plus the
	// cascade eviction that follows a nonce gap proven in that chunk) so eviction
	// stays atomic with admission; never held across the lock-free pool scan.
	mu        sync.Mutex
	runner    txRunner
	encCache  *EncoderCache
	txEncoder sdk.TxEncoder
	decoder   sdk.TxDecoder
	// pending avoids re-walking the pool on every PendingTxs() call.
	pending pendingTxCache
}

// runTxLocked runs tx against checkState. Precondition: the caller holds mu.
func (e *txExec) runTxLocked(mode sdk.ExecMode, bz []byte, tx sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	return e.runner.RunTx(mode, bz, tx, -1, nil, nil)
}

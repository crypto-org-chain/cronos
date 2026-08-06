package mempool

import (
	"sync"
	"sync/atomic"

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
// branched state both run against, and the codecs both need.
type txExec struct {
	// mu guards state.base, the shared nonce authority for admission and recheck:
	// RunTx is serialized through it, and App.Commit holds it across
	// BaseApp.Commit() plus the post-Commit refresh so the swap never races a
	// RunTx reader or the live memiavl tree mid-Commit. AppMempool.Lock() is a
	// no-op, so mu also replaces the mempool lock BaseApp normally relies on.
	// Held around RunTx and the cascade eviction that follows a proven nonce
	// gap in the same chunk, so eviction stays atomic with admission; never
	// held across the lock-free pool scan.
	mu     sync.Mutex
	runner txRunner
	// state holds the CacheMultiStore branch RunTx uses in place of checkState.
	// nil until the first refreshLocked call (after LoadLatestVersion in
	// production, or never in the newManager() test constructor); store() returns
	// nil in the meantime so RunTx's 5th arg falls back to checkState.
	state *mempoolState
	// gen counts state refreshes; a recheck pass abandons its remaining groups
	// once gen advances, since they were selected against a superseded base.
	gen       atomic.Uint64
	encCache  *EncoderCache
	txEncoder sdk.TxEncoder
	decoder   sdk.TxDecoder
}

// runTxLocked runs tx against the mempool's own branch instead of checkState.
// Precondition: the caller holds mu.
func (e *txExec) runTxLocked(mode sdk.ExecMode, bz []byte, tx sdk.Tx) (sdk.GasInfo, *sdk.Result, []abci.Event, error) {
	return e.runner.RunTx(mode, bz, tx, -1, e.state.store(), nil)
}

// refreshLocked rebranches off the freshly committed store and bumps gen,
// canceling any recheck pass still validating against the superseded base.
// Precondition: the caller holds mu.
func (e *txExec) refreshLocked() {
	if e.state == nil {
		return
	}
	e.state.refreshLocked()
	e.gen.Add(1)
}

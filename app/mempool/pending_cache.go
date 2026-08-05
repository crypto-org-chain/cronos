package mempool

import (
	"sync"
	"sync/atomic"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// pendingCache caches a PendingTxs() snapshot so concurrent RPC readers
// single-flight onto one pool walk instead of one each. Invalidated purely on
// tx admission and block completion (see Manager.admit, CheckTxHandler,
// StageRecheckSenders) — no TTL.
type pendingCache struct {
	mu          sync.Mutex
	enabled     bool
	snapshot    []sdk.Tx
	loaded      bool
	loadedEpoch uint64
	epoch       atomic.Uint64
}

func (c *pendingCache) get(load func() []sdk.Tx) []sdk.Tx {
	if !c.enabled {
		return load()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Read epoch before the walk, commit loadedEpoch after: an invalidate
	// racing the walk is never swallowed.
	epoch := c.epoch.Load()
	if c.loaded && c.loadedEpoch == epoch {
		telemetry.IncrCounter(1, "cronos", "mempool", "pending", "cache", "hit")
		return c.copySnapshot()
	}

	telemetry.IncrCounter(1, "cronos", "mempool", "pending", "cache", "miss")
	c.snapshot, c.loaded, c.loadedEpoch = load(), true, epoch
	return c.copySnapshot()
}

func (c *pendingCache) copySnapshot() []sdk.Tx {
	out := make([]sdk.Tx, len(c.snapshot))
	copy(out, c.snapshot)
	return out
}

// invalidate marks the snapshot stale. Lock-free: admit()/CheckTxHandler hold
// a.mu while calling this, and StageRecheckSenders runs on the consensus
// path — neither may block on c.mu, which get() can hold for a full pool walk.
func (c *pendingCache) invalidate() {
	c.epoch.Add(1)
}

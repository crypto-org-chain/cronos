package mempool

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// pendingCache TTL-caches a PendingTxs() snapshot so concurrent RPC readers
// single-flight onto one pool walk instead of one each.
type pendingCache struct {
	mu          sync.Mutex
	ttl         time.Duration
	snapshot    []sdk.Tx
	expiry      time.Time
	loadedEpoch uint64
	epoch       atomic.Uint64
	now         func() time.Time
}

func (c *pendingCache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func (c *pendingCache) get(load func() []sdk.Tx) []sdk.Tx {
	if c.ttl <= 0 {
		return load()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock()
	epoch := c.epoch.Load()

	if !c.expiry.IsZero() && now.Before(c.expiry) && c.loadedEpoch == epoch {
		telemetry.IncrCounter(1, "cronos", "mempool", "pending", "cache", "hit")
		return c.copySnapshot()
	}

	telemetry.IncrCounter(1, "cronos", "mempool", "pending", "cache", "miss")

	c.snapshot = load()
	c.expiry = now.Add(c.ttl)
	c.loadedEpoch = epoch
	return c.copySnapshot()
}

func (c *pendingCache) copySnapshot() []sdk.Tx {
	out := make([]sdk.Tx, len(c.snapshot))
	copy(out, c.snapshot)
	return out
}

// invalidate marks the snapshot stale on every block boundary
func (c *pendingCache) invalidate() {
	c.epoch.Add(1)
}

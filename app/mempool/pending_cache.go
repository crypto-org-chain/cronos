package mempool

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// pendingCache TTL-caches a PendingTxs() snapshot so a burst of concurrent RPC
// readers doesn't re-walk the pool once per call.
type pendingCache struct {
	mu       sync.Mutex
	ttl      time.Duration
	snapshot []sdk.Tx
	expiry   time.Time
	// loadedEpoch is the epoch the cached snapshot was taken in; a mismatch with
	// epoch means the snapshot predates an invalidation and must be reloaded.
	loadedEpoch uint64
	epoch       atomic.Uint64
	// now is injectable for deterministic tests; nil means time.Now.
	now func() time.Time
}

func (c *pendingCache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// get returns the cached snapshot, reloading it via load when the TTL has passed
// or the cache was invalidated. ttl <= 0 disables caching: every call runs load.
// The mutex spans load, not just the freshness check, so concurrent callers
// single-flight onto one pool walk instead of each racing their own.
func (c *pendingCache) get(load func() []sdk.Tx) []sdk.Tx {
	if c.ttl <= 0 {
		return load()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock()
	epoch := c.epoch.Load()
	// A zero expiry means nothing has been loaded yet. An empty pool caches a valid
	// zero-length snapshot, so neither length nor nil-ness can stand in for that.
	if !c.expiry.IsZero() && now.Before(c.expiry) && c.loadedEpoch == epoch {
		telemetry.IncrCounter(1, "cronos", "mempool", "pending", "cache", "hit")
		return c.snapshot
	}

	telemetry.IncrCounter(1, "cronos", "mempool", "pending", "cache", "miss")
	// Read the epoch before load, so an invalidation racing this walk marks the
	// result stale rather than being swallowed.
	c.snapshot = load()
	c.expiry = now.Add(c.ttl)
	c.loadedEpoch = epoch
	return c.snapshot
}

// invalidate marks the cached snapshot stale. Needed at every block boundary:
// committed txs are already out of the pool, and reporting them as still pending
// over-counts a sender's pending nonce, so the client submits a gapped nonce that
// this chain rejects outright.
//
// Atomic rather than taking mu, because the caller runs on the consensus path and
// mu can be held for a full pool walk by an RPC reader.
func (c *pendingCache) invalidate() {
	c.epoch.Add(1)
}

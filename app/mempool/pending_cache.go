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
	mu       sync.Mutex
	ttl      time.Duration
	snapshot []sdk.Tx
	expiry   time.Time
	// loadedEpoch mismatching epoch means the snapshot predates an invalidation.
	loadedEpoch uint64
	epoch       atomic.Uint64
	now         func() time.Time // injectable clock for tests; nil means time.Now
}

func (c *pendingCache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// get returns a copy of the cached snapshot, reloading via load when the TTL
// lapsed or the cache was invalidated. ttl <= 0 disables caching (every call
// runs load). The mutex spans load so concurrent callers single-flight onto
// one pool walk.
func (c *pendingCache) get(load func() []sdk.Tx) []sdk.Tx {
	if c.ttl <= 0 {
		return load()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock()
	epoch := c.epoch.Load()
	// Zero expiry means never loaded; an empty pool still caches a valid
	// zero-length snapshot, so length/nil-ness can't serve as that sentinel.
	if !c.expiry.IsZero() && now.Before(c.expiry) && c.loadedEpoch == epoch {
		telemetry.IncrCounter(1, "cronos", "mempool", "pending", "cache", "hit")
		return c.copySnapshot()
	}

	telemetry.IncrCounter(1, "cronos", "mempool", "pending", "cache", "miss")
	// Epoch read before load: an invalidation racing this walk is caught by the
	// next call, not this one (this call's result was already in flight).
	c.snapshot = load()
	c.expiry = now.Add(c.ttl)
	c.loadedEpoch = epoch
	return c.copySnapshot()
}

// copySnapshot prevents a mutating caller from corrupting the slice shared
// with other concurrent readers for the rest of the TTL window.
func (c *pendingCache) copySnapshot() []sdk.Tx {
	out := make([]sdk.Tx, len(c.snapshot))
	copy(out, c.snapshot)
	return out
}

// invalidate marks the snapshot stale on every block boundary — otherwise a
// committed tx still reads as pending, over-counting the sender's nonce.
// Atomic, not mu: the caller runs on the consensus path and mu can be held
// for a full pool walk by an RPC reader.
func (c *pendingCache) invalidate() {
	c.epoch.Add(1)
}

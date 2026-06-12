package mempool

import (
	"sync"
	"time"

	cmdcfg "github.com/crypto-org-chain/cronos/cmd/cronosd/config"
)

// gossipTracker throttles the gossip reap so the AppReactor stops re-broadcasting
// the whole pool every ReapInterval. It records each tx's last gossip time (by
// canonical-bytes hash) and suppresses re-reap until ttl elapses.
type gossipTracker struct {
	mu       sync.Mutex
	seen     map[[32]byte]int64 // canonical-bytes hash -> last gossip (unix nanos)
	ttlNanos int64
}

// newGossipTracker builds a tracker. ttl <= 0 falls back to the default (mirrors
// NewEncoderCache): a zero TTL would re-gossip the whole pool every tick, the
// exact flood this guards against.
func newGossipTracker(ttl time.Duration) *gossipTracker {
	if ttl <= 0 {
		ttl = cmdcfg.DefaultMempoolGossipTTL
	}
	return &gossipTracker{
		seen:     make(map[[32]byte]int64),
		ttlNanos: ttl.Nanoseconds(),
	}
}

// gossiped reports whether tx h may be gossiped now, recording the time when
// it may. Returns false if h was gossiped within the last ttl. Caller passes a
// single now per reap so all txs in one scan share a timestamp.
func (g *gossipTracker) gossiped(h [32]byte, now int64) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if last, ok := g.seen[h]; ok && now-last < g.ttlNanos {
		return false
	}
	g.seen[h] = now
	return true
}

// prune drops entries past ttl (eligible for re-gossip anyway), bounding the map
// to txs gossiped within the ttl window. Called once per reap.
func (g *gossipTracker) prune(now int64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for h, last := range g.seen {
		if now-last >= g.ttlNanos {
			delete(g.seen, h)
		}
	}
}

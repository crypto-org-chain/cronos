package mempool

import (
	"testing"
	"time"

	cmdcfg "github.com/crypto-org-chain/cronos/cmd/cronosd/config"
)

func TestGossipTracker_DedupWithinTTL(t *testing.T) {
	g := newGossipTracker(time.Second)
	h := [32]byte{1}
	now := int64(1_000)
	if !g.gossip(h, now) {
		t.Fatal("first gossip should be allowed")
	}
	if g.gossip(h, now) {
		t.Fatal("repeat within ttl should be suppressed")
	}
}

func TestGossipTracker_RegossipAfterTTL(t *testing.T) {
	g := newGossipTracker(time.Second)
	h := [32]byte{2}
	base := int64(1_000_000_000)
	if !g.gossip(h, base) {
		t.Fatal("first allowed")
	}
	if g.gossip(h, base+int64(time.Second)-1) {
		t.Fatal("just under ttl should still suppress")
	}
	if !g.gossip(h, base+int64(time.Second)) {
		t.Fatal("at/after ttl should re-allow re-gossip")
	}
}

func TestGossipTracker_Prune(t *testing.T) {
	g := newGossipTracker(time.Second)
	fresh, stale := [32]byte{1}, [32]byte{2}
	now := 10 * int64(time.Second)
	g.gossip(stale, now-2*int64(time.Second)) // older than ttl
	g.gossip(fresh, now)
	g.prune(now)
	if _, ok := g.seen[stale]; ok {
		t.Fatal("stale entry should be pruned")
	}
	if _, ok := g.seen[fresh]; !ok {
		t.Fatal("fresh entry should remain")
	}
}

func TestGossipTracker_TTLDefaultsWhenNonPositive(t *testing.T) {
	g := newGossipTracker(0)
	if want := cmdcfg.DefaultMempoolGossipTTL.Nanoseconds(); g.ttlNanos != want {
		t.Fatalf("ttlNanos = %d, want default %d", g.ttlNanos, want)
	}
}

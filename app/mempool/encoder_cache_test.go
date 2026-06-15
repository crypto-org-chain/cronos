package mempool

import (
	"crypto/sha256"
	"testing"
)

func TestEncoderCache_EvictsAtCapacity(t *testing.T) {
	const cap = 4
	c := NewEncoderCache(cap, 0)

	txs := make([]*ptrTx, 0, cap*3)
	for i := 0; i < cap*3; i++ {
		tx := &ptrTx{id: i}
		txs = append(txs, tx)
		c.Set(tx, []byte{byte(i)})
		if got := c.lru.Len(); got > cap {
			t.Fatalf("after %d inserts lru.Len()=%d exceeds cap %d", i+1, got, cap)
		}
	}

	if got := len(c.items); got != cap {
		t.Fatalf("items=%d, want cap %d (unbounded growth)", got, cap)
	}
	if got := c.lru.Len(); got != cap {
		t.Fatalf("lru.Len()=%d, want cap %d", got, cap)
	}

	// Only the last cap insertions survive; everything older was evicted.
	for _, tx := range txs[:len(txs)-cap] {
		if _, ok := c.Get(tx); ok {
			t.Fatalf("tx id=%d should have been evicted", tx.id)
		}
	}
	for _, tx := range txs[len(txs)-cap:] {
		if _, ok := c.Get(tx); !ok {
			t.Fatalf("tx id=%d should still be cached", tx.id)
		}
	}
}

func TestEncoderCache_SkipsOversizedBytes(t *testing.T) {
	const maxTxBytes = 8
	c := NewEncoderCache(4, maxTxBytes)

	big := &ptrTx{id: 1}
	c.Set(big, make([]byte, maxTxBytes+1)) // over ceiling → not cached
	if _, ok := c.Get(big); ok {
		t.Fatal("oversized tx should not be cached")
	}

	small := &ptrTx{id: 2}
	c.Set(small, make([]byte, maxTxBytes)) // at ceiling → cached
	if _, ok := c.Get(small); !ok {
		t.Fatal("tx at the ceiling should be cached")
	}
}

func TestEncoderCache_LRUPromotesOnRead(t *testing.T) {
	const cap = 2
	c := NewEncoderCache(cap, 0)

	a, b := &ptrTx{id: 1}, &ptrTx{id: 2}
	c.Set(a, []byte{1})
	c.Set(b, []byte{2})

	// Touch a so it becomes MRU; b is now the LRU victim.
	if _, ok := c.Get(a); !ok {
		t.Fatal("a missing before promotion")
	}

	d := &ptrTx{id: 3}
	c.Set(d, []byte{3}) // evicts b, not a

	if _, ok := c.Get(a); !ok {
		t.Fatal("a was evicted despite recent access")
	}
	if _, ok := c.Get(b); ok {
		t.Fatal("b should have been evicted as LRU")
	}
	if _, ok := c.Get(d); !ok {
		t.Fatal("d missing after insert")
	}
}

func TestEncoderCache_NilReceiverMisses(t *testing.T) {
	var nilCache *EncoderCache
	if _, ok := nilCache.Get(&ptrTx{id: 1}); ok {
		t.Fatal("nil *EncoderCache must miss, not hit")
	}
}

func TestEncoderCache_ReRegisterUpdatesBytes(t *testing.T) {
	c := NewEncoderCache(4, 0)
	tx := &ptrTx{id: 1}
	c.Set(tx, []byte{1})
	c.Set(tx, []byte{2, 2})

	if got := len(c.items); got != 1 {
		t.Fatalf("items=%d, want 1 (duplicate pointer should not add an entry)", got)
	}
	got, ok := c.Get(tx)
	if !ok || len(got) != 2 {
		t.Fatalf("re-register did not overwrite bytes: got=%v ok=%v", got, ok)
	}
}

func TestEncoderCache_Evict(t *testing.T) {
	c := NewEncoderCache(1, 0)
	a, b := &ptrTx{id: 1}, &ptrTx{id: 2}
	c.Set(a, []byte{1})
	c.Set(b, []byte{2}) // evicts a (cap 1)

	// Evict on an entry already LRU-evicted is a safe no-op, leaving b intact.
	c.Evict(a)
	if _, ok := c.Get(b); !ok {
		t.Fatal("Evict of an absent tx must not disturb live entries")
	}

	// Evict a live entry removes it from both the map and the LRU list.
	c.Evict(b)
	if _, ok := c.Get(b); ok {
		t.Fatal("evicted tx must miss")
	}
	if len(c.items) != 0 || c.lru.Len() != 0 {
		t.Fatalf("after Evict items=%d lru=%d, want 0/0", len(c.items), c.lru.Len())
	}

	var nilCache *EncoderCache
	nilCache.Evict(a) // nil receiver: no panic
}

func TestEncoderCache_GetReturnsCopy(t *testing.T) {
	c := NewEncoderCache(4, 0)
	tx := &ptrTx{id: 1}
	c.Set(tx, []byte("canonical"))

	got, ok := c.Get(tx)
	if !ok {
		t.Fatal("tx missing")
	}
	got[0] = 'X' // mutate the returned slice; must not touch the cache

	again, _ := c.Get(tx)
	if string(again) != "canonical" {
		t.Fatalf("Get leaked its backing array: cache now %q, want %q", again, "canonical")
	}
}

func TestEncoderCache_HashTx(t *testing.T) {
	c := NewEncoderCache(4, 0)
	tx := &ptrTx{id: 1}
	bz := []byte("canonical")
	want := sha256.Sum256(bz)

	c.Set(tx, bz)
	if got := c.HashTx(tx, bz); got != want {
		t.Fatalf("HashTx=%x, want %x", got, want)
	}
	// Cached after first call.
	if it := c.items[tx].Value.(*item); !it.hashed || it.hash != want {
		t.Fatalf("hash not cached: hashed=%v hash=%x", it.hashed, it.hash)
	}

	// Re-Set with new bytes must invalidate the cached hash.
	bz2 := []byte("different")
	want2 := sha256.Sum256(bz2)
	c.Set(tx, bz2)
	if it := c.items[tx].Value.(*item); it.hashed {
		t.Fatal("Set must clear cached hash when bytes change")
	}
	if got := c.HashTx(tx, bz2); got != want2 {
		t.Fatalf("HashTx after re-Set=%x, want %x", got, want2)
	}

	// Uncached tx (and nil receiver) hash directly without caching.
	uncached := &ptrTx{id: 2}
	if got := c.HashTx(uncached, bz); got != want {
		t.Fatalf("uncached HashTx=%x, want %x", got, want)
	}
	if _, ok := c.items[uncached]; ok {
		t.Fatal("HashTx must not insert entries for uncached txs")
	}
	var nilCache *EncoderCache
	if got := nilCache.HashTx(tx, bz); got != want {
		t.Fatalf("nil HashTx=%x, want %x", got, want)
	}
}

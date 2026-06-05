package mempool

import "testing"

func TestEncoderCache_EvictsAtCapacity(t *testing.T) {
	const cap = 4
	c := NewEncoderCache(cap)

	txs := make([]*ptrTx, 0, cap*3)
	for i := 0; i < cap*3; i++ {
		tx := &ptrTx{id: i}
		txs = append(txs, tx)
		c.Register(tx, []byte{byte(i)})
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
		if _, ok := c.Bytes(tx); ok {
			t.Fatalf("tx id=%d should have been evicted", tx.id)
		}
	}
	for _, tx := range txs[len(txs)-cap:] {
		if _, ok := c.Bytes(tx); !ok {
			t.Fatalf("tx id=%d should still be cached", tx.id)
		}
	}
}

func TestEncoderCache_LRUPromotesOnRead(t *testing.T) {
	const cap = 2
	c := NewEncoderCache(cap)

	a, b := &ptrTx{id: 1}, &ptrTx{id: 2}
	c.Register(a, []byte{1})
	c.Register(b, []byte{2})

	// Touch a so it becomes MRU; b is now the LRU victim.
	if _, ok := c.Bytes(a); !ok {
		t.Fatal("a missing before promotion")
	}

	d := &ptrTx{id: 3}
	c.Register(d, []byte{3}) // evicts b, not a

	if _, ok := c.Bytes(a); !ok {
		t.Fatal("a was evicted despite recent access")
	}
	if _, ok := c.Bytes(b); ok {
		t.Fatal("b should have been evicted as LRU")
	}
	if _, ok := c.Bytes(d); !ok {
		t.Fatal("d missing after insert")
	}
}

func TestEncoderCache_NilReceiverMisses(t *testing.T) {
	var nilCache *EncoderCache
	if _, ok := nilCache.Bytes(&ptrTx{id: 1}); ok {
		t.Fatal("nil *EncoderCache must miss, not hit")
	}
}

func TestEncoderCache_ReRegisterUpdatesBytes(t *testing.T) {
	c := NewEncoderCache(4)
	tx := &ptrTx{id: 1}
	c.Register(tx, []byte{1})
	c.Register(tx, []byte{2, 2})

	if got := len(c.items); got != 1 {
		t.Fatalf("items=%d, want 1 (duplicate pointer should not add an entry)", got)
	}
	got, ok := c.Bytes(tx)
	if !ok || len(got) != 2 {
		t.Fatalf("re-register did not overwrite bytes: got=%v ok=%v", got, ok)
	}
}

func TestEncoderCache_Evict(t *testing.T) {
	c := NewEncoderCache(1)
	a, b := &ptrTx{id: 1}, &ptrTx{id: 2}
	c.Register(a, []byte{1})
	c.Register(b, []byte{2}) // evicts a (cap 1)

	// Evict on an entry already LRU-evicted is a safe no-op, leaving b intact.
	c.Evict(a)
	if _, ok := c.Bytes(b); !ok {
		t.Fatal("Evict of an absent tx must not disturb live entries")
	}

	// Evict a live entry removes it from both the map and the LRU list.
	c.Evict(b)
	if _, ok := c.Bytes(b); ok {
		t.Fatal("evicted tx must miss")
	}
	if len(c.items) != 0 || c.lru.Len() != 0 {
		t.Fatalf("after Evict items=%d lru=%d, want 0/0", len(c.items), c.lru.Len())
	}

	var nilCache *EncoderCache
	nilCache.Evict(a) // nil receiver: no panic
}

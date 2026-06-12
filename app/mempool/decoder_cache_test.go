package mempool

import (
	"container/list"
	"encoding/binary"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cespare/xxhash/v2"
	cmdcfg "github.com/crypto-org-chain/cronos/cmd/cronosd/config"
	protov2 "google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// cacheTx is a minimal sdk.Tx stub used in decode cache tests.
type cacheTx struct{ id uint64 }

func (c *cacheTx) GetMsgs() []sdk.Msg                    { return nil }
func (c *cacheTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

func makeRaw(id uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, id)
	return b
}

func makeDecoder(t *testing.T) (sdk.TxDecoder, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	dec := func(bz []byte) (sdk.Tx, error) {
		calls.Add(1)
		if len(bz) < 8 {
			return nil, errors.New("too short")
		}
		return &cacheTx{id: binary.LittleEndian.Uint64(bz)}, nil
	}
	return dec, &calls
}

func TestDecodeCache_HitAndMiss(t *testing.T) {
	base, calls := makeDecoder(t)
	c := NewDecodeCache(cmdcfg.DefaultTxCacheSize, cmdcfg.DefaultTxCacheMaxTxBytes)
	dec := NewCachingDecoder(base, c)

	raw := makeRaw(42)

	tx1, err := dec(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("base calls = %d, want 1", got)
	}
	if got := tx1.(*cacheTx).id; got != 42 {
		t.Fatalf("first decode tx.id = %d, want 42", got)
	}

	tx2, err := dec(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("base calls = %d after cache hit, want 1", got)
	}
	if tx1 != tx2 {
		t.Fatal("cache returned different tx pointer on hit")
	}
	if got := tx2.(*cacheTx).id; got != 42 {
		t.Fatalf("cached tx.id = %d, want 42 (cache returned wrong tx)", got)
	}
}

func TestDecodeCache_ErrorNotCached(t *testing.T) {
	base, calls := makeDecoder(t)
	c := NewDecodeCache(cmdcfg.DefaultTxCacheSize, cmdcfg.DefaultTxCacheMaxTxBytes)
	dec := NewCachingDecoder(base, c)

	bad := []byte{1, 2} // too short → error from base

	if _, err := dec(bad); err == nil {
		t.Fatal("expected error on first call")
	}
	if _, err := dec(bad); err == nil {
		t.Fatal("expected error on retry")
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("base calls = %d, want 2 (errors must not be cached)", got)
	}
}

func TestDecodeCache_DifferentPayloads(t *testing.T) {
	base, calls := makeDecoder(t)
	c := NewDecodeCache(cmdcfg.DefaultTxCacheSize, cmdcfg.DefaultTxCacheMaxTxBytes)
	dec := NewCachingDecoder(base, c)

	const n = 10
	for i := uint64(0); i < n; i++ {
		if _, err := dec(makeRaw(i)); err != nil {
			t.Fatalf("id %d: %v", i, err)
		}
	}
	if got := calls.Load(); got != n {
		t.Fatalf("base calls = %d, want %d (each distinct key is a miss)", got, n)
	}
	for i := uint64(0); i < n; i++ {
		if _, err := dec(makeRaw(i)); err != nil {
			t.Fatalf("re-decode id %d: %v", i, err)
		}
	}
	if got := calls.Load(); got != n {
		t.Fatalf("base calls = %d after re-decode, want still %d (all hits)", got, n)
	}
}

func TestDecodeCache_LRUEviction(t *testing.T) {
	const shardCacheSize = cmdcfg.DefaultTxCacheSize / shardCount
	s := &cacheShard{cap: shardCacheSize, items: make(map[uint64]*list.Element, shardCacheSize)}

	// Fill shard to capacity with keys 0..shardCacheSize-1.
	for i := 0; i < shardCacheSize; i++ {
		bz := makeRaw(uint64(i))
		s.set(xxhash.Sum64(bz), bz, &cacheTx{id: uint64(i)})
	}

	// Access key 0 — promotes it to MRU.
	h0 := xxhash.Sum64(makeRaw(0))
	if _, ok := s.get(h0, makeRaw(0)); !ok {
		t.Fatal("key 0 missing before eviction test")
	}

	// Insert one new key. Shard is full, so LRU (key 1, never re-accessed)
	// must be evicted, NOT key 0 (MRU).
	newBz := makeRaw(uint64(shardCacheSize))
	s.set(xxhash.Sum64(newBz), newBz, &cacheTx{id: uint64(shardCacheSize)})

	if _, ok := s.get(h0, makeRaw(0)); !ok {
		t.Fatal("LRU evicted key 0 (recently used); should have evicted key 1")
	}
	h1 := xxhash.Sum64(makeRaw(1))
	if _, ok := s.get(h1, makeRaw(1)); ok {
		t.Fatal("key 1 (LRU) should have been evicted")
	}
}

func countEntries(c *DecodeCache) int {
	total := 0
	for i := range c.shards {
		c.shards[i].mu.Lock()
		total += len(c.shards[i].items)
		c.shards[i].mu.Unlock()
	}
	return total
}

func TestDecodeCache_EvictionBounded(t *testing.T) {
	base, _ := makeDecoder(t)
	c := NewDecodeCache(cmdcfg.DefaultTxCacheSize, cmdcfg.DefaultTxCacheMaxTxBytes)
	dec := NewCachingDecoder(base, c)

	for i := uint64(0); i < 2*cmdcfg.DefaultTxCacheSize; i++ {
		if _, err := dec(makeRaw(i)); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	got := countEntries(c)
	if got == 0 {
		t.Fatal("cache is empty after inserts — no entries were cached")
	}
	if got > cmdcfg.DefaultTxCacheSize {
		t.Fatalf("cache exceeds capacity: %d entries > %d limit", got, cmdcfg.DefaultTxCacheSize)
	}
}

func TestDecodeCache_Concurrent(t *testing.T) {
	base, calls := makeDecoder(t)
	c := NewDecodeCache(cmdcfg.DefaultTxCacheSize, cmdcfg.DefaultTxCacheMaxTxBytes)
	dec := NewCachingDecoder(base, c)

	const goroutines = 16
	const itersPerG = 200

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < itersPerG; i++ {
				raw := makeRaw(uint64(id*itersPerG + i))
				if _, err := dec(raw); err != nil {
					t.Errorf("goroutine %d iter %d first decode: %v", id, i, err)
				}
				// Second call exercises concurrent cache hits on same key
				if _, err := dec(raw); err != nil {
					t.Errorf("goroutine %d iter %d re-decode: %v", id, i, err)
				}
			}
		}(g)
	}
	wg.Wait()

	// Keys are disjoint across goroutines: ~200 per shard (3200/16), well below
	// each shard's ~625-entry cap, so no eviction. Exactly one miss per unique key.
	want := int64(goroutines * itersPerG)
	if got := calls.Load(); got != want {
		t.Fatalf("cache hit rate wrong: calls=%d, want %d (one miss per unique key)", got, want)
	}
}

func TestDecodeCache_SkipLargePayloads(t *testing.T) {
	calls := atomic.Int64{}
	base := func(bz []byte) (sdk.Tx, error) {
		calls.Add(1)
		return &cacheTx{id: uint64(len(bz))}, nil
	}
	c := NewDecodeCache(cmdcfg.DefaultTxCacheSize, cmdcfg.DefaultTxCacheMaxTxBytes)
	dec := NewCachingDecoder(base, c)

	big := make([]byte, cmdcfg.DefaultTxCacheMaxTxBytes+1)
	for i := 0; i < 3; i++ {
		if _, err := dec(big); err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("oversize payload was cached: calls=%d, want 3 (each call must miss)", got)
	}

	// Boundary: exactly cmdcfg.DefaultTxCacheMaxTxBytes should be cached.
	calls.Store(0)
	atSize := make([]byte, cmdcfg.DefaultTxCacheMaxTxBytes)
	if _, err := dec(atSize); err != nil {
		t.Fatalf("at-size payload first decode: %v", err)
	}
	if _, err := dec(atSize); err != nil {
		t.Fatalf("at-size payload second decode: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("at-size payload not cached: calls=%d, want 1", got)
	}
}

func TestDecodeCache_CustomMaxTxBytes(t *testing.T) {
	const customMax = 32
	calls := atomic.Int64{}
	base := func(bz []byte) (sdk.Tx, error) {
		calls.Add(1)
		return &cacheTx{id: uint64(len(bz))}, nil
	}
	c := NewDecodeCache(cmdcfg.DefaultTxCacheSize, customMax)
	dec := NewCachingDecoder(base, c)

	// Above cap: each call must miss.
	above := make([]byte, customMax+1)
	for i := 0; i < 3; i++ {
		if _, err := dec(above); err != nil {
			t.Fatalf("above-cap iter %d: %v", i, err)
		}
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("above-cap was cached: calls=%d, want 3", got)
	}

	// Default const would cache this; with a custom cap of 32 even 33-byte
	// payloads bypass — proves the field, not the const, governs the gate.
	calls.Store(0)
	atCap := make([]byte, customMax)
	if _, err := dec(atCap); err != nil {
		t.Fatalf("at-cap first decode: %v", err)
	}
	if _, err := dec(atCap); err != nil {
		t.Fatalf("at-cap second decode: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("at-cap was not cached: calls=%d, want 1", got)
	}
}

func TestDecodeCache_DefaultsOnZero(t *testing.T) {
	c := NewDecodeCache(0, 0)
	if c.maxTxBytes != cmdcfg.DefaultTxCacheMaxTxBytes {
		t.Fatalf("maxTxBytes = %d, want default %d", c.maxTxBytes, cmdcfg.DefaultTxCacheMaxTxBytes)
	}
	wantShardCap := (cmdcfg.DefaultTxCacheSize + shardCount - 1) / shardCount
	if got := c.shards[0].cap; got != wantShardCap {
		t.Fatalf("shard cap = %d, want %d (default size / shardCount)", got, wantShardCap)
	}
}

package app

import (
	"bytes"
	"container/list"
	cryptorand "crypto/rand"
	"encoding/binary"
	"sync"

	"github.com/cespare/xxhash/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// xxhashSeed randomizes shard assignment at startup so an attacker cannot
// precompute which tx bytes land in which shard and flood a single shard.
var xxhashSeed = func() uint64 {
	var b [8]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		panic(err)
	}
	return binary.LittleEndian.Uint64(b[:])
}()

const (
	decodeCacheSize = 10_000
	// maxCachedTxBytes caps per-entry raw payload size. The wire-byte
	// footprint ceiling is decodeCacheSize * maxCachedTxBytes (~640 MiB),
	// but the cached value is a fully-decoded sdk.Tx whose heap footprint
	// (proto messages, slices, interface values) can be several times the
	// raw bytes. Operators sizing memory should not rely on this constant
	// as a hard upper bound on RSS impact. Txs above the threshold are
	// decoded normally but not cached, preventing an adversary submitting
	// MaxTxBytes-sized txs from exhausting RAM via the cache.
	maxCachedTxBytes = 64 * 1024

	shardCount     = 16
	shardCacheSize = decodeCacheSize / shardCount // 625 per shard
)

type lruItem struct {
	h   uint64
	key []byte
	tx  sdk.Tx
}

// cacheShard is one stripe of the decodeCache. Each shard has its own
// mutex, so concurrent access to different tx hashes is contention-free.
// Eviction is LRU: the front of the list is most-recently-used.
type cacheShard struct {
	mu    sync.Mutex
	items map[uint64]*list.Element
	lru   list.List // front = MRU, back = LRU; zero value is empty list
}

// get returns the cached tx if h matches and the stored payload bytes equal
// bz. On hash collision (same h, different bz) it returns (nil, false) — a
// silent miss — without evicting the colliding entry. The caller re-decodes
// and calls set, which then evicts the stale entry. With the seeded 64-bit
// xxhash and 625 entries per shard, collisions are rare enough that this
// "let set handle it" path is preferable to forward-eviction in get.
func (s *cacheShard) get(h uint64, bz []byte) (sdk.Tx, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	el, ok := s.items[h]
	if !ok {
		return nil, false
	}
	item := el.Value.(*lruItem)
	if !bytes.Equal(item.key, bz) {
		return nil, false
	}
	s.lru.MoveToFront(el)
	return item.tx, true
}

func (s *cacheShard) set(h uint64, bz []byte, tx sdk.Tx) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if el, ok := s.items[h]; ok {
		item := el.Value.(*lruItem)
		if bytes.Equal(item.key, bz) {
			// Identical bytes already cached; promote to MRU.
			s.lru.MoveToFront(el)
			return
		}
		// Hash collision (rare): evict the old entry, insert the new one.
		delete(s.items, h)
		s.lru.Remove(el)
	}

	if s.lru.Len() >= shardCacheSize {
		back := s.lru.Back()
		delete(s.items, back.Value.(*lruItem).h)
		s.lru.Remove(back)
	}
	s.items[h] = s.lru.PushFront(&lruItem{h: h, key: bytes.Clone(bz), tx: tx})
}

// decodeCache memoises decoded transactions. It is sharded for low lock
// contention under parallel runTx (block-stm) and uses LRU eviction so
// frequently-proposed txs stay warm across re-proposals.
//
// Returned sdk.Tx pointers are SHARED across consumers (PrepareProposal,
// ProcessProposal, BaseApp.runTx, CheckTx). Consumers MUST treat the
// returned tx as read-only; any mutation of the tx wrapper or its inner
// messages will leak across phases.
type decodeCache struct {
	shards [shardCount]cacheShard
}

func newDecodeCache() *decodeCache {
	c := &decodeCache{}
	for i := range c.shards {
		c.shards[i].items = make(map[uint64]*list.Element, shardCacheSize)
	}
	return c
}

func (c *decodeCache) get(bz []byte) (sdk.Tx, bool) {
	h := xxhash.Sum64(bz) ^ xxhashSeed
	return c.shards[h%shardCount].get(h, bz)
}

func (c *decodeCache) set(bz []byte, tx sdk.Tx) {
	if len(bz) > maxCachedTxBytes {
		return
	}
	h := xxhash.Sum64(bz) ^ xxhashSeed
	c.shards[h%shardCount].set(h, bz, tx)
}

// newCachingDecoder wraps base and returns a decoder that memoises results
// for the lifetime of the cache. Callers share the same cache instance so
// PrepareProposal and BaseApp.runTx benefit from each other's work.
func newCachingDecoder(base sdk.TxDecoder, cache *decodeCache) sdk.TxDecoder {
	return func(bz []byte) (sdk.Tx, error) {
		if tx, ok := cache.get(bz); ok {
			return tx, nil
		}
		tx, err := base(bz)
		if err != nil {
			return nil, err
		}
		cache.set(bz, tx)
		return tx, nil
	}
}

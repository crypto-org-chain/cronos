package app

import (
	"bytes"
	"container/list"
	"sync"

	"github.com/cespare/xxhash/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	decodeCacheSize = 10_000
	// maxCachedTxBytes caps per-entry payload size to bound worst-case
	// memory at decodeCacheSize * maxCachedTxBytes (~640MB). Txs above
	// this threshold are decoded normally but not cached. Covers normal
	// traffic (transfers, ERC20, most contract calls) and prevents an
	// adversary submitting MaxTxBytes-sized txs from exhausting RAM.
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
	h := xxhash.Sum64(bz)
	return c.shards[h%shardCount].get(h, bz)
}

func (c *decodeCache) set(bz []byte, tx sdk.Tx) {
	if len(bz) > maxCachedTxBytes {
		return
	}
	h := xxhash.Sum64(bz)
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

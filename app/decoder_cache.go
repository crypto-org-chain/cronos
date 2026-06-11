package app

import (
	"bytes"
	"container/list"
	cryptorand "crypto/rand"
	"encoding/binary"
	"sync"

	"github.com/cespare/xxhash/v2"
	cmdcfg "github.com/crypto-org-chain/cronos/cmd/cronosd/config"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// xxhashSeed randomizes shard assignment so an attacker can't target one shard.
var xxhashSeed = func() uint64 {
	var b [8]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		panic(err)
	}
	return binary.LittleEndian.Uint64(b[:])
}()

// shardCount is the number of independent LRU stripes in decodeCache. Must be a
// power of two so shardMask can replace modulo in shard selection.
const shardCount = 16

// shardMask maps a hash to a shard via h&shardMask (== h%shardCount for a
// power-of-two shardCount).
const shardMask = shardCount - 1

// Compile-time assert shardCount is a power of two.
var _ = [1]struct{}{}[shardCount&shardMask]

type lruItem struct {
	h   uint64
	key []byte // raw tx bytes; resolves xxhash collisions
	tx  sdk.Tx
}

// cacheShard is one LRU stripe of decodeCache with its own mutex, so access to
// different tx hashes is contention-free.
type cacheShard struct {
	mu    sync.Mutex
	cap   int
	items map[uint64]*list.Element
	lru   list.List // front = MRU, back = LRU; zero value is empty list
}

// get returns the cached tx for (h, bz). On hash collision returns (nil, false);
// the next set evicts the stale entry.
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

	if s.lru.Len() >= s.cap {
		back := s.lru.Back()
		delete(s.items, back.Value.(*lruItem).h)
		s.lru.Remove(back)
	}
	s.items[h] = s.lru.PushFront(&lruItem{h: h, key: bytes.Clone(bz), tx: tx})
}

// decodeCache is a sharded LRU cache of decoded txs. Returned sdk.Tx pointers
// are shared across consumers (PrepareProposal, runTx, CheckTx) — callers MUST
// NOT mutate them. Safe today because decode is the only writer: the EVM ante's
// VerifySender only compares against MsgEthereumTx.From (set at proto-decode),
// never writes it. A future ante that mutates a decoded msg would break sharing.
type decodeCache struct {
	shards     [shardCount]cacheShard
	maxTxBytes int
}

// newDecodeCache returns a cache with total capacity ~size and per-entry
// payload cap maxTxBytes. Pass <=0 for either to use defaults.
func newDecodeCache(size, maxTxBytes int) *decodeCache {
	if size <= 0 {
		size = cmdcfg.DefaultTxCacheSize
	}
	if maxTxBytes <= 0 {
		maxTxBytes = cmdcfg.DefaultTxCacheMaxTxBytes
	}
	shardCap := (size + shardCount - 1) / shardCount
	c := &decodeCache{maxTxBytes: maxTxBytes}
	for i := range c.shards {
		c.shards[i].cap = shardCap
		c.shards[i].items = make(map[uint64]*list.Element, shardCap)
	}
	return c
}

func (c *decodeCache) get(bz []byte) (sdk.Tx, bool) {
	h := xxhash.Sum64(bz) ^ xxhashSeed
	return c.shards[h&shardMask].get(h, bz)
}

func (c *decodeCache) set(bz []byte, tx sdk.Tx) {
	if len(bz) > c.maxTxBytes {
		return
	}
	h := xxhash.Sum64(bz) ^ xxhashSeed
	c.shards[h&shardMask].set(h, bz, tx)
}

// newCachingDecoder wraps base with the shared cache, so PrepareProposal and
// BaseApp.runTx reuse each other's decodes.
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

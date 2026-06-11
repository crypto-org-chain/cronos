package mempool

import (
	"container/list"
	"crypto/sha256"
	"sync"

	cmdcfg "github.com/crypto-org-chain/cronos/cmd/cronosd/config"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EncoderCache caches tx encoding
type EncoderCache struct {
	mu         sync.Mutex
	cap        int
	maxTxBytes int // per-entry payload ceiling; bytes above it aren't cached
	items      map[sdk.Tx]*list.Element
	lru        list.List // front = MRU, back = LRU; zero value is an empty list
}

type item struct {
	tx     sdk.Tx
	bz     []byte
	hash   [32]byte
	hashed bool // hash computed lazily on first HashTx call
}

// NewEncoderCache returns an LRU-bounded cache holding at most size entries,
// skipping txs whose canonical bytes exceed maxTxBytes (mirrors decodeCache).
// Pass <=0 for size/maxTxBytes to fall back to the cmdcfg defaults.
func NewEncoderCache(size, maxTxBytes int) *EncoderCache {
	if size <= 0 {
		size = cmdcfg.DefaultTxCacheSize
	}
	if maxTxBytes <= 0 {
		maxTxBytes = cmdcfg.DefaultTxCacheMaxTxBytes
	}
	return &EncoderCache{
		cap:        size,
		maxTxBytes: maxTxBytes,
		items:      make(map[sdk.Tx]*list.Element, size),
	}
}

// Set stores canonical proto bytes for a tx (raw req.Tx bytes on encode
// error). Concurrency-safe. Evicts the LRU entry when at capacity. Bytes above
// maxTxBytes aren't cached: EncodeTx re-encodes them on miss (rare; bounds heap).
func (e *EncoderCache) Set(tx sdk.Tx, bz []byte) {
	if tx == nil || len(bz) > e.maxTxBytes {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	if el, ok := e.items[tx]; ok {
		it := el.Value.(*item)
		it.bz = bz
		it.hashed = false // bytes changed; force re-hash
		e.lru.MoveToFront(el)
		return
	}
	if e.lru.Len() >= e.cap {
		if back := e.lru.Back(); back != nil {
			delete(e.items, back.Value.(*item).tx)
			e.lru.Remove(back)
		}
	}
	e.items[tx] = e.lru.PushFront(&item{tx: tx, bz: bz})
}

// Evict drops tx's entry so a stale/removed tx stops pinning the heap. No op if tx is absent.
func (e *EncoderCache) Evict(tx sdk.Tx) {
	if e == nil || tx == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if el, ok := e.items[tx]; ok {
		delete(e.items, tx)
		e.lru.Remove(el)
	}
}

// Get returns the registered bytes for tx, promoting it to MRU. Safe to call
// on a nil *EncoderCache (returns nil, false).
func (e *EncoderCache) Get(tx sdk.Tx) ([]byte, bool) {
	if e == nil || tx == nil {
		return nil, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	el, ok := e.items[tx]
	if !ok {
		return nil, false
	}
	e.lru.MoveToFront(el)
	return el.Value.(*item).bz, true
}

// HashTx returns sha256(bz), caching it on tx's entry so repeated reaps don't
// re-hash the same canonical bytes. bz must be the canonical bytes EncodeTx
// returned for tx. Safe on a nil receiver (hashes without caching). The hash is
// computed outside the lock so a slow hash never blocks admission's Set.
func (e *EncoderCache) HashTx(tx sdk.Tx, bz []byte) [32]byte {
	if e == nil || tx == nil {
		return sha256.Sum256(bz)
	}
	e.mu.Lock()
	if el, ok := e.items[tx]; ok {
		if it := el.Value.(*item); it.hashed {
			h := it.hash
			e.lru.MoveToFront(el) // accessed: keep hot, match Get/Set
			e.mu.Unlock()
			return h
		}
	}
	e.mu.Unlock()

	h := sha256.Sum256(bz)
	// Re-check: the entry may have been evicted while unlocked. The canonical
	// bytes are deterministic, so h stays valid for any re-added entry.
	e.mu.Lock()
	if el, ok := e.items[tx]; ok {
		it := el.Value.(*item)
		it.hash = h
		it.hashed = true
		e.lru.MoveToFront(el)
	}
	e.mu.Unlock()
	return h
}

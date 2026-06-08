package mempool

import (
	"container/list"
	"sync"

	cmdcfg "github.com/crypto-org-chain/cronos/cmd/cronosd/config"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TxGetter recovers the decoded tx for its raw proto bytes, so the EncoderCache
// can be keyed on the tx pointer. InsertTxHandler uses it after RunTx.
type TxGetter func(bz []byte) (sdk.Tx, bool)

// EncoderCache caches tx encoding
type EncoderCache struct {
	mu    sync.Mutex
	cap   int
	items map[sdk.Tx]*list.Element
	lru   list.List // front = MRU, back = LRU; zero value is an empty list
}

type item struct {
	tx sdk.Tx
	bz []byte
}

// NewEncoderCache returns an LRU-bounded cache holding at most size entries.
// Pass <=0 to fall back to cmdcfg.DefaultTxEncodeCacheSize.
func NewEncoderCache(size int) *EncoderCache {
	if size <= 0 {
		size = cmdcfg.DefaultTxEncodeCacheSize
	}
	return &EncoderCache{
		cap:   size,
		items: make(map[sdk.Tx]*list.Element, size),
	}
}

// Set stores canonical proto bytes for a tx (raw req.Tx bytes on encode
// error). Concurrency-safe. Evicts the LRU entry when at capacity.
func (e *EncoderCache) Set(tx sdk.Tx, bz []byte) {
	if tx == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	if el, ok := e.items[tx]; ok {
		el.Value.(*item).bz = bz
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

// Evict drops tx's entry so a stale/removed tx stops pinning the heap. No-op on
// a nil receiver or a tx that was never registered (e.g. already LRU-evicted).
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

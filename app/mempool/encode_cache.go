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

// EncoderCache maps decoded-tx pointers to canonical proto bytes, skipping
// proto.Marshal in the reap and PrepareProposal hot paths. LRU-bounded at cap:
// each entry pins the tx on the heap, so without eviction the map would grow
// unbounded. Register stores the canonical form so non-minimal peer bytes never
// reach a proposal.
type EncoderCache struct {
	mu    sync.Mutex
	cap   int
	items map[sdk.Tx]*list.Element
	lru   list.List // front = MRU, back = LRU; zero value is an empty list
}

// encoderItem is one LRU node: the tx pointer and its canonical bytes.
type encoderItem struct {
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

// Register stores canonical proto bytes for a tx (raw req.Tx bytes on encode
// error). Concurrency-safe. Evicts the LRU entry when at capacity.
func (e *EncoderCache) Register(tx sdk.Tx, bz []byte) {
	if tx == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	if el, ok := e.items[tx]; ok {
		el.Value.(*encoderItem).bz = bz
		e.lru.MoveToFront(el)
		return
	}
	if e.lru.Len() >= e.cap {
		if back := e.lru.Back(); back != nil {
			delete(e.items, back.Value.(*encoderItem).tx)
			e.lru.Remove(back)
		}
	}
	e.items[tx] = e.lru.PushFront(&encoderItem{tx: tx, bz: bz})
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

// Bytes returns the registered bytes for tx, promoting it to MRU. Safe to call
// on a nil *EncoderCache (returns nil, false).
func (e *EncoderCache) Bytes(tx sdk.Tx) ([]byte, bool) {
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
	return el.Value.(*encoderItem).bz, true
}

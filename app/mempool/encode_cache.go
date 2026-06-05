package mempool

import (
	"container/list"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"

	cmdcfg "github.com/crypto-org-chain/cronos/cmd/cronosd/config"
)

// TxGetter recovers the decoded tx for its raw proto bytes. InsertTxHandler
// uses it after RunTx so the EncoderCache can be keyed on the tx pointer.
type TxGetter func(bz []byte) (sdk.Tx, bool)

// EncoderCache maps decoded-tx pointers to canonical proto bytes for the
// reap and PrepareProposal hot paths, skipping proto.Marshal per reap cycle.
// Keys are sdk.Tx interface values (pointer equality for pointer-typed Txs).
// LRU eviction caps live entries at cap; each entry pins the tx on the heap
// so without eviction the map grows unboundedly over the node's lifetime.
// Register re-encodes to canonical form so non-minimal peer bytes are never
// stored verbatim in a proposal.
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

// Register stores the canonical proto bytes for a decoded tx (raw req.Tx
// bytes on the encoder-error fallback). Safe to call concurrently. Evicts the
// least-recently-used entry when the cache is at capacity.
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

// Bytes returns the registered bytes for tx if they were previously stored,
// promoting the entry to most-recently-used. Safe to call on a nil
// *EncoderCache — returns (nil, false).
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

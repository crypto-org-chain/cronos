package mempool

import (
	"container/list"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// defaultEncoderCacheSize is the NewEncoderCache fallback when a non-positive
// size is passed. Mirrors cmdcfg.DefaultTxDecodeCacheSize; kept as a local
// literal to avoid importing cmd/cronosd/config into the mempool package.
const defaultEncoderCacheSize = 10000

// TxGetter recovers the decoded tx for its raw proto bytes. InsertTxHandler
// uses it after RunTx so the EncoderCache can be keyed on the tx pointer.
type TxGetter func(bz []byte) (sdk.Tx, bool)

// EncoderCache maps decoded-tx pointers to their canonical proto bytes.
// InsertTxHandler registers entries; ReapTxsHandler and the fast
// PrepareProposal handler read them to skip proto.Marshal on the hot path.
//
// Keys are sdk.Tx interface values; for pointer-typed Txs (all cosmos-sdk Tx
// implementations) interface equality is pointer equality, so the same object
// held by the priority mempool produces a cache hit with zero encoding work.
//
// Bounded LRU eviction: the map key (an sdk.Tx interface holding a pointer)
// pins the tx on the heap, so an entry is never garbage-collected while it
// lives in the cache. Without eviction the map would grow with every tx ever
// admitted over the node's lifetime — not bounded by mempool.max-txs — and leak
// indefinitely. The LRU caps live entries at cap; the least-recently-used entry
// is dropped on overflow, releasing both the bytes and the last reference that
// pinned the tx, so the tx can then be collected once the mempool also drops it.
// A lookup that misses (because its entry was evicted) falls back to txEncoder
// on the reap / PrepareProposal path, so eviction only costs a re-encode, never
// correctness.
//
// Canonical bytes: Register stores the app-re-encoded tx, so a peer's
// non-minimal proto encoding cannot land verbatim in a proposal — every node
// decode-then-re-encodes to the same form, so replay paths
// (debug_traceTransaction, state-sync, migration) reproduce identical bytes.
// Exception: on a re-encode error the raw req.Tx bytes are registered instead;
// a tx that passed the AnteHandler is not expected to fail re-encoding, so this
// is effectively unreachable.
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
// Pass <=0 to fall back to defaultEncoderCacheSize.
func NewEncoderCache(size int) *EncoderCache {
	if size <= 0 {
		size = defaultEncoderCacheSize
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

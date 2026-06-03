package mempool

import (
	"reflect"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TxGetter retrieves a decoded tx by its raw proto bytes. Used by
// InsertTxHandler to look up the decoded tx after RunTx so raw bytes can be
// registered in the EncoderCache.
type TxGetter func(bz []byte) (sdk.Tx, bool)

// EncoderCache maps decoded-tx pointers to their original raw bytes.
// InsertTxHandler registers entries; ReapTxsHandler reads them to skip
// proto.Marshal on the hot reap path.
//
// Keys are the runtime pointer of the decoded sdk.Tx (always a pointer type
// in cosmos-sdk). The same pointer is stored in the priority mempool, so
// lookups during reap hit with zero encoding work.
//
// Entries are never explicitly deleted; stale entries (from txs evicted from
// the mempool) are harmless — their pointers are only reused after GC, at
// which point InsertTxHandler overwrites with fresh bytes. Memory is bounded
// by the number of live unique txs (≤ mempool.max-txs) plus a short-lived
// tail of recently-reaped entries awaiting GC.
type EncoderCache struct {
	m sync.Map // key: uintptr (tx pointer), value: []byte
}

// Register stores the raw bytes for a decoded tx. Safe to call concurrently.
func (e *EncoderCache) Register(tx sdk.Tx, bz []byte) {
	if ptr := txPointer(tx); ptr != 0 {
		e.m.Store(ptr, bz)
	}
}

// Bytes returns the raw bytes for tx if they were previously registered.
func (e *EncoderCache) Bytes(tx sdk.Tx) ([]byte, bool) {
	if ptr := txPointer(tx); ptr != 0 {
		if v, ok := e.m.Load(ptr); ok {
			return v.([]byte), true
		}
	}
	return nil, false
}

// txPointer returns the underlying pointer value of a sdk.Tx interface.
// All cosmos-sdk Tx implementations are pointer types; returns 0 for nil or
// value types.
func txPointer(tx sdk.Tx) uintptr {
	v := reflect.ValueOf(tx)
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		return v.Pointer()
	}
	return 0
}

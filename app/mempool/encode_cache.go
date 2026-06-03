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
// tail of recently-reaped entries awaiting GC. Note: because entry removal
// depends on Go GC address reuse (not explicit eviction), the map is
// append-only under load; operators should account for up to mempool.max-txs
// sync.Map entries when sizing node memory.
//
// GC pointer reuse edge case: if a tx is evicted, its object is collected by
// the GC, and a new tx is allocated at the same address before
// InsertTxHandler calls Register, Bytes() will return the evicted tx's bytes
// as a false hit. The window is the few lines between RunTx and Register in
// InsertTxHandler — very narrow and not practically exploitable. In the worst
// case (a blocklisted tx's bytes slip into a proposal), ProcessProposalHandler
// on every validating node decodes from the raw bytes and re-checks signers,
// causing the proposal to be rejected before commit. No safety violation
// occurs; only a liveness hiccup for that proposer round.
//
// Non-canonical bytes: the stored bytes are the raw gossip bytes as received
// by InsertTxHandler (req.Tx). A sender can submit a tx with non-minimal proto
// encoding (e.g. extra unknown fields, non-minimal varints). These bytes are
// used verbatim in proposals. All validating nodes receive and commit the same
// raw bytes, so there is no immediate block-level mismatch. However, any
// replay or re-execution path that decodes then re-encodes the tx (e.g.
// debug_traceTransaction, state-sync, upgrade migration) may produce different
// bytes, which would cause an AppHash divergence. Operators should ensure
// tx bytes are canonical at the RPC ingress layer if this is a concern.
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
// Safe to call on a nil *EncoderCache — returns (nil, false).
func (e *EncoderCache) Bytes(tx sdk.Tx) ([]byte, bool) {
	if e == nil {
		return nil, false
	}
	ptr := txPointer(tx)
	if ptr == 0 {
		return nil, false
	}
	v, ok := e.m.Load(ptr)
	if !ok {
		return nil, false
	}
	return v.([]byte), true
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

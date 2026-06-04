package mempool

import (
	"reflect"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TxGetter recovers the decoded tx for its raw proto bytes. InsertTxHandler
// uses it after RunTx so the EncoderCache can be keyed on the tx pointer.
type TxGetter func(bz []byte) (sdk.Tx, bool)

// EncoderCache maps decoded-tx pointers to their canonical proto bytes.
// InsertTxHandler registers entries; ReapTxsHandler reads them to skip
// proto.Marshal on the hot reap path.
//
// Keys are the runtime pointer of the decoded sdk.Tx (always a pointer type in
// cosmos-sdk); the same pointer is held by the priority mempool, so reap
// lookups hit with zero encoding work.
//
// Entries are never explicitly deleted — a stale pointer is only reused after
// GC, at which point Register overwrites it with fresh bytes. The map is thus
// append-only under load; size node memory for up to mempool.max-txs entries.
//
// GC pointer-reuse race: if an evicted tx is GC'd and a new tx lands at the same
// address before Register runs, Bytes() returns the old tx's bytes (a false hit,
// in the few lines between RunTx and Register). Those bytes are always from a
// previously-admitted, ante-valid tx, so they are well-formed and decodable — a
// stale hit cannot inject malformed or unauthorized bytes. Safety rests on
// deterministic execution of the block bytes all nodes agree on (BFT consensus
// is over the byte sequence), NOT on ProcessProposal re-validation (which
// ACCEPTs all when no blocklist is set — the default). A stale tx (e.g. a
// consumed nonce) just fails at execution and wastes its slot: a liveness
// hiccup, not a safety violation.
//
// Canonical bytes: Register stores the app-re-encoded tx, so a peer's
// non-minimal proto encoding cannot land verbatim in a proposal — every node
// decode-then-re-encodes to the same form, so replay paths
// (debug_traceTransaction, state-sync, migration) reproduce identical bytes.
// Exception: on a re-encode error the raw req.Tx bytes are registered instead;
// a tx that passed the AnteHandler is not expected to fail re-encoding, so this
// is effectively unreachable.
type EncoderCache struct {
	m sync.Map // key: uintptr (tx pointer), value: []byte
}

// Register stores the canonical proto bytes for a decoded tx (raw req.Tx
// bytes on the encoder-error fallback). Safe to call concurrently.
func (e *EncoderCache) Register(tx sdk.Tx, bz []byte) {
	if ptr := txPointer(tx); ptr != 0 {
		e.m.Store(ptr, bz)
	}
}

// Bytes returns the registered bytes for tx if they were previously stored.
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
	if v.Kind() == reflect.Pointer && !v.IsNil() {
		return v.Pointer()
	}
	return 0
}

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

// EncoderCache maps decoded-tx pointers to their canonical proto bytes.
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
// the GC, and a new tx is allocated at the same address before InsertTxHandler
// calls Register, Bytes() returns the evicted tx's bytes as a false hit. The
// window is the few lines between RunTx and Register in InsertTxHandler — very
// narrow. The false-hit bytes are always those of a previously-admitted,
// ante-valid tx (only successfully-admitted txs are Registered), so they are
// well-formed and decodable — a stale hit cannot inject malformed or
// unauthorized bytes. If such stale bytes land in a proposal, every validating
// node still agrees on the identical block bytes (BFT consensus is over the
// byte sequence) and runs FinalizeBlock on them deterministically, so no
// AppHash divergence occurs. The stale tx (e.g. a consumed nonce) merely fails
// at execution and wastes its slot — a liveness/efficiency hiccup, not a safety
// violation. This guarantee does NOT depend on ProcessProposal re-validating
// the bytes: ProcessProposalHandler short-circuits to ACCEPT when no blocklist
// is configured (the default), so safety rests on deterministic execution of
// the agreed-upon bytes, not on a proposal-time recheck.
//
// Canonical bytes: InsertTxHandler re-encodes the decoded tx with the app's
// TxEncoder and registers those canonical proto bytes here. A peer that
// gossips a tx with non-minimal proto encoding (extra unknown fields,
// non-minimal varints) therefore cannot have its raw bytes land verbatim in a
// proposal — every node decodes-then-re-encodes to the same canonical form, so
// replay/re-execution paths (debug_traceTransaction, state-sync, upgrade
// migration) reproduce identical bytes and no AppHash divergence arises from
// the encoding. The sole exception is the fallback in InsertTxHandler: if
// re-encoding errors, the raw req.Tx bytes are registered so reap can still
// ship the tx. Those bytes are non-canonical and, if non-minimal, carry the
// divergence risk on decode-re-encode paths — but a tx that already passed the
// AnteHandler is not expected to fail re-encoding, so the fallback is
// effectively unreachable in practice.
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

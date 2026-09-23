# App-mempool recheck: per-signer groups, chunked lock holds, gap-proof cascade

Tracks the remaining items of [#2109](https://github.com/crypto-org-chain/cronos/issues/2109)
for `mempool.type=app`: timing-dependent recheck evictions (item 3), nonce-gap orphans
(item 4), and the admission/recheck split (item 5).

## Problem

Post-Commit recheck re-validates pending txs with `RunTx(ExecModeReCheck)`. Before this
change it released the admission mutex between candidates, so peer `InsertTx` and RPC
`CheckTx` for the same sender could land in the middle of that sender's nonce chain. A
candidate then failed wrong-sequence or passed depending on scheduling, and was evicted or
kept accordingly. Not a data race (every `RunTx` is serialized), but node-local and
non-deterministic.

Separately, an eviction in the middle of a sender's chain (timeout, TTL, ante failure)
left the higher-nonce siblings pooled with no way to become valid. Recheck only picks
senders touched by a block, and the fast `PrepareProposal` path skips the ante, so those
orphans could sit in the pool until TTL, or be proposed and fail at `FinalizeBlock`.

## Why not a branched store

An earlier draft gave the mempool its own `CacheMultiStore` branched off the committed
store and passed it as `RunTx`'s `txMultiStore` for admission and recheck. It was dropped:

- It does not change what recheck interleaves with. Admission and recheck still share one
  pending-nonce view under one mutex; only the object holding it moved.
- `BaseApp.Simulate` runs on `checkState`, and the cosmos sequence check is not gated on
  simulate. With nonce bumps redirected into a private branch, `checkState` stays at the
  committed nonce and a `Simulate` of a pipelined cosmos tx fails wrong-sequence. Default
  SDK behavior (and the previous app-mempool behavior) is for `Simulate` to see pending
  nonces.
- It forced `CheckTxHandler` to bypass the `runTx` closure baseapp passes in and
  re-derive the exec mode from `req.Type`.

`checkState` therefore remains the single pending-nonce view. `App.Commit` holds the
admission mutex across `BaseApp.Commit()` so the `checkState` reset never races a
`RunTx`.

## Design

### Per-signer groups

`groupCandidates` buckets candidates by first signer — the one `PriorityNonceMempool`
orders by — in first-appearance order, and sorts each group ascending by sequence (the
deferred carry can hand candidates out of nonce order). Encoding happens here, outside
the mutex.

`maxRecheckBatch` (`cronos.mempool-txs-per-block`) is a soft cap applied at group
boundaries only: once the running total would exceed it, the remaining whole groups carry
into `deferred` for the next cycle and their senders are merged back into staging. A
sender's chain is never split across cycles. The sender re-staging matters because
`deferred` is keyed on tx identity: a fee bump replacing a carried tx at the same nonce
would otherwise vanish from the carry and its live tail would fail as a false gap.

### Chunked lock holds

A group runs in chunks of `recheckChunkSize` (256) candidates, each under one hold of the
admission mutex. Without this, one sender's queue depth bounds how long `App.Commit`
waits on the mutex. Between chunks, a same-sender admission or a Commit can land; that is
the same residual interleaving accepted between groups.

### Cascade eviction

Within a chunk, a candidate that fails is evicted. If the failure is a nonce error
(`ErrWrongSequence` from cosmos sig verification, `ErrInvalidSequence` from the EVM ante)
*and* an earlier candidate in the **same lock hold** was accepted *and* the failing nonce
is strictly above `lastOK+1`, the account's next expected nonce is provably `lastOK+1`
and every remaining candidate in the chunk is above a hole nothing can fill while the
mutex is held. They are evicted without a `RunTx` each.

Nothing carries across chunks. Once the mutex is released, admissions can move the
account's nonce past the next chunk's head, so a nonce error there means *stale*, not
*gap* — the two are indistinguishable from the error alone. An earlier draft carried the
cursor and blind-evicted on a head nonce error; with two or more same-sender admissions
between chunks that evicted valid replacements. The next chunk therefore starts with no
accepted nonce and must re-prove any gap. Cost: after a gap, each later chunk of that
sender spends one `RunTx` per candidate instead of none.

Cascade is disabled for a group that is not the signer's clean ascending view: unknown
signer, any signer named by a multi-signer tx (that tx is grouped under its first signer
and can fill a nonce this group cannot see), an unordered tx (keyed by timeout, not
sequence), a duplicate sequence, or a tx dropped on encode error. A non-nonce failure
(e.g. insufficient funds) evicts only that tx; the next sibling then fails as a proven gap
if it really is one.

### Ante cache

ethermint's EVM ante caches one `(from, nonce)` per `MsgEthereumTx` at CheckTx so a
replacement can skip nonce verification. A cascade or TTL eviction never runs the ante on
the evicted tx, so `evict` also drops those entries via `SetAnteCache`; otherwise a stale
entry lets a resubmit of the evicted nonce bypass the check.

### Split

- `exec.go` — `txExec`: the mutex, `RunTx` entry point, codecs, and the `PendingTxs`
  snapshot cache both halves invalidate.
- `admitter.go` — peer `InsertTx` and RPC `CheckTx`.
- `scheduler.go` — staging, selection/TTL, grouping, chunked recheck, eviction.

`Manager` is a facade so `app.go` and the proposal handler call sites do not churn. Lock
order: `recheckMu > exec.mu > stagingMu`.

## Residuals

- `PriorityNonceMempool.Remove` resolves by `(sender, nonce)`, not tx identity. Evicting a
  stale candidate whose slot was replaced by a fee bump between snapshot and eviction
  drops the replacement. `BaseApp.RunTx` does the same on a ReCheck ante failure; the
  client's resubmit resolves it.
- A Commit landing mid-pass is harmless: the remaining chunks run against the newer
  `checkState`, and each chunk re-proves any gap under its own lock hold.
- The admission mutex is still held across the whole `BaseApp.Commit()` (item 1 of
  #2109) until memiavl reads are safe during commit.

## Differential tests

`app/proposal_diff_test.go` seeds two identical pools and runs the fast `PrepareProposal`
path against the default full-ante handler. Same-sender gaps produce identical
selections (the per-signer sequence guard is shared). Stale nonce, recheck backlog, and
timeout height diverge as expected: the fast path proposes what the ante would reject and
leaves it pooled for recheck; the default path drops and evicts during the proposal. In
every divergent case the real `ProcessProposalHandler` still accepts the block, so an
ante-invalid tx only becomes a failed tx result at `FinalizeBlock`.

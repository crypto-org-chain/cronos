# App-mempool recheck: per-signer groups, chunked lock holds, gap-proof cascade

Covers items 3–5 of [#2109](https://github.com/crypto-org-chain/cronos/issues/2109) for
`mempool.type=app`: timing-dependent recheck evictions and nonce-gap orphans.

## Why not a branched store

An earlier draft ran admission and recheck on a mempool-owned `CacheMultiStore`. It was
dropped: both still shared one nonce view under one lock, and `BaseApp.Simulate` runs on
`checkState`, so simulating a pipelined cosmos tx failed wrong-sequence. `checkState`
stays the single pending-nonce view.

## Design

Lock order: `recheckMu > exec.gate > stagingMu`.

### Groups

`groupCandidates` buckets candidates by first signer and sorts each group by nonce.
`maxRecheckBatch` caps work per cycle, but only between groups. Overflow groups carry
whole into `deferred`, and their senders are re-staged, because a fee bump changes tx
identity and would drop out of the carry.

### Chunks

A group runs in chunks of up to 64 under one gate hold each (~1.4 ms at ~22 µs per
`RunTx`). Before each chunk, recheck lets admissions already queued go first, for up to
1 ms: `sync.Mutex` otherwise lets the recheck goroutine re-take the gate ahead of them
until they have waited 1 ms. A pending Commit goes ahead of queued holders, and a chunk
ends early once a Commit is waiting, so Commit waits out at most one `RunTx`.

### Commit during a cycle

The gate counts Commits. The cycle reads that count when it drains staging, under the
same gate hold. Once a Commit runs after it, the cycle stops and re-stages every sender it
has not reached: run on the reset `checkState`, they would re-apply nonces that the
recheck Commit triggers then rejects as stale. Counter: `cronos.mempool.recheck.restaged`.

### Cascade eviction

A failing candidate is evicted. If it failed with a nonce error above `lastOK+1`, where
`lastOK` passed earlier in the **same chunk**, the gap is proven: nothing can fill it
while the gate is held. The rest of the chunk is evicted without a `RunTx` each. Nothing
carries across chunks, since an admission between them can move the nonce.

Cascade is off for a group that isn't the signer's whole ascending-nonce view:

- unknown signer;
- a co-signer of a multi-signer tx (that tx can fill a nonce from another group);
- a sender of a batch eth tx (the EVM signer adapter reports only the first nonce);
- an unordered tx, or a duplicate nonce;
- a tx dropped while grouping (encode error or panic).

A non-nonce failure, or a panic in `RunTx` or eviction, never starts a cascade.

## Residuals

- A same-sender admission can still land between chunks or groups, so recheck stays
  timing-dependent there. After a gap, each later chunk spends one `RunTx` per candidate.
- `PriorityNonceMempool.Remove` matches by `(sender, nonce)`, so evicting a stale
  candidate whose slot a fee bump just replaced drops the replacement, as `BaseApp` does.
- The gate is held across all of `BaseApp.Commit()` (item 1 of #2109).

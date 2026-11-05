# App-mempool recheck: per-signer groups, chunked lock holds, gap-proof cascade

Covers items 3–5 of [#2109](https://github.com/crypto-org-chain/cronos/issues/2109) for
`mempool.type=app`.

## Problem

- Recheck released the admission lock between candidates. A same-sender `InsertTx` or
  `CheckTx` could land mid-chain, so a candidate passed or failed depending on timing.
- Evicting a tx mid-chain left its higher-nonce siblings pooled with no way to become
  valid. They sat there until TTL, or got proposed and failed at `FinalizeBlock`.

## Why not a branched store

An earlier draft ran admission and recheck on a mempool-owned `CacheMultiStore`. It was
dropped:

- Admission and recheck still shared one nonce view under one lock, so nothing changed.
- `BaseApp.Simulate` runs on `checkState`, and its sequence check isn't skipped for
  simulate. With nonces in a private branch, simulating a pipelined cosmos tx failed.

`checkState` stays the single pending-nonce view. `App.Commit` holds the gate across
`BaseApp.Commit()`, so the reset never races a `RunTx`.

## Design

### Groups

`groupCandidates` buckets candidates by first signer and sorts each group by nonce.
`maxRecheckBatch` caps work per cycle, but only between groups. Overflow groups carry
whole into `deferred`, and their senders are re-staged, because a fee bump changes tx
identity and would drop out of the carry.

### Chunks

A group runs in chunks of up to 256 under one gate hold each. A pending Commit goes
ahead of queued holders, and a chunk ends early once a Commit is waiting, so Commit
waits out at most one `RunTx`.

### Commit during a cycle

The gate counts Commits. The cycle reads that count when it drains staging, under the
same gate hold, so the pin is never newer than the drained senders. Once a Commit runs
after it, the cycle stops and re-stages every sender it has not reached. Running them on the
reset `checkState` would re-apply their nonces, and the recheck that Commit triggers
would then reject the same valid txs as stale. Counter: `cronos.mempool.recheck.restaged`.

### Cascade eviction

A failing candidate is evicted. If it failed with a nonce error, an earlier candidate in
the **same chunk** passed, and its nonce is above `lastOK+1`, the gap is proven: nothing
can fill it while the gate is held. The rest of the chunk is evicted without a `RunTx`
each.

Nothing carries across chunks. Once the gate is released, an admission can move the
nonce, so a nonce error at the next chunk's head may mean stale, not gap.

Cascade is off for a group that isn't the signer's whole ascending-nonce view:

- unknown signer;
- a co-signer of a multi-signer tx (that tx can fill a nonce from another group);
- a sender of a batch eth tx (the EVM signer adapter reports only the first nonce);
- an unordered tx, or a duplicate nonce;
- a tx dropped while grouping (encode error or panic).

A non-nonce failure, or a panic in `RunTx` or eviction, never starts a cascade.

### Split

- `exec.go`: the gate, `RunTx`, codecs, the pending cache, and the logger.
- `admitter.go`: `InsertTx` and `CheckTx`, with the RPC in-flight cap.
- `scheduler.go`: staging, selection, grouping, chunked recheck, eviction.

Lock order: `recheckMu > exec.gate > stagingMu`.

## Residuals

- A same-sender admission can still land between chunks or groups, so recheck stays
  node-local and timing-dependent there. After a gap, each later chunk spends one `RunTx`
  per candidate instead of none.
- `PriorityNonceMempool.Remove` matches by `(sender, nonce)`. Evicting a stale candidate
  whose slot a fee bump just replaced drops the replacement, as `BaseApp` does on a
  recheck failure. The client resubmits.
- The gate is held across all of `BaseApp.Commit()` (item 1 of #2109).

## Differential tests

`app/proposal_diff_test.go` runs the fast `PrepareProposal` path against the default
full-ante handler on identical pools. They agree on same-sender gaps. They diverge on
stale nonces, recheck backlog, and timeout height, and `ProcessProposal` still accepts
every divergent block.

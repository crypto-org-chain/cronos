# Plan — C3: lock-free signature verification on the app-mempool admission path

- **Finding:** `docs/audit/code-review-pr-2091.md` → C3 (Required, "document"); this plan covers the *real* fix.
- **Status:** Design / not started.
- **Scope:** `mempool.type=app` admission path only. The default `flood` path is unaffected.

## Problem

`Admitter.InsertTxHandler` (`app/mempool/insert.go`) serializes **all** mempool
admission behind a single mutex:

```go
a.mu.Lock()
defer a.mu.Unlock()
_, _, _, err := a.runner.RunTx(sdk.ExecModeCheck, req.Tx, nil, -1, nil, nil)
```

The lock exists because `RunTx(ExecModeCheck, …, txMultiStore=nil, …)` runs the
**entire** ante chain against the shared `checkState` multistore — a Go map that
is unsafe for concurrent writes (`baseapp.go:778`, `getContextForTx` →
`cacheTxContext` → `msCache.Write`). Without the lock, concurrent ingestion
panics with `concurrent map writes`.

Each `RunTx(ExecModeCheck)` performs one secp256k1 `ecrecover`, which dominates
admission cost. Serializing every admission caps ingest at roughly **1.5k–5.4k
tx/s** regardless of how many block-stm workers execute the resulting block.
The PR's +40% TPS win is execution-side; admission is the next wall.

## Why this is lower-risk than it looks

**Admission is node-local, not consensus.** A tx that wrongly slips into the
local mempool is re-validated in `FinalizeBlock` by every node's deterministic
ante run; a bad tx simply fails there and wastes block space — it cannot fork
the chain. So admission-side concurrency only risks *local* correctness
(mempool quality, nonce dedup), never consensus safety. This bounds the blast
radius of the change.

## Current call path (traced)

| Step | Location | State touched |
|------|----------|---------------|
| Admission entrypoint | `app/mempool/insert.go:83` `InsertTxHandler` | takes `a.mu` |
| RunTx | `vendor/.../baseapp/baseapp.go:778` | `checkState` ms (shared map) |
| `txMultiStore` override hook | `baseapp.go:791-793` | caller may supply isolated store |
| Ante dispatch | `vendor/.../ethermint/evmd/ante/ante.go:45` `NewAnteHandler` | — |
| **EVM stateless sig-verify** | `vendor/.../ethermint/evmd/ante/handler_options.go:123` → `evmante.VerifyEthSig(tx, signer)` (`ante/sigverify.go:31`) | **none** — pure fn of `tx` + `ethSigner` |
| EVM stateful ante (fee deduct, sequence) | `handler_options.go` (post-123) | reads/writes account state |
| Cosmos sig-verify | `evmd/ante/evm_handler.go:50-58` (`SetPubKey`/`SigGasConsume`/sig decorators) | reads account pubkey |
| Nonce-existence dedup cache | `vendor/.../ethermint/ante/cache/antecache.go` | own `sync.RWMutex`, independent of `checkState` |

Key enablers discovered:

1. **`evmante.VerifyEthSig(tx, signer)` is standalone and stateless.** The
   `ethSigner` is built from block config (chain ID, block number, block time) —
   all read-only. cronos can call this **directly, lock-free, with no fork
   change**.
2. **`RunTx` already accepts a caller-supplied `txMultiStore`** (`baseapp.go:791`)
   — the block-stm fork's isolation hook. A non-nil store makes RunTx run
   against a branch instead of shared `checkState`.
3. **The nonce dedup (`AnteCache`) already has its own lock** — it does not need
   the admission mutex, so it can stay correct while the heavy crypto goes
   lock-free.

## The core obstacle: double-verify

Pre-verifying the signature outside the lock saves nothing if the in-lock
`RunTx` re-runs `VerifyEthSig` — we'd pay `ecrecover` **twice** (net loss). To
win, the in-lock ante must **skip** sig-verify when the tx was already verified.
That skip cannot be injected from cronos (the ante chain is vendored), so it
requires a **narrow fork change** — not a RunTx rewrite.

## Design options

### Option A — cronos-only, isolated `txMultiStore` (no fork bump)
Give each concurrent admission its own `CacheMultiStore` branched off
`checkState` and pass it as `txMultiStore`. RunTx (incl. `ecrecover`) runs
lock-free against the branch.

- **Pro:** no fork change; real ecrecover parallelism.
- **Con:** loses cross-tx nonce dedup (each branch reads the same committed
  nonce, none sees the others' increments). Must add a short critical section
  using `AnteCache` (or equivalent) for nonce reservation + a final merge.
  Requires verifying `checkState`'s underlying CacheKV is safe for concurrent
  reads. Net: partial, semantically delicate, still not a clean split.

### Option B — pre-verify lock-free + fork skips in-lock verify (RECOMMENDED)
1. **cronos (no fork):** in `InsertTxHandler`, before the lock, decode the tx,
   build the `ethSigner` from a read-only block-config snapshot, and call
   `evmante.VerifyEthSig` (EVM) / the stateless cosmos sig path. Reject bad
   signatures immediately — the expensive crypto runs fully concurrent.
2. **fork (`crypto-org-chain/ethermint`):** make the EVM sig-verify decorator
   skip `VerifyEthSig` when a "already verified" signal is present (e.g. a value
   on `sdk.Context`, or a per-tx entry in the incarnation cache that `RunTx`
   already threads through). The stateful ante (fee deduct, sequence increment)
   still runs under the existing admission lock — but it no longer carries the
   `ecrecover` cost.
3. **cronos:** thread the signal through `RunTx`'s `incarnationCache` param
   (already in the signature) so no new SDK API is needed — only ethermint ante
   reads it.

- **Pro:** clean separation (stateless concurrent, stateful serialized);
  preserves nonce dedup; consensus path (`FinalizeBlock`) unchanged because the
  skip only applies to `ExecModeCheck` admission.
- **Con:** cross-repo PR + `go.mod` bump; consensus-adjacent review of the
  ethermint fork.

## Recommendation

**Option B**, scoped to **EVM txs first** (cronos load is EVM-dominant; cosmos
txs keep the current locked path). The fork change is narrow: one decorator
consulting a skip-signal, gated to `ExecModeCheck`.

## Phased tasks

1. **Verify concurrent-read safety** of `checkState` CacheKV (read `store/cachekv`).
   → verify: race test with N goroutines reading while one writes.
2. **Bench baseline:** admission tps at the single-mutex ceiling (need a
   load harness driving `InsertTx` concurrently). → verify: numbers recorded.
3. **cronos pre-verify (Option B.1):** call `VerifyEthSig` before the lock for
   EVM txs; reject early. No skip yet (double-verify accepted temporarily).
   → verify: existing `app/mempool` tests + race target pass; correctness
   unchanged.
4. **ethermint fork (Option B.2):** add `ExecModeCheck`-gated skip-signal to the
   EVM sig decorator; unit-test the decorator both ways.
   → verify: ethermint test suite; skip only fires in check mode.
5. **Wire signal + bump go.mod (Option B.3):** thread skip via `incarnationCache`;
   move `VerifyEthSig` outside the lock for real.
   → verify: admission bench shows scaling past the single-mutex ceiling;
   nonce-dedup integration test (`integration_tests/test_app_mempool.py`) green.
6. **Consensus-safety check:** confirm `FinalizeBlock` ante is untouched (skip is
   check-mode only) → verify: a node syncs a block built under the new path.

## Success criteria

- Admission tps scales with cores past the documented 1.5k–5.4k ceiling.
- `ecrecover` runs outside the admission mutex (confirmed by profile).
- No `concurrent map writes`; `test-race-mempool` clean.
- Cross-tx nonce dedup preserved (same-account same-nonce flood still deduped).
- `FinalizeBlock` path byte-for-byte unchanged → consensus-safe.

## Open questions

- Is a `sdk.Context` value or the `incarnationCache` the cleaner skip-signal
  carrier? (incarnationCache avoids a new ctx key but is block-stm-flavored.)
- Cosmos-tx path: leave locked, or apply the same split later?
- Do we need the `txMultiStore` isolation (Option A) *in addition*, to also
  parallelize the stateful reads, or is serializing only the (now cheap)
  stateful ante enough? Bench after step 5 decides.

## Measured results & gate decision (2026-06-05)

Phases 1–3 (cronos-only) landed; the ethermint fork (4–5) was gated on a bench
proving admission CPU is the wall. The bench (`app.BenchmarkAdmission`,
16-way concurrent `InsertTx`, plain EVM transfers, Apple M1 Max):

| Build | admit-tx/s |
|-------|-----------|
| CGO=1 (C libsecp256k1, ~634µs/ecrecover) | 10,862 |
| CGO=0 (pure-Go, ~184µs/ecrecover)        | 12,751 |

A 3.5× ecrecover speedup moves admission throughput only ~17%, so **in-lock
`ecrecover` is a minor fraction of the serialized admission cost** on this
machine — the lock is held mostly by the rest of the ante (account/fee/nonce
reads) + mempool insert, which Option B's skip does *not* parallelize. Both
numbers already clear the doc's feared 1.5k–5.4k ceiling.

**Gate decision: hold the fork (phases 4–5).** On arm64 the win is bounded at
~17% and the serialized cost is elsewhere. Caveats that could reopen it: on
x86_64 with CGO `libsecp256k1` (634µs) ecrecover may dominate enough to approach
the 1.5k ceiling — but the cheaper lever there is **pure-Go ecrecover (CGO=0)**,
which is already 3.5× faster with no fork/consensus change. Re-measure on a
production-representative x86 host before committing to the cross-repo fork.

Phase 1 also surfaced a **pre-existing data race** unrelated to Option B — see
`docs/audit/app-mempool-beginblock-chainid-race.md`. The lock-free pre-verify
itself is race-clean (pure signer via `LatestSignerForChainID`, mutex-guarded
decode cache); it deliberately avoids `EVMBlockConfig`, which reads
BeginBlock-mutated keeper state and writes a per-block cache.


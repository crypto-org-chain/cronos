# Plan: mempool-owned branched context for recheck + admission

Tracks the remaining open items of [#2109](https://github.com/crypto-org-chain/cronos/issues/2109).

## Context

PR #2118 moved the post-Commit mempool recheck off the consensus path onto an async
worker. In review, songgaoye flagged a remaining follow-up:

> recheck uses the shared `checkState` and releases `a.mu` between candidates, so
> concurrent `InsertTx/CheckTx` can interleave and make evictions timing-dependent.
> Move recheck onto a dedicated branched context. cosmos/evm's `mempool/rechecker.go`
> runs the ante on a branched `CacheMultiStore` instead of the shared `checkState`.

Today all three mempool `RunTx` call sites — `admit` (`app/mempool/manager.go:215`), the
RPC `CheckTxHandler` (`:257`), and `runRecheck` (`:498`) — pass a `nil` `txMultiStore` and
so run against baseapp's **shared** `checkState`. `RunTx(ExecModeReCheck)` writes nonce
bumps straight back into it (baseapp `mode != execModeCheck` → `msCache.Write()`), which is
*load-bearing*: `Commit` resets `checkState` to committed state, and recheck's ante writes
are what rebuild the pending-nonce view admission relies on. Because `a.mu` is released
between recheck candidates, admission interleaves and perturbs those reads/writes →
node-local, non-deterministic evictions (not a data race — every `RunTx` is individually
serialized by `a.mu`).

Goal: give the app-mempool its **own** working state — a `CacheMultiStore` branched off the
committed store — that admission and recheck share, mirroring cosmos/evm's model where the
mempool owns its context rather than sharing baseapp `checkState`.

## Key enabler

The crypto-org-chain SDK fork's `RunTx` already takes a `txMultiStore` 5th arg
(`baseapp/baseapp.go:778`): when non-nil it overrides the ctx multistore
(`ctx.WithMultiStore(txMultiStore)`, `:791`) and its internal ante branch writes back into
that store on success (ReCheck `:918`, Check-after-Insert `:932`). So we do **not**
reimplement cosmos/evm's `GetContext()/write()` closure — we pass a mempool-owned
`CacheMultiStore` as the 5th arg. `getContextForTx` (`:610`) still sources
header/consensus/gas/`IsReCheckTx` from `checkState`, so **`checkState` must stay** as the
context source; we only displace its multistore's nonce-tracking role.

## Two decisions that shape the design

### 1. A shared `base` alone does not fix the determinism complaint

Moving both paths from `checkState` to one shared `base` keeps exactly the interleaving
review objected to: `runRecheck` still drops the lock between candidates and admission
still lands in between. What the shared `base` *does* buy is that admission and recheck no
longer write into `checkState`, so queries and `Simulate` see committed state instead of a
pending-nonce view.

Determinism is fixed separately, by **grouping recheck candidates per signer** and holding
the store lock per bounded chunk within each group (Phase B) — see below.

### 2. A recheck-private branch is rejected

Forking a private `CacheMultiStore` at batch start and `Write()`-ing it back at batch end
looks like stronger isolation but is worse:

- `Write()` applies the whole write set, last-writer-wins per key. An admission that lands
  mid-batch (alice nonce 7 → writes nonce 8) is silently rolled back by the batch's older
  view (alice 5→6), so alice's next tx fails wrong-sequence. That trades a benign
  timing-dependence for state loss.
- Discarding the batch instead of writing it back is not an option either: recheck's writes
  are load-bearing for the pending-nonce view (see Context).

So: one `base`, shared, plus per-sender lock grouping.

### 3. All three `RunTx` call sites must move in the same change

`base` has to be the *sole* nonce authority. If recheck writes to `base` while admission
still reads `checkState` (reset to committed state at every `Commit`), admission stops
seeing the pending-nonce view and rejects legitimate higher-nonce siblings. The same
divergence appears between peer `InsertTx` and RPC `CheckTx` if only one of them moves.
This corrects an earlier draft of this plan, which staged recheck ahead of admission as
"lowest risk" — that intermediate state is a functional regression, not a safe step.

## Design

### `mempoolState`

`app/mempool/state.go` (new):

```go
type mempoolState struct {
    mu       sync.RWMutex
    base     storetypes.CacheMultiStore
    provider func() storetypes.CommitMultiStore
}
```

- `refreshLocked()` — `base = provider().CacheMultiStore()`; caller holds the store guard.
- `store() storetypes.MultiStore` — RLock, return `base`. Nil-safe: a nil `mempoolState`
  (or nil `base`) returns nil, so `RunTx` falls back to `checkState` and the existing
  `newManager()` unit tests stay green without a store.

`Manager` gains `state *mempoolState` and `gen atomic.Uint64`. `NewManager` wires
`provider: app.CommitMultiStore` but does *not* refresh: it runs inside `baseAppOptions`,
before `LoadLatestVersion`, so a refresh there would branch off an unloaded store. `base`
stays nil (and `store()` falls back to `checkState`) until `App` calls
`RefreshMempoolStateLocked` right after `LoadLatestVersion` succeeds — the earliest point
the store is actually loaded. The `newManager()` test constructor also leaves `state` nil.


### Call sites

Change the 5th `RunTx` arg `nil → a.state.store()` in `admit`, `runRecheck`, and
`CheckTxHandler`.

`CheckTxHandler` receives a `runTx sdk.RunTx` closure from baseapp that hardcodes
`txMultiStore = nil` (`baseapp/abci.go:408`). The handler therefore calls
`a.runner.RunTx` directly and derives the exec mode from `req.Type` the same way baseapp
does (`New → ExecModeCheck`, `Recheck → ExecModeReCheck`, anything else is an error). That
bypasses baseapp's "avoid users overriding the execution mode" wrapper, so the mapping
must stay in sync with `BaseApp.CheckTx`.

### Refresh at Commit

`App.Commit` already holds `AdmissionMutex()` across `BaseApp.Commit()`. Refresh `base` and
bump `gen` inside that same critical section, right after `BaseApp.Commit()` returns and
before `TriggerRecheck()`. Every `base`-writing `RunTx` is serialized by the same mutex, so
the swap can never race a reader and the `CacheMultiStore`'s maps are never concurrently
mutated.

### Lock model (no new mutex)

`Manager.mu` — renamed `txExec.mu`, since it now guards `base` rather than `checkState` — is
held briefly around each `RunTx` (unchanged granularity) and around
`BaseApp.Commit()` + `refreshLocked` in `App.Commit`. Order `recheckMu > txExec.mu >
stagingMu` is preserved; `refreshLocked` is a leaf that assumes the caller holds `txExec.mu`,
so there is no re-entrant acquisition.

### Cancellation

`RecheckTxs` captures `gen` right before `runRecheck`, after `selectTxs`/grouping/capping —
not right after `drainStaging` — so a Commit landing during the O(pool) scan doesn't abort
the whole pass before a single group has run. `runRecheck` returns early once `gen` differs,
abandoning candidates validated against a superseded `base`. `drainStaging` already cleared
`recheckSenders` for this cycle, so those abandoned candidates' senders would otherwise
vanish — a sender not touched again by a later block would never get rechecked until TTL.
`runRecheck` re-merges the senders of the unreached candidates back into staging
(`mergeRecheckSenders` under `stagingMu`) before returning, so the next cycle's `selectTxs`
re-picks their live pool txs. It does not touch `deferred`, which `capRecheckGroups` may have
already set this cycle.


### What this does *not* fix

`base` reads still fall through to the live memiavl tree that `Commit` mutates, so the
store guard must keep covering the whole `BaseApp.Commit()`. Item 1 of #2109 (narrow the
mutex to the `checkState` reset) stays blocked on making memiavl read-safe during commit.
Item 2 (lock-light `PoolSnapshot`) stays blocked on an SDK mempool change; PR #2156 only
collapses the RPC read fan-out.

## Phases

Each phase is a separate PR.

### Phase A — mempool-owned branched state (item 3b)

`mempoolState` + all three call sites + refresh at `Commit` + generation cancellation, as
described above. Atomic by necessity (decision 3).

### Phase B — per-sender grouping in `runRecheck`

Candidates are bucketed by first signer — the one `PriorityNonceMempool` orders by — keeping
first-appearance order across groups (so the front-loaded deferred prefix still runs first)
and pool order within a group. `maxRecheckBatch` is a soft cap applied at group boundaries
only, after grouping: once the running total of a cycle's group sizes would exceed the cap,
the remaining whole groups are deferred, never split mid-group — a sender's nonce chain always
runs to completion in one cycle even if it alone exceeds the cap. Encoding moved out of the
lock into the grouping pass.

`stateMu` is no longer held for a group's full duration — the issue's "hold the mutex across
the whole batch" variant, which reintroduces the admission stall #2118 removed, applies just
as much to one oversized group. Instead a group runs in bounded chunks of `recheckChunkSize`
candidates: `stateMu` is taken, `gen` is re-checked, up to one chunk runs, and the mutex is
released before the next chunk starts. This keeps every mutex hold short regardless of how
deep any one sender's queue is.

Cancellation granularity follows: `gen` is checked at the start of every chunk, not just once
per group, so a group can now abort mid-flight at a chunk boundary. An abort leaves the
unreached tail — from the aborted chunk's start onward — untouched for `recoverSenders`, the
same recovery path used for groups that never got a turn at all.

Cascade eviction inside a group: on a nonce failure (`ErrWrongSequence` from cosmos sig
verification, `ErrInvalidSequence` from the EVM ante) the remaining higher-nonce siblings are
evicted without spending a `RunTx` on each. Guarded, because the naive rule is wrong: a
wrong-sequence failure can mean either a *gap* (nonce too high — siblings are unreachable) or
a *stale* nonce (already committed — the successor may be exactly the expected one, and
cascading would evict valid txs). The cascade fires only when the gap is provable: some
earlier tx in the same group passed recheck this pass (so `lastOK + 1` is the account's next
expected nonce) and the failing tx's nonce is strictly greater. It is also disabled for any
group that isn't the signer's contiguous ascending view — unknown signer, repeated or
descending nonce, or a tx dropped on encode error. Counter:
`cronos.mempool.recheck.cascade_evicted`.

Residual, accepted, documented: an admission *of the same sender* can still interleave between
that sender's chunks, and between that sender's groups across cycles. It can bump the nonce in
`base` past the next chunk's first candidate, which then fails as stale and is evicted — but
because `PriorityNonceMempool.Remove` resolves by (sender, nonce) key rather than tx identity,
the tx actually dropped from the pool may be the freshly admitted replacement at that nonce, not
the stale candidate the recheck pass was validating (no cascade either way — a stale nonce is
exactly `lastOK + 1`, so the gap rule doesn't fire). This predates this change and is inherent to
key-based removal in the SDK pool — `BaseApp.RunTx` does the same key-based removal itself on a
ReCheck ante failure. Ordering there is inherently racy (the tx arrived concurrently), the effect
is node-local, and the client's resubmit resolves it. A cross-chunk cascade needs its own check
for the same reason: a gap proven in one chunk is only atomic with the admissions it must stay
consistent with for the duration of that chunk's own lock hold, not across the boundary into the
next one. `cascadeChunkLocked` therefore spends one `RunTx` on the next chunk's head before
blind-evicting anything in that chunk; if it now passes (the gap was filled by an admission that
landed between chunks), the chunk falls back to normal `recheckChunkLocked` semantics from
there, instead of evicting a now-valid nonce sight unseen.

### Phase C — split `Manager` (item 5)

Three files, split along the boundary A + B made real:

- `exec.go` — `txExec`: the mutex, `mempoolState`, `gen`, and the codecs/`EncoderCache`.
  Both halves execute through `runTxLocked`, so this state belongs to neither alone.
- `admitter.go` — `admitter`: `admit`, `insertTxHandler`, `checkTxHandler`, `cacheTx`.
- `scheduler.go` — `recheckScheduler`: staging (`stageRecheckSenders`,
  `stageSkippedSenders`, `drainStaging`), `selectTxs`/`evictForRecheck`/TTL,
  `groupCandidates`/`runGroup`/`runRecheck`, `recheckWorker`, deferred carry.

`Manager` becomes a thin facade holding `{exec, adm, sched}` and forwarding the public API,
so `app.go` and `MempoolProposalHandler` call sites don't churn. Composition is explicit
rather than embedded: three embedded structs would make field promotion depend on nesting
depth, which is not the kind of thing a lock-order invariant should rest on. Lock order is
unchanged — `recheckMu > txExec.mu > stagingMu`, with `mempoolState.mu` innermost.

### Phase D — differential tests

`app/proposal_diff_test.go` seeds two identical pools and runs the fast path
(`MempoolProposalHandler` + `CacheProposalTxVerifier` over a warm encoder cache) against the
default full-ante handler, then pins down where they may disagree. The pooled txs are a
local `diffTx` carrying its own signer, nonce, fee, gas, and timeout height, so no account
keeper or real codec is needed; the ante is a per-case predicate standing in for
`RunTx(PrepareProposal)`.

What the cases assert:

- All-valid pool, and a same-sender nonce gap: identical selections and identical pools. The
  gap guard lives in `DefaultProposalHandler`'s per-signer sequence tracking, which both
  paths share, so a divergence there would be a bug in the wiring.
- Stale nonce, recheck backlog (a whole stale prefix), timeout height: the fast path proposes
  txs the ante rejects, and leaves them pooled instead of evicting them mid-proposal —
  eviction is recheck's job. The default path drops and evicts them during the proposal.
- baseFee drift: selections match, because the proposal gate replaces the ante's fee check;
  only the pool differs, since a gated tx stays pooled for a later block.

Each divergent case also runs the real `ProcessProposalHandler` over the fast path's
proposal (with a non-empty blocklist, so per-tx validation actually executes) and asserts
ACCEPT: cronos `ProcessProposal` is blocklist-only, so an ante-invalid tx cannot make peers
reject the block — `FinalizeBlock` records it as a failed tx result.

## Verification (Phase A)

`go test -tags objstore -mod=mod ./app/... -race`, plus `go build -tags objstore -mod=mod ./...`.

- `manager_test.go` / `recheck_test.go`: assert `admit`, `CheckTxHandler`, and `runRecheck`
  all pass a non-nil `base`; assert a ReCheck write to `base` is visible to a later `admit`
  of a higher-nonce sibling (nonce continuity across the branch); assert refresh swaps
  `base` identity and bumps `gen`.
- `recheck_async_test.go`: a pass superseded mid-flight by a newer commit skips its
  remaining candidates and the next pass re-covers the staged senders; extend
  `TestTriggerRecheck_ConcurrentCommits` to run `admit` concurrently with commit + recheck
  under `-race`.
- Local integration: node with `mempool.type=app`, burst of same-sender sequential txs;
  confirm nonce continuity across blocks and that committed txs leave the pool; watch
  `cronos.mempool.recheck.*` for regressions.

## Changelog

```
- `[mempool]` run recheck and admission on a mempool-owned branched context
  ([#NNNN](https://github.com/crypto-org-chain/cronos/pull/NNNN))
```

under `## UNRELEASED` IMPROVEMENTS.

# Audit note — pre-existing BeginBlock/admission race on `EvmKeeper.eip155ChainID`

- **Severity:** data race (Go `-race` confirmed). Practical impact low (redundant
  same-value write), but undefined behavior and a `-race` failure.
- **Scope:** `mempool.type=app` path only. Independent of the Option B work
  (`app-mempool-admission-concurrency-plan.md`); surfaced by its phase-1 test.
- **Status:** documented; fix belongs in the ethermint fork.

## The race

Lock-free admission runs the EVM ante via `RunTx(ExecModeCheck)`. The ante calls
`EvmKeeper.ChainID()` / `EVMBlockConfig` (`evmd/ante/handler_options.go:88`),
which **reads** `Keeper.eip155ChainID`. Concurrently, every block's
`FinalizeBlock → BeginBlock → (*Keeper).WithChainID → WithChainIDString`
(`x/evm/keeper/keeper.go:170`) **writes** that same field.

These overlap because, on the app-mempool path, CheckTx/InsertTx are lock-free
versus consensus: CometBFT's `LockFreeContext` lets mempool admission skip the
`localClient` mutex that would otherwise serialize it against `FinalizeBlock`.
The cronos `Admitter` mutex serializes admission against `Commit`, but **not**
against `FinalizeBlock`, so `BeginBlock`'s write races the admission ante's read.

```
WRITE  BeginBlock → WithChainIDString → k.eip155ChainID = chainID   (FinalizeBlock)
READ   RunTx(ExecModeCheck) → EVMBlockConfig/ChainID()              (InsertTx/CheckTx)
```

Reproduce: unskip `TestAdmissionVsFinalizeBlockRace` in `app/admission_test.go`
(or run `app.TestInsertTxConcurrentAdmission` with a concurrent FinalizeBlock
loop) under `-race`.

## Why it's benign-ish but should be fixed

`WithChainIDString` re-derives the **same** chain ID every block and only panics
on a *different* value (`keeper.go:166`). So the write never changes the value —
it's a redundant store. Still: concurrent read/write of a pointer field is UB and
trips `-race`.

## Proposed fix (ethermint fork)

Make `WithChainIDString` a no-op when the chain ID is already set to the same
value — skip the write entirely:

```go
func (k *Keeper) WithChainIDString(value string) {
    chainID, err := ethermint.ParseChainID(value)
    if err != nil { panic(err) }
    if k.eip155ChainID != nil {
        if k.eip155ChainID.Cmp(chainID) != 0 {
            panic("chain id already set")
        }
        return // already set to the same value; no write, no race
    }
    k.eip155ChainID = chainID
}
```

This removes the per-block write after the first block, eliminating the race for
all readers (the ante on the lock-free admission path included). Set the chain ID
once at startup (already done via `App` wiring) and BeginBlock's call becomes a
guarded no-op.

Note: other BeginBlock-mutated keeper fields read by the ante (if any) would need
the same treatment; `eip155ChainID` is the one observed here.

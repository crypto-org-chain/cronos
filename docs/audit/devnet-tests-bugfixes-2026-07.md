# Devnet test-run bug fixes (2026-07)

Found by running `devnet_tests/` against a real local single-validator devnet
(pystarport, `scripts/cronos-single-devnet.yaml`). Five issues total: one real
chain-correctness bug, four test/config/isolation fixes. All verified passing
together in one combined run (see "Final verification" below).

## 1. Blob transactions were silently accepted instead of rejected

**Severity:** chain-correctness bug (the only one in this list that isn't a
test-only fix).

**Root cause:** `EthereumTx.Validate()` in
`vendor/github.com/evmos/ethermint/x/evm/types/eth.go` never checked
`tx.Type()`. An EIP-4844 blob transaction has no `TxData` representation in
this ethermint fork (`NewTxDataFromTx` in `tx_data.go` has no `BlobTxType`
case, and is dead code anyway — zero callers in the whole codebase). Because
`Validate()` didn't reject the type up front, a blob tx passed
`ValidateBasic()` and got broadcast/accepted as if it were a plain transfer,
silently dropping its blob-specific fields (`BlobHashes`, sidecar).

**Fix:** added an explicit type check at the top of `Validate()`:

```go
if tx.Type() == ethtypes.BlobTxType {
    return errorsmod.Wrap(ethtypes.ErrTxTypeNotSupported, "blob transactions are not supported")
}
```

File: `vendor/github.com/evmos/ethermint/x/evm/types/eth.go`.

**Important build gotcha for verifying this fix:** `make build` passes
`-mod=readonly` explicitly, which disables Go's automatic vendor-mode
detection — so `make build` silently ignores this edit and resolves
`github.com/evmos/ethermint` from the module cache/proxy instead of
`vendor/`. To actually exercise a vendor patch locally, build with:

```
go build -mod=vendor -tags "netgo objstore pebbledb mainnet" -o build/cronosd ./cmd/cronosd
```

(tags match `make build LEDGER_ENABLED=false`'s tag set). Confirmed the
difference directly: `grep -a -o "blob transactions are not supported"
build/cronosd` returned nothing after `make build`, and returned the string
after the `-mod=vendor` build.

**Verification:** `devnet_tests/test_eip_behavior.py::test_blob_tx_rejected`
— PASSED against a devnet running the `-mod=vendor` binary.

## 2. `test_below_base_fee_rejected` was flaky and asserted the wrong error text

File: `devnet_tests/devnet_tests/eip_probes.py`,
`devnet_tests/devnet_tests/test_eip_behavior.py`.

**Root cause (flakiness):** the probe queried `baseFeePerGas` once, then
submitted a tx with `maxFeePerGas = base_fee - 1`. On an idle devnet the
feemarket module decays the base fee every empty block, so that 1-unit margin
could be erased between the query and the tx actually landing in `CheckTx` —
flipping the tx from correctly-rejected to accepted, purely from timing.

**Root cause (wrong assertion):** the test asserted
`"insufficient gas prices" in result.error`. The real ante-handler error
(from `ethermint/ante.CheckEthCanTransfer`) is:
`"max fee per gas less than block base fee (X < Y): insufficient fee: insufficient fee"`.

**Fix:**
- `send_below_base_fee` now uses `maxFeePerGas = base_fee // 2` instead of
  `base_fee - 1`, giving enough margin to survive one block's decay.
- The test's assertion now checks for
  `"max fee per gas less than block base fee"`.

**Verification:** `devnet_tests/test_eip_behavior.py` (all 5 tests) —
PASSED, rerun multiple times without flaking.

## 3. `txpool_status`/`txpool_content` RPC calls errored with a missing-`result` KeyError

File: `scripts/cronos-single-devnet.yaml`.

**Root cause:** the devnet's `json-rpc.api` list was
`"eth,net,web3,debug,cronos"` — no `txpool` namespace, so any
`txpool_*` JSON-RPC call errored at the transport level (no `result` key at
all, not even an RPC error), which any caller doing `rsp["result"]` would hit
as a `KeyError`.

**Fix:** added `txpool` to the `api` list:
`"eth,net,web3,debug,cronos,txpool"`.

**Verification:** `curl txpool_status` returns a valid (non-erroring) JSON-RPC
result after the config change.

## 4. Mempool-saturation test left the shared account's nonce desynced for later tests

File: `devnet_tests/devnet_tests/mempool_probes.py`.

**Root cause:** `saturate_pool` fired a burst of txs and returned
immediately, without waiting for them to land on-chain. Because
`funded_account` is shared across the whole test session, any accepted-but-
not-yet-committed txs left the account's actual nonce out of sync with what
later tests assumed via `get_transaction_count(..., "pending")`. This showed
up as a downstream nonce-race failure in
`test_security_behavior.py::test_unauthorized_cro_bridge_call_is_rejected`
when run after the mempool tests in the same session.

**Fix:** `saturate_pool` now polls
`get_transaction_count(account, "latest")` after sending the burst and blocks
(up to `drain_timeout=30s`) until all accepted txs are actually committed,
before returning.

**Verification:** `test_mempool_behavior.py` + `test_security_behavior.py`
run together — PASSED, no nonce desync.

## 5. `test_pool_saturation_reports_growth` asserted a metric that's unobservable on this devnet config

File: `devnet_tests/devnet_tests/test_mempool_behavior.py`.

**Root cause:** the test asserted `result.pool_pending > 0` after a
300-tx burst. `txpool_status`'s `pending` count
(`vendor/github.com/evmos/ethermint/rpc/namespaces/ethereum/txpool/api.go`,
`PublicAPI.Status()`) only reflects a real count when
`api.mempoolClient` is non-nil, which requires `mempool.type: app`
(the app-level custom mempool) to be configured.
`scripts/cronos-single-devnet.yaml` has no `mempool:` section at all (default
CometBFT mempool), so `mempoolClient` is always `nil` here and `pending` is
permanently hard-coded to `0` — a config limitation, not a timing bug.
(Enabling `mempool.type=app` on this devnet profile is out of scope: it's a
separate, already-paused investigation — see project memory on the v0.54
`mempool.type=app` TPS-regression work.)

**Fix:** changed the assertion to check the actually-observable signal —
`result.accepted == SATURATION_BATCH` (the burst was fully absorbed at
submission time) — with a comment explaining why `pool_pending` can't be
used here.

**Verification:** `test_mempool_behavior.py` + `test_security_behavior.py`
run together — PASSED.

## 6. `test_register_ica_with_unknown_connection_is_rejected` — not a bug, precompile is intentionally unregistered

File: `devnet_tests/devnet_tests/test_ica_behavior.py`.

Investigated why this test's ICA-registration call was accepted
(`receipt.status == 1`, no logs) instead of reverted.

**Finding:** the ICA precompile (address `0x66`) was deliberately removed
from `app.go`'s `CustomContractFn` list in commit `6d9579f2`
("chore: remove unused precompiles (#1986)"), which also disabled the
equivalent test at `integration_tests/test_ica_precompile.py` with
`pytest.skip(..., allow_module_level=True)`. Verified independently at the
IBC-msg level (bypassing the EVM path) with:

```
cronosd tx interchain-accounts controller register connection-999999999 ...
```

which correctly errors `connection-999999999: connection not found` — the
IBC connection-lookup logic itself is fine. But since no precompile is
registered at `0x66`, an EVM `CALL` to that address just hits an empty
account with no code, which the EVM reports as a no-op success
(`status == 1`, empty output) — there is no handler left to reject anything.

**Fix:** applied the same skip pattern used in `integration_tests/`:

```python
pytest.skip("ica precompile is not registered in app.go, see 6d9579f2", allow_module_level=True)
```

**Verification:** `test_ica_behavior.py` — 1 skipped (was: 1 failed).

## Final verification — full suite, one devnet, all fixes together

```
devnet_tests/test_eip_behavior.py::test_max_tx_gas_rejected PASSED
devnet_tests/test_eip_behavior.py::test_floor_data_gas_rejected PASSED
devnet_tests/test_eip_behavior.py::test_below_base_fee_rejected PASSED
devnet_tests/test_eip_behavior.py::test_insufficient_balance_rejected PASSED
devnet_tests/test_eip_behavior.py::test_blob_tx_rejected PASSED
devnet_tests/test_mempool_behavior.py::test_nonce_gap_rejected_at_submission PASSED
devnet_tests/test_mempool_behavior.py::test_pool_saturation_reports_growth PASSED
devnet_tests/test_rpc_diff.py::test_rpc_diff_equivalence SKIPPED (needs >=2 nodes)
devnet_tests/test_security_behavior.py::test_unauthorized_cro_bridge_call_is_rejected PASSED
devnet_tests/test_state_safety.py::test_app_hash_agreement SKIPPED (needs >=2 nodes)
devnet_tests/test_state_safety.py::test_historical_query_soak PASSED
devnet_tests/test_ica_behavior.py::test_register_ica_with_unknown_connection_is_rejected SKIPPED (precompile removed)

9 passed, 3 skipped
```

The 3 skips are all by design on a single-node devnet / with the ICA
precompile intentionally absent — not failures.

## Files changed

- `vendor/github.com/evmos/ethermint/x/evm/types/eth.go` — blob-tx rejection (chain fix)
- `devnet_tests/devnet_tests/eip_probes.py` — base-fee margin fix
- `devnet_tests/devnet_tests/test_eip_behavior.py` — base-fee assertion fix
- `scripts/cronos-single-devnet.yaml` — added `txpool` RPC namespace
- `devnet_tests/devnet_tests/mempool_probes.py` — drain-before-return fix
- `devnet_tests/devnet_tests/test_mempool_behavior.py` — re-scoped saturation assertion
- `devnet_tests/devnet_tests/test_ica_behavior.py` — skip (precompile removed upstream)

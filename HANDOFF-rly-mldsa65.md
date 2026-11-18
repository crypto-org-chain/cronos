# Handoff: let rly relay after ML-DSA-65 validator rotation

## Status on 2026-11-18: done

- crypto-org-chain/relayer#22 (CometBFT v0.38.26) is merged at `35da7da1`.
- Cronos `nix/sources.json` pins `relayer` at `3903db75`, an ancestor of the merge commit (crypto-org-chain/cronos#2263).
- The test is `integration_tests/test_ibc_mldsa65.py` + `configs/ibc_mldsa65.jsonnet`, which runs both rly and Hermes on a Cronos↔Cronos pair.
- Hermes is fixed too (crypto-org-chain/hermes#11, see `HANDOFF-hermes-mldsa65.md`). The upgrade-notes item at the end is obsolete: the migration doc §5.3 now names both forks.
- Next: chain-main moves to CometBFT v0.38.26 (chain-main `HANDOFF-cometbft-v0.38.26-mldsa65.md`). Then re-pin chain-main in Cronos nix and add a Cronos↔chain-main case.

The rest of this doc is the original plan, kept for context.

## Goal

Make the crypto-org-chain/relayer fork (`rly`) relay IBC packets to Cronos after a validator rotates its consensus key to ML-DSA-65. Then cover it with a Cronos integration test.

## Why

- Cronos v1.9 (draft PR crypto-org-chain/cronos#2263, branch `feat/sdk-v0.55`) lets validators rotate ed25519 → ML-DSA-65 (`MsgRotateConsPubKey`).
- Neither relayer pinned in Cronos nix can decode ML-DSA-65 validator keys, so IBC channels stop updating once any validator rotates.
  - hermes v1.13.1 (`nix/hermes.nix`; latest release is v1.13.3): blocked upstream. It needs the open, unreviewed PRs cometbft/tendermint-rs#1532 (proto `mldsa65`) and cometbft/tendermint-rs#1533 (signature verification), then a Hermes release. informalsystems/hermes#4435 is unrelated: it adds extra tx signatures, not header key support.
  - rly: pinned in `nix/sources.json` → `relayer` = crypto-org-chain/relayer@fe5722292a (main, last commit 2024-05-27). Its go.mod has `cometbft v0.38.7-0.20240412124004-1f67e396cf45`, SDK v0.50.5, ibc-go v8.2.0 and strangelove `cometbft-client v0.1.0`.

## Key finding (checked 2026-11-15)

Upstream CometBFT **v0.38.26** (2026-08-13) backported ML-DSA-65 (cometbft/cometbft#6028). Its wire format matches the crypto-org fork Cronos runs (`crypto-org-chain/cometbft@34b0a74b4248`, cometbft v0.40 line):

- `proto/tendermint/crypto/keys.proto`: `bytes mldsa65 = 4` in both.
- Amino names `cometbft/PubKeyMlDsa65` and `cometbft/PrivKeyMlDsa65` in both. The JSON type is registered in `crypto/mldsa65` init.
- Address = `tmhash.SumTruncated(pubKey.Bytes())`; Verify = `mldsa65.Verify(pk, msg, nil, sig)` in both.
- v0.38.26 `crypto/encoding/codec.go` and `types/params.go` import `mldsa65`, so a plain dependency bump registers the key type.

Signature verification of headers happens on the destination chain's 07-tendermint client (Cronos's ibc-go v11 + fork cometbft), not in rly. rly only has to decode validator sets from CometBFT RPC JSON and proto-encode them into the header.

## Plan

1. In crypto-org-chain/relayer: bump `github.com/cometbft/cometbft` to `v0.38.26`, then `go mod tidy`, `go build ./...` and the repo's tests. Watch for transitive bumps; the fork is stale. Check whether `cometbft-client` needs a bump to decode the new key type from RPC JSON.
2. Open a PR there (ask the user first, and run `pre-pr-review` before it).
3. In Cronos (`feat/sdk-v0.55` or a follow-up PR): point `nix/sources.json` `relayer` at the new rev (update `sha256` too).
4. Add an integration test: rotate a validator to ML-DSA-65, then relay a packet with rly. Reuse:
   - `integration_tests/test_cons_key_rotation.py`: `signed_rotations_tx`, `gen_mldsa65_key`, `swap_consensus_keys`, `update_validator_pubkey_types` (in `utils.py`). Validators must allow `["ed25519", MLDSA65]` in consensus params first.
   - rly network setup: `prepare_network(..., relayer=cluster.Relayer.RLY.value)` in `integration_tests/ibc_utils.py`, see `test_ibc_rly.py`.

## Local test-running notes

- Run pytest from the **repo root** (`pytest integration_tests/test_x.py`), not from `integration_tests/`. The nix `cronosd` is coverage-instrumented and writes "coverage dir missing" errors into the output pystarport parses as JSON when started from `integration_tests/`.
- Use a short `--basetemp` (e.g. `/tmp/x`) for the supervisord socket path limit, and set `TMPDIR=/tmp`.
- `integration_tests/contracts` must be built (`npm ci && npm run build` in a nix shell) for tests that deploy contracts.
- Measured: an IBC client update with an ML-DSA-65 header needs ~201k gas, just over the 200k default, so pass `gas=1000000`.

## State of crypto-org-chain/cronos#2263 at handoff

- Local commit `98524c05` (test: rotate both validators in one multi-signer tx) is reviewed clean and gate-marked. It is **not pushed**; ask before pushing.
- Upgrade notes should mention that Hermes can't relay after an ML-DSA-65 rotation, and argue for keeping ML-DSA-65 out of `pub_key_types` at the upgrade.

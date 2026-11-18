# Handoff: let Hermes relay after ML-DSA-65 validator rotation

## Status on 2026-11-18: done

- crypto-org-chain/hermes#11 is merged at `cd4664d8` (option A: tendermint-rs v0.40 backport from JayT106/tendermint-rs rev `abbf51ca`).
- Cronos `nix/sources.json` pins `hermes` at `cd4664d8` (crypto-org-chain/cronos#2263).
- The test is `integration_tests/test_ibc_mldsa65.py` + `configs/ibc_mldsa65.jsonnet`, which runs both Hermes and rly on a Cronos↔Cronos pair.
- The v1.8→v1.9 migration doc §5.3 names the Hermes fork commit.
- Still open:
  - cometbft/tendermint-rs#1532/#1533 are unmerged.
  - Move the tendermint-rs patch from the personal fork to an org fork or upstream before mainnet.
  - Hermes needs `compat_mode = '0.38'` for Cronos (it has no `0.40` mode).
  - The chain-main counterparty work is in chain-main `HANDOFF-cometbft-v0.38.26-mldsa65.md`.

The rest of this doc is the original plan, kept for context.

## Goal

Make the crypto-org-chain/hermes fork relay IBC packets to and from Cronos after a validator rotates its consensus key to ML-DSA-65:

1. Sync the fork with upstream informalsystems/hermes v1.13.3.
2. Build it against tendermint-rs with cometbft/tendermint-rs#1533 (which includes #1532), served from the user's fork https://github.com/JayT106/tendermint-rs.
3. Prove it with a Cronos integration test, then point Cronos's nix at the fork build.

## Why

- Cronos v1.9 (crypto-org-chain/cronos#2263, branch `feat/sdk-v0.55`) lets validators rotate ed25519 → ML-DSA-65 (`MsgRotateConsPubKey`).
- Hermes decodes validator sets and builds light-client headers with tendermint-rs. Released tendermint-rs has no ML-DSA-65 key type, so the IBC client of Cronos stops updating once any validator rotates.
- Header signatures are verified on the destination chain (ibc-go 07-tendermint). Hermes also verifies them itself in its light client (`tendermint-light-client-verifier`), so it needs both #1532 (proto `PublicKey.mldsa65`) and #1533 (verification).
- rly is already fixed (crypto-org-chain/relayer#22) and has a Cronos test. Hermes is the default relayer in Cronos tests (`nix/hermes.nix`, v1.13.1 release tarball).

## State checked on 2026-11-16

- **crypto-org-chain/hermes**: default branch `master` at `a653c86f38`. It already contains the upstream `v1.13.3` tag: compare `v1.13.3...master` = 9 ahead, 0 behind. The 9 are 5 upstream post-tag master commits (dependabot bumps, `ci: fix namada release URL`) plus 4 fork commits: `feat: nix compability (#3)`, `feat: remove MAX_TOTAL_DELAY in fibonacci backoff (#4)`, `fix ci (#8)`, `fix bug EventBus::broadcast (#6)`. The fork has no tags. Re-check before syncing; the sync may be a no-op, or the user may want a branch at exactly `v1.13.3` plus the fork commits (cherry-pick `-x`, keep authors; don't squash).
- **Hermes v1.13.3 deps**: all tendermint crates `0.40.0` (`tendermint`, `-light-client`, `-light-client-detector`, `-light-client-verifier`, `-proto`, `-rpc`, `-testgen`); `ibc-proto = "0.51.0"`; `rust-toolchain.toml` channel `1.85.0`. `Cargo.toml` already has a commented `[patch.crates-io]` block for tendermint-rs git deps.
- **tendermint-rs PRs** (both OPEN, author zmanian, base `main` = `0.41.0`, MSRV 1.82):
  - #1532 `zaki/proto-v0.38.26`: regenerates v0_38 protos from CometBFT v0.38.26 (adds `PublicKey.mldsa65`).
  - #1533 `zaki/mldsa65-verify`: contains #1532; review only its last commit, "tendermint: verify ML-DSA-65 validator signatures". Adds `PublicKey::MlDsa65` and `Algorithm::MlDsa65` behind a new `mldsa65` feature (also a passthrough feature on `tendermint-light-client-verifier`). Uses RustCrypto `ml-dsa` 0.1.2, which needs **Rust 1.87**. Without the feature, ML-DSA validator sets parse but fail with an error.
- **JayT106/tendermint-rs**: fork of cometbft/tendermint-rs with only a `main` branch. The #1533 branch is not pushed there yet.

## Version mismatch to resolve first

Hermes is on tendermint-rs 0.40.0, the PRs target 0.41.0. Two options:

- **A (recommended): backport onto 0.40.** In JayT106/tendermint-rs, branch from tag `v0.40.0` (e.g. `mldsa65-v0.40`), cherry-pick the #1532 and #1533 commits with `-x`, fix conflicts, run the crate tests. In hermes, uncomment `[patch.crates-io]` and point every tendermint crate at that git branch or rev. `[patch]` keeps the version at 0.40, so `ibc-proto` 0.51 keeps resolving to the patched `tendermint-proto`.
- **B: move hermes to 0.41.** Push the #1533 branch as-is to the fork, bump hermes to tendermint 0.41, and bump `ibc-proto` to a release built on tendermint-proto 0.41 (check crates.io). Bigger diff, more API churn.

Check `cargo tree -d | grep tendermint`: there must be exactly one copy of each tendermint crate.

## Hermes changes

- Enable `mldsa65` on `tendermint` and `tendermint-light-client-verifier` (and any crate whose feature list needs it).
- Bump `rust-toolchain.toml` to ≥ 1.87 and the crates' `rust-version` where set; fix new clippy lints.
- Fix compile errors from the new `PublicKey`/`Algorithm` variants (exhaustive matches, key conversions to protobuf/`Any`). Grep `PublicKey::`, `Algorithm::`, `Ed25519` in `crates/relayer` and `crates/relayer-types`.
- The enum grows to about 2 KB with the feature on (`PublicKey` stays `Copy`); watch for large stack copies in hot paths.
- `cargo build --release`, `cargo test --all-features`, `cargo clippy`.

## Cronos test and nix pin

- `nix/hermes.nix` downloads the upstream release tarball. Switch it to build the fork from source (rev + hashes), or to a fork release binary. Keep it working on aarch64-darwin and x86_64-linux.
- Write `integration_tests/test_ibc_hermes_mldsa65.py` modeled on the rly one already on `feat/sdk-v0.55`: `integration_tests/test_ibc_rly_mldsa65.py` + `configs/ibc_rly_mldsa65.jsonnet` (Cronos↔Cronos pair; chain-main's cometbft v0.38.17 can't decode ML-DSA-65). Reuse `test_cons_key_rotation.py` helpers (`signed_rotations_tx`, `gen_mldsa65_key`, `swap_consensus_keys`, `update_validator_pubkey_types` in `utils.py`). Validators must allow `["ed25519", MLDSA65]` in consensus params first.
- Run the existing hermes IBC tests (`test_ibc.py`, `test_ibc_update_client.py`) against the new pin.
- Update the v1.8→v1.9 migration doc §5.3 (relayer support) once Hermes works.

## Local test notes

- Run pytest from the repo root, with a short `--basetemp` (e.g. `/tmp/x`) and `TMPDIR=/tmp`.
- Build `integration_tests/contracts` first (`npm ci && npm run build` in a nix shell) for tests that deploy contracts.
- An IBC client update with an ML-DSA-65 header needs ~201k gas; set relayer gas to 1000000.
- Counterparties need cometbft ≥ v0.38.26 or v0.40 to accept ML-DSA-65 Cronos headers.

## Rules

- Ask before any push or PR (hermes fork, tendermint-rs fork, Cronos). Run `pre-pr-review` before each PR.
- No Claude attribution in commits or on GitHub. Draft GitHub replies in chat; post only with the user's OK.
- Cronos worktree hooks: one plain git command per Bash call; use Edit/Write, not shell writes; nix commands go in scratchpad scripts.

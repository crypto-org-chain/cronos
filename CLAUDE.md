# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Cronos is an EVM-compatible Cosmos SDK blockchain (the Crypto.org EVM chain). It is built on top of
**Ethermint** (EVM execution, imported from `github.com/crypto-org-chain/ethermint`, a fork) and the
**Cosmos SDK** (v0.54.x), with **CometBFT** consensus and **IBC-go v11**. The daemon binary is `cronosd`.

## Development workflow

### Build, test, lint

```bash
make build          # -> build/cronosd  (respects NETWORK=mainnet|testnet, LEDGER_ENABLED)
make install        # go install ./cmd/cronosd
make test           # go unit tests, ALWAYS with -tags=objstore
make lint           # golangci-lint (v2.1.6) + go mod verify
make lint-fix       # auto-fix Go lint issues
make lint-py        # flake8 for integration_tests python
make lint-py-fix    # isort + black
make vulncheck      # govulncheck ./... over the resolved module graph
```

**The `objstore` build tag is required for tests.** Running `go test ./...` without it will fail to
compile parts of the store layer. Use:

```bash
go test -tags=objstore ./app/...                       # a package
go test -tags=objstore -run TestName ./x/cronos/keeper # a single test
make test-race-mempool                                 # mempool concurrency tests (needs -race)
```

Default build tags: `netgo objstore pebbledb mainnet nativebyteorder` (+ `ledger` if gcc present).
`COSMOS_BUILD_OPTIONS=rocksdb make install` builds with the RocksDB backend (needs cgo + rocksdb libs);
a bare `go build`/`ls` in this repo may print harmless "Package rocksdb was not found" pkg-config
warnings from cgo probing — ignore them.

### Nix / dev environment

Building and CI rely heavily on **Nix**. `gomod2nix.toml` mirrors `go.mod` — after changing Go deps run
`gomod2nix generate` to regenerate it, or CI will fail.

```bash
nix develop            # default dev shell (go, gomod2nix, nixfmt)
nix develop .#rocksdb  # adds rocksdb libs for the rocksdb backend
nix develop .#full     # adds the integration-test environment (test-env)
```

### Integration tests (Python / pytest, via Nix)

Integration tests live in `integration_tests/` and drive real `cronosd` nodes with **pystarport**.
They are orchestrated with Nix, not plain pytest.

```bash
make run-integration-tests                    # runs everything via scripts/run-integration-tests
TESTS_TO_RUN=upgrade make run-integration-tests  # run only tests with a given pytest marker

# Manual, inside the nix shell:
nix-shell ./integration_tests/shell.nix
cd integration_tests && pytest -k test_basic      # select by name/marker
pytest -k cronos    # some tests run on both geth and cronos; filter by platform
```

`TESTS_TO_RUN` maps to pytest markers (see `integration_tests/pytest.ini`). Node topologies are defined
by jsonnet configs in `integration_tests/configs/`.

### Protobuf

Proto sources are in `proto/{cronos,e2ee,memiavl}`. Generation runs in a Docker proto-builder image:
```bash
make proto-gen           # regenerate Go from .proto
make proto-lint          # buf lint
make proto-format        # clang-format
make proto-check-breaking
```
`memiavl` protos are checked for breaking changes against the external `cronos-store` repo.

## Architecture

### App wiring (`app/app.go`)
`CronosApp` composes standard Cosmos SDK modules + Ethermint's `x/evm` and `x/feemarket` + the two
custom modules below. Read `app/app.go` to find keeper wiring, store keys, ante handlers, and upgrade
handler registration. Related app-level files:
- `app/upgrades.go` — chain upgrade handlers (each network upgrade adds a handler here).
- `app/forks.go`, `app/unblockable.go`, `app/block_address.go` — block-level address blocking / forks.
- `app/mempool/` — a custom app-side mempool with lock-free admission (`preverify.go`), gossip, and
  encode/decode caches. Concurrency-sensitive; changes here must pass `make test-race-mempool`.

### Custom store backends (the performance core)
Cronos does **not** use the vanilla IAVL store for production. Two swappable subsystems, both wired in
`app/app.go` and selected by app options / CLI flags:
- **memiavl** — an optimized IAVL replacement (from `github.com/crypto-org-chain/cronos-store`).
  Set up via `memiavlstore.SetupMemIAVL(...)`. Note: memiavl's in-memory cache is only enabled when
  neither block-stm nor optimistic execution is active (it is not concurrency-safe).
- **versiondb** — a separate historical-state store fed by a `StreamingService` (enabled with
  `versiondb.enable`). When on, a custom store loader (`app/storeloader.go`) constrains the loaded IAVL
  version to not exceed the versiondb version to avoid gaps. Managed via `cronosd` subcommands in
  `cmd/cronosd/cmd/versiondb.go`.
DB opening / migration lives in `cmd/cronosd/opendb/` and `cmd/cronosd/dbmigrate/`.

### `x/cronos` — the glue module (`x/cronos/`)
The core custom module. It "glues IBC, gravity bridge, and Ethermint together through hooks and token
mapping" (see `x/cronos/spec/`). Key responsibilities:
- **Token mapping**: converts IBC / gravity assets to CRC20 (ERC20) contracts on arrival, and handles
  the 8↔18 decimal conversion for the CRO gas token. State layout is in `x/cronos/spec/02_state.md`.
- **EVM hooks** (`keeper/evm_hooks.go`): `LogProcessEvmHook` translates specific contract event logs
  into native Cosmos module calls (e.g. sending tokens over IBC from a contract).
- **Stateful precompiles** (`keeper/precompiles/`): `bank`, `ica`, `relayer` precompiled contracts that
  let EVM contracts call native modules. They use `ExtStateDB.ExecuteNativeAction` to run SDK logic
  inside EVM execution. `keeper/permissions.go` gates who can call what.
- **Middleware** (`middleware/`) and IBC callback wiring (`keeper/ibc.go`).

### `x/e2ee` — end-to-end encryption module (`x/e2ee/`)
Lets users register encryption public keys on-chain and exchange encrypted messages. Includes a keyring
integration (`x/e2ee/keyring/`) and `autocli` command wiring.

### Entry point & CLI (`cmd/cronosd/`)
`main.go` → `cmd/root.go` builds the root command. Custom subcommands beyond the standard SDK set:
`versiondb`, `migrate_db`/`database`/`patch_db` (DB maintenance), under `cmd/cronosd/cmd/`.
Address bech32 prefixes are network-dependent (`cmd/cronosd/config/prefix_{mainnet,testnet}.go`),
selected by the `NETWORK` build tag.

## Dependencies & security-review scope

Most of Cronos's attack surface is **not in this repo** — it lives in a handful of large upstream
dependencies, and for the most important ones Cronos runs **`crypto-org-chain` forks pinned via
`replace` directives in `go.mod`, not the upstream versions**. When reviewing code, auditing behavior,
or scanning for vulnerabilities, you MUST read the forked source (the `replace` target), not the
upstream project on GitHub — the fork can and does differ. Resolve the actual code with
`go list -m -f '{{.Dir}}' <module>` or read it under `vendor/`.

| Concern | Module (import path) | Actual source after `replace` | Pinned version |
|---|---|---|---|
| Cosmos SDK (app framework, auth/bank/gov/staking, ante handlers, baseapp) | `github.com/cosmos/cosmos-sdk` | **`github.com/crypto-org-chain/cosmos-sdk`** (fork) | `v0.50.6-...20260612214333-941a8f1d05d0` |
| EVM execution & JSON-RPC (`x/evm`, `x/feemarket`, statedb) | `github.com/evmos/ethermint` | **`github.com/crypto-org-chain/ethermint`** (fork) | `v0.22.1-...20260702171011-a639532d9759` |
| Consensus / networking / mempool | `github.com/cometbft/cometbft` | upstream (no replace) | `v0.39.4-...20260526181141` |
| Custom store: memiavl, versiondb, store | `github.com/crypto-org-chain/cronos-store/{memiavl,versiondb,store}` | **`crypto-org-chain/cronos-store`** (fork target pins) | `...20260529153812-1f2f45ec5a3c` |
| EVM crypto / core types | `github.com/ethereum/go-ethereum` | **`github.com/crypto-org-chain/go-ethereum`** (fork) | `v1.10.20-...20260521015249` |
| IBC | `github.com/cosmos/ibc-go/v11` | upstream | `v11.1.0` |

**Security-review scope, in priority order:**
1. **`x/cronos` (this repo)** — precompiles, EVM hooks, token mapping, permissions: the bespoke,
   highest-risk custom logic (see architecture section).
2. **cosmos-sdk fork** — ante handlers, baseapp CheckTx/mempool path, module keepers. The fork diverges
   from upstream (e.g. mempool insert-before-commit, staking end-block changes — see CHANGELOG); audit
   the fork's diff, not upstream.
3. **ethermint fork** — EVM state transition, gas/fee logic (EIP-1559 floor-data-gas, EIP-7702), signer
   pre-verification. Consensus- and value-critical.
4. **cronos-store fork (memiavl/versiondb)** — state commitment and historical state; correctness bugs
   here corrupt state or break determinism.
5. **cometbft, go-ethereum, ibc-go** — larger blast radius but less Cronos-specific customization.

When checking whether a known CVE applies, compare against the **pinned fork commit**, not the nominal
upstream version string — the `v0.50.6`/`v1.10.20` prefixes are base tags; the real code is the pseudo-
version commit hash. `make vulncheck` (`govulncheck ./...`) scans the resolved module graph.

## Code style

Style is **enforced by `make lint`** (golangci-lint v2.1.6, config in `.golangci.yml`) — treat that
config as the source of truth and run `make lint-fix` before committing rather than hand-formatting.
The rules below are the ones the linters actually enforce here:

- **Formatting: `gofumpt` (with `extra-rules`) + `gci`.** Stricter than `gofmt`; let the tool format.
- **Import grouping (`gci`, custom order).** Four blocks, in this exact order, each separated by a blank
  line: (1) standard library; (2) third-party — this includes `github.com/crypto-org-chain/...`,
  `evmos/ethermint`, `ibc-go`, and `ethereum/go-ethereum`; (3) `cosmossdk.io/*`;
  (4) `github.com/cosmos/cosmos-sdk/*`. See `x/cronos/keeper/keeper.go` for the canonical layout.
- **Errors — sentinel + wrap.** Register module errors in `types/errors.go` with
  `errors.Register(ModuleName, code, msg)`, where `code` comes from a private `iota` block (code 1 is
  reserved for internal errors). Wrap at call sites with `errorsmod.Wrap`/`Wrapf`, aliasing
  `cosmossdk.io/errors` as `errorsmod`. `errorlint` is on: wrap dynamic errors with `%w` and compare
  with `errors.Is`/`errors.As` — never `==` or a type assertion on an error.
- **Doc comments on exported identifiers.** `revive`'s `exported` rule runs at *error* severity, so
  exported funcs/consts need a doc comment (bare exported types without any comment are tolerated).
- **No repeated string literals** — extract a `const` (`goconst`).
- **Tests: table-driven with `testify` suites.** Use a `suite.Suite` (e.g. `CronosTestSuite`) and a
  `testCases` slice of anonymous structs carrying a `name` and a `malleate func()` plus expected
  outcomes. Test/setup helpers must call `t.Helper()` (`thelper`).
- **Other enforced linters:** `misspell` (US spelling), `unconvert` (no redundant conversions),
  `nakedret` (avoid naked returns), `ineffassign`, `copyloopvar`, and `nolintlint` — every `//nolint`
  must be used and target a specific linter (`allow-unused: false`). `gosec` runs but G101/G107/G404
  (math/rand) and G115 (integer-overflow conversion) are intentionally excluded.

## Conventions

- **CHANGELOG.md is mandatory for non-trivial PRs.** Add an entry under `## UNRELEASED` in the correct
  section (`Improvements`, `Bug fixes`, `Chores`) formatted as
  `* [#PR](https://github.com/crypto-org-chain/cronos/pull/PR) description`. Entries describe the net
  user-facing effect, not the code change.
- Commit / PR messages use conventional-commit prefixes: `feat`, `fix`, `chore`, often scoped
  (`fix(ante):`, `feat(app):`, `fix(versiondb):`).
- ABCI / consensus-affecting changes generally must be gated behind an upgrade handler in
  `app/upgrades.go` — do not change deterministic behavior unconditionally.
- Ethermint is a pinned fork; bumping it uses `scripts/go-update-ethermint.sh`. After any Go dep change,
  regenerate `gomod2nix.toml`.

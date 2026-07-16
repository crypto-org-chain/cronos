# 3-Validator Devnet via pystarport (in nix-shell)

Sets up a local 3-validator Cronos devnet using `pystarport`, tuned with the
throughput-oriented settings from the
[V1.4 Benchmark wiki page](https://github.com/crypto-org-chain/cronos/wiki/V1.4-Benchmark)
("3 Validators" section: `config_patch` / `app_patch` / `genesis_patch`).

This mirrors the standard `integration_tests` pystarport workflow
(see `docs/integration-test.md`), just with a benchmark-tuned config instead
of `configs/default.jsonnet`.

Note: the wiki's benchmark numbers were produced by the separate
`testground/` load-testing framework (its own Docker/k8s harness), not by
pystarport directly. The config below re-applies the same CometBFT / app /
genesis tuning to a real pystarport devnet so you can drive load against it
locally (e.g. with the `testground/benchmark` CLI's load generator, or your
own tx spammer).

## 1. Install Nix (one-time)

```shell
sh <(curl -L https://nixos.org/nix/install) --daemon
source ~/.nix-profile/etc/profile.d/nix.sh   # add to shell profile too
```

## 2. Install cachix and configure binary caches (one-time)

```shell
nix-env -iA cachix -f https://cachix.org/api/v1/install
cachix use cronos
cachix use dapp   # needed on newer macOS
```

## 3. Clone the repo

```shell
git clone https://github.com/crypto-org-chain/cronos.git
cd cronos
```

## 4. Add the benchmark config

Create `integration_tests/configs/benchmark-3val.jsonnet` (full contents
below) — a copy of `integration_tests/configs/default.jsonnet` with the
wiki's 3-validator benchmark tuning applied uniformly to all 3 validators.

## 5. Enter nix-shell

`pystarport` and `cronosd` are provided by the nix shell — no extra install
step needed.

```shell
nix-shell integration_tests/shell.nix
```

## 6. Compile the jsonnet config and inspect it (optional)

```shell
<nix-shell> $ jsonnet integration_tests/configs/benchmark-3val.jsonnet | yq -P
```

## 7. Initialize the 3-validator devnet

```shell
<nix-shell> $ pystarport init \
    --config integration_tests/configs/benchmark-3val.jsonnet \
    --data /tmp/cronos-benchmark-3val \
    --base_port 26650 \
    --no_remove
```

This generates 3 validator homes under
`/tmp/cronos-benchmark-3val/cronos_777-1/node{0,1,2}`, each with its own
`config.toml` / `app.toml` / `genesis.json` derived from the jsonnet config,
and a supervisor config (`tasks.ini`) to run all nodes.

## 8. Start the network

```shell
<nix-shell> $ pystarport start --data /tmp/cronos-benchmark-3val --quiet
```

This starts all 3 validators (and their EVM JSON-RPC servers) under
`supervisord`, in the foreground. Leave this running in one terminal.

Node `i`'s base port is `base_port + i * 10`; each service is at a fixed
offset from that (`p2p=+0`, `evmrpc=+1`, `evmrpc_ws=+2`, `grpc=+3`,
`api=+4`, `pprof=+5`, `grpc_tx_only=+6`, `rpc=+7`, `grpc_web=+8`). E.g. with
`base_port=26650`:

| Node | RPC (tcp) | EVM JSON-RPC | EVM WS |
| ---- | --------- | ------------ | ------ |
| node0 | 26657 | 26651 | 26652 |
| node1 | 26667 | 26661 | 26662 |
| node2 | 26677 | 26671 | 26672 |

## 9. Interact with the devnet (separate terminal)

```shell
<nix-shell> $ cd /tmp/cronos-benchmark-3val
<nix-shell> $ supervisorctl -c tasks.ini status
<nix-shell> $ cronosd status --node tcp://127.0.0.1:26657
<nix-shell> $ cronosd query bank balances <address> \
    --node tcp://127.0.0.1:26657 \
    --home cronos_777-1/node0
```

Or with node0's EVM JSON-RPC endpoint (`http://127.0.0.1:26651`) via `web3.py` /
`cast` / any Ethereum JSON-RPC client.

## 10. Stop the network

```shell
<nix-shell> $ supervisorctl -c /tmp/cronos-benchmark-3val/tasks.ini shutdown
```

Or `Ctrl-C` the `pystarport start` process from step 8.

## 11. Tear down / restart clean

```shell
rm -rf /tmp/cronos-benchmark-3val
```

Then repeat step 7 without `--no_remove` (or omit `--no_remove` entirely to
let `pystarport init` wipe and recreate the data dir automatically).

---

## Applied benchmark tuning (from the wiki's "3 Validators" `Options` block)

| Section | Key | Value |
| ------- | --- | ----- |
| `config_patch` (`config.toml`) | `db_backend` | `rocksdb` |
| | `mempool.size` | `100000` |
| | `consensus.timeout_commit` | `20ms` |
| `app_patch` (`app.toml`) | `async-check-tx` | `true` |
| | `mempool.max-txs` | `-1` |
| | `evm.block-stm-pre-estimate` | `false` |
| | `memiavl.async-commit-buffer` | `16` |
| | `json-rpc.enable-indexer` | `false` |
| `genesis_patch` (`genesis.json`) | `consensus.params.block.max_gas` | `363000000` |

`validators: 3`, `fullnodes: 0` from the wiki options map to the 3 validator
entries (and 0 fullnode entries) in the pystarport config below.

## Full config: `integration_tests/configs/benchmark-3val.jsonnet`

```jsonnet
// 3-validator devnet config matching the "3 Validators / Simple Transfer" setup
// described in https://github.com/crypto-org-chain/cronos/wiki/V1.4-Benchmark
{
  dotenv: '../../scripts/.env',
  'cronos_777-1': {
    cmd: 'cronosd',
    'start-flags': '--trace',
    // config_patch (CometBFT config.toml) from the wiki benchmark options
    config: {
      db_backend: 'rocksdb',
      mempool: {
        size: 100000,
      },
      consensus: {
        timeout_commit: '20ms',
      },
    },
    // app_patch (app.toml) from the wiki benchmark options
    'app-config': {
      chain_id: 'cronos_777-1',
      'app-db-backend': 'rocksdb',
      'minimum-gas-prices': '0basetcro',
      'index-events': ['ethereum_tx.ethereumTxHash'],
      'iavl-lazy-loading': true,
      'async-check-tx': true,
      'json-rpc': {
        address: '127.0.0.1:{EVMRPC_PORT}',
        'ws-address': '127.0.0.1:{EVMRPC_PORT_WS}',
        api: 'eth,net,web3,debug,cronos',
        'feehistory-cap': 100,
        'block-range-cap': 10000,
        'logs-cap': 10000,
        'enable-indexer': false,
      },
      mempool: {
        'max-txs': -1,
      },
      evm: {
        'block-executor': 'block-stm',
        'block-stm-workers': 32,
        'block-stm-pre-estimate': false,
      },
      memiavl: {
        enable: true,
        'zero-copy': true,
        'snapshot-interval': 5,
        'cache-size': 0,
        'async-commit-buffer': 16,
      },
      grpc: {
        'skip-check-header': true,
      },
    },
    // 3 uniform validators, all rocksdb-backed
    validators: [
      {
        coins: '1000000000000000000stake,10000000000000000000000basetcro',
        staked: '1000000000000000000stake',
        mnemonic: '${VALIDATOR1_MNEMONIC}',
        client_config: {
          'broadcast-mode': 'sync',
        },
      },
      {
        coins: '1000000000000000000stake,10000000000000000000000basetcro',
        staked: '1000000000000000000stake',
        mnemonic: '${VALIDATOR2_MNEMONIC}',
        client_config: {
          'broadcast-mode': 'sync',
        },
      },
      {
        coins: '1000000000000000000stake,10000000000000000000000basetcro',
        staked: '1000000000000000000stake',
        mnemonic: '${VALIDATOR3_MNEMONIC}',
        client_config: {
          'broadcast-mode': 'sync',
        },
      },
    ],
    accounts: [
      {
        name: 'community',
        coins: '10000000000000000000000basetcro',
        mnemonic: '${COMMUNITY_MNEMONIC}',
      },
      {
        name: 'signer1',
        coins: '20000000000000000000000basetcro',
        mnemonic: '${SIGNER1_MNEMONIC}',
      },
      {
        name: 'signer2',
        coins: '30000000000000000000000basetcro',
        mnemonic: '${SIGNER2_MNEMONIC}',
      },
    ],
    // genesis_patch from the wiki benchmark options
    genesis: {
      consensus: {
        params: {
          block: {
            max_bytes: '1048576',
            max_gas: '363000000',
          },
        },
      },
      app_state: {
        evm: {
          params: {
            evm_denom: 'basetcro',
          },
        },
        cronos: {
          params: {
            cronos_admin: '${CRONOS_ADMIN}',
            enable_auto_deployment: true,
            ibc_cro_denom: '${IBC_CRO_DENOM}',
          },
        },
        gov: {
          params: {
            expedited_voting_period: '1s',
            voting_period: '10s',
            max_deposit_period: '10s',
            min_deposit: [
              {
                denom: 'basetcro',
                amount: '1',
              },
            ],
          },
        },
        transfer: {
          params: {
            receive_enabled: true,
            send_enabled: true,
          },
        },
        feemarket: {
          params: {
            no_base_fee: false,
            base_fee: '100000000000',
          },
        },
      },
    },
  },
}
```

Mnemonics/addresses (`${VALIDATOR1_MNEMONIC}`, `${CRONOS_ADMIN}`, etc.) come
from `scripts/.env`, loaded via the `dotenv` field — no need to duplicate
them here.

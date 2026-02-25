# Testground Benchmark

A simplified benchmark framework inspired by [testground](https://github.com/testground/testground). Each node gets a unique integer ID; hostnames are derived from that for peer discovery.

## Quick Start (All-in-One)

Build the image, embed test data, and run -- all in one command:

```bash
# From the repository root directory
cd testground

# 1. Build image + embed data + run (Docker)
docker build -t cronos-testground:latest -f Dockerfile .. --build-arg EMBED_DATA=true \
  && mkdir -p /tmp/outputs \
  && jsonnet -S benchmark/compositions/docker-compose.jsonnet \
       --ext-str outputs=/tmp/outputs --ext-code nodes=3 > /tmp/docker-compose-testground.yaml \
  && docker compose -f /tmp/docker-compose-testground.yaml down 2>/dev/null; \
     docker compose -f /tmp/docker-compose-testground.yaml up --remove-orphans --force-recreate
```

Or step-by-step if you prefer more control:

```bash
# Step 1: Build image
docker build -t cronos-testground:latest -f Dockerfile ..

# Step 2: Update test data (re-gen + patch, reuses existing image)
docker run --rm -v /tmp/data:/data cronos-testground:latest \
  stateless-testcase generic-gen "$(jq '.outdir = "/data/out"' benchmark-options.json)"
echo 'FROM cronos-testground:latest
ADD ./out /data' | docker build -t cronos-testground:latest -f - /tmp/data

# Step 3: Run
mkdir -p /tmp/outputs
jsonnet -S benchmark/compositions/docker-compose.jsonnet \
  --ext-str outputs=/tmp/outputs --ext-code nodes=3 > /tmp/docker-compose-testground.yaml
docker compose -f /tmp/docker-compose-testground.yaml down 2>/dev/null
docker compose -f /tmp/docker-compose-testground.yaml up --remove-orphans --force-recreate
```

Results are collected in `/tmp/outputs`.

## Build Image

### Docker Build (works everywhere)

```bash
cd testground

# Current platform (auto-detects ARM64 on Apple Silicon)
docker build -t cronos-testground:latest -f Dockerfile ..

# Explicit ARM64 build
docker buildx build --platform linux/arm64 -t cronos-testground:latest -f Dockerfile ..

# Multi-arch push to registry
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ghcr.io/crypto-org-chain/cronos-testground:latest --push -f Dockerfile ..
```

### Nix Build

> Requires: nix with flakes. macOS also needs a [Linux remote builder](https://nix.dev/manual/nix/2.22/advanced-topics/distributed-builds.html).

```bash
# Apple Silicon (M1/M2/M3/M4)
docker load < $(nix build '.#legacyPackages.aarch64-linux.testground-image' --no-link --print-out-paths) \
  | grep "^Loaded image:" | cut -d ' ' -f 3 \
  | xargs -I{} docker tag {} cronos-testground:latest

# x86_64 Linux / Intel Mac
docker load < $(nix build '.#legacyPackages.x86_64-linux.testground-image' --no-link --print-out-paths) \
  | grep "^Loaded image:" | cut -d ' ' -f 3 \
  | xargs -I{} docker tag {} cronos-testground:latest
```

### Prebuilt Images

Available from [GitHub Container Registry](https://github.com/crypto-org-chain/cronos/pkgs/container/cronos-testground):

```bash
docker pull ghcr.io/crypto-org-chain/cronos-testground:latest
docker tag ghcr.io/crypto-org-chain/cronos-testground:latest cronos-testground:latest
```

## Configure Benchmark

Edit `testground/benchmark-options.json` before building or patching:

```json
{
  "outdir": "/data",
  "validators": 3,
  "fullnodes": 0,
  "num_accounts": 10000,
  "num_txs": 5,
  "batch_size": 100,
  "tx_type": "simple-transfer",
  "validator_generate_load": true,
  "num_idle": 20,
  "config_patch": {},
  "app_patch": {},
  "genesis_patch": {},
  "node_overrides": {}
}
```

| Field | Default | Description |
| ----- | ------- | ----------- |
| `validators` | `3` | Number of validators |
| `fullnodes` | `0` | Number of full nodes |
| `num_accounts` | `100` | Test accounts per node |
| `num_txs` | `1000` | Transactions per account |
| `tx_type` | `simple-transfer` | `simple-transfer` or `erc20-transfer` |
| `batch_size` | `1` | Txs per batch (`100` for batch tests) |
| `validator_generate_load` | `true` | Whether validators generate load |
| `num_idle` | `20` | Idle blocks before stopping |
| `config_patch` | `{}` | CometBFT config.toml overrides |
| `app_patch` | `{}` | Cronos app.toml overrides |
| `genesis_patch` | `{}` | genesis.json overrides |
| `node_overrides` | `{}` | Per-node overrides (see below) |

### Per-Node Overrides (`node_overrides`)

Apply different settings to individual nodes. Keys are `global_seq` as strings (validators `"0"`, `"1"`, ...; fullnodes continue after). Values are deep-merged on top of defaults.

Overridable fields: `config_patch`, `app_patch`, `num_accounts`, `num_txs`, `tx_type`, `batch_size`, `validator_generate_load`, `num_idle`.

```json
{
  "node_overrides": {
    "0": { "app_patch": { "evm": { "block-executor": "sequential" } } },
    "1": { "num_accounts": 20000, "num_txs": 10 }
  }
}
```

When overrides are active, the benchmark prints a per-node config diff at startup and after results.

### Config Defaults

**CometBFT** (`config_patch`): `db_backend: rocksdb`, `mempool.recheck: false`, `mempool.size: 50000`, `consensus.timeout_commit: 1s`, `tx_index.indexer: null`

**Cronos App** (`app_patch`): `memiavl.enable: true`, `evm.block-executor: block-stm`, `evm.block-stm-workers: 0`, `evm.block-stm-pre-estimate: true`, `mempool.max-txs: 50000`

## Embed / Update Test Data

### Option A: Embed at build time

```bash
docker build -t cronos-testground:latest -f Dockerfile .. --build-arg EMBED_DATA=true
```

Reads `benchmark-options.json` and bakes data into the image.

### Option B: Patch an existing image (faster iteration)

```bash
# Generate data
docker run --rm -v /tmp/data:/data cronos-testground:latest \
  stateless-testcase generic-gen "$(jq '.outdir = "/data/out"' testground/benchmark-options.json)"

# Patch image
echo 'FROM cronos-testground:latest
ADD ./out /data' | docker build -t cronos-testground:latest -f - /tmp/data
```

### Option C: Nix patchimage

```bash
nix run .#stateless-testcase -- patchimage cronos-testground:latest /tmp/data/out
```

## Run Benchmark

### Docker Compose (local)

```bash
mkdir -p /tmp/outputs
jsonnet -S testground/benchmark/compositions/docker-compose.jsonnet \
  --ext-str outputs=/tmp/outputs --ext-code nodes=3 > /tmp/docker-compose-testground.yaml
docker compose -f /tmp/docker-compose-testground.yaml down 2>/dev/null
docker compose -f /tmp/docker-compose-testground.yaml up --remove-orphans --force-recreate
```

Node data and `block_stats.log` are collected in `/tmp/outputs`.

### Kubernetes

See [KUBERNETES.md](KUBERNETES.md) for Indexed Job and StatefulSet deployment guides.

## Development

### Run tests

```bash
cd testground/benchmark
nix develop -c pytest -vv -s
```

### Lint

```bash
# Check
nix-shell -I nixpkgs=./nix -p test-env --run "make lint-py"
nix-shell -I nixpkgs=./nix -p nixfmt-rfc-style --run "make lint-nix"

# Auto-fix all (Go + Python + Nix)
make lint-fix-all
```

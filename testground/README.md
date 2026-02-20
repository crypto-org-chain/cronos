# Testground

The implementation is inspired by [testground](https://github.com/testground/testground), but we did a lot of simplifications to make it easier to deploy:

- No centralized sync service, each node are assigned an unique continuous integer identifier, and node's hostname can be derived from that, that's how nodes discover each other and build the network.
- Don't support networking configuration, but we might implement it in the future.

## Build Image

You can test with the prebuilt images in [GitHub registry](https://github.com/crypto-org-chain/cronos/pkgs/container/cronos-testground), or build the image locally using one of the methods below.

### Method 1: Docker Build (Recommended for Apple Silicon / M1/M2/M3)

Build natively for your platform using Docker:

```bash
# From the repository root directory
cd testground

# Build for current platform (auto-detects ARM64 on M1 Macs)
docker build -t cronos-testground:latest -f Dockerfile ..

# Or explicitly build for ARM64
docker buildx build --platform linux/arm64 -t cronos-testground:arm64 -f Dockerfile ..

# Build multi-arch image (requires docker buildx)
# Note: multi-platform builds do NOT load images into the local Docker daemon,
# so the tag won't be immediately runnable. Use one of these alternatives:
#   --load   : load a single-platform image locally (cannot combine with multi-platform)
#   --push   : push multi-arch manifest to a registry for later pull/use
docker buildx build --platform linux/amd64,linux/arm64 -t cronos-testground:latest --push -f Dockerfile ..
```

### Method 2: Nix Build

> Prerequisites: nix, for macOS also need [linux remote builder](https://nix.dev/manual/nix/2.22/advanced-topics/distributed-builds.html)

```bash
# From the repository root directory
$ nix build '.#testground-image'
# for apple silicon mac: nix build '.#legacyPackages.aarch64-linux.testground-image'
# for x86 mac: nix build '.#legacyPackages.x86_64-linux.testground-image'
$ docker load < ./result
Loaded image: cronos-testground:<imageID>
$ docker tag cronos-testground:<imageID> ghcr.io/crypto-org-chain/cronos-testground:latest
```

Or one liner like this:

```bash
docker load < $(nix build '.#legacyPackages.aarch64-linux.testground-image' --no-link --print-out-paths) \
  | grep "^Loaded image:" \
  | cut -d ' ' -f 3 \
  | xargs -I{} docker tag {} ghcr.io/crypto-org-chain/cronos-testground:latest
```

## Generate data files locally

You need to have the `cronosd` in `PATH`.

```bash
nix run .#stateless-testcase -- gen /tmp/data/out \
  --validator-generate-load \
  --validators 3 \
  --fullnodes 0 \
  --num-accounts 800 \
  --num-txs 20 \
  --num-idle 20 \
  --app-patch '{"mempool": {"max-txs": -1}}' \
  --config-patch '{"mempool": {"size": 10000}}' \
  --tx-type erc20-transfer \
  --genesis-patch '{"consensus": {"params": {"block": {"max_gas": "263000000"}}}}'
```

* `validators`/`fullnodes` is the number of validators/full nodes.
* `num_accounts` is the number of test accounts for each full node.
* `num_txs` is the number of test transactions to be sent for each test account.
* `config`/`app` is the config patch for config/app.toml.
* `genesis` is the patch for genesis.json.

## Embed Test Data Into Image

There are multiple ways to embed test data into the image. See the [V1.4 Benchmark wiki](https://github.com/crypto-org-chain/cronos/wiki/V1.4-Benchmark#summary-table-cronos-140) for benchmark configurations.

### Test Type Options

| Test Type | `tx_type` | `batch_size` |
|-----------|-----------|--------------|
| Simple Transfer | `simple-transfer` | `1` |
| ERC20 Transfer | `erc20-transfer` | `1` |
| Batch Simple Transfer | `simple-transfer` | `100` |
| Batch ERC20 Transfer | `erc20-transfer` | `100` |

### Method 1: Embed at Build Time (Options File)

The simplest way - edit the options file and build:

**Step 1: Edit `testground/benchmark-options.json`**

```json
{
  "outdir": "/data",
  "validators": 3,
  "fullnodes": 0,
  "num_accounts": 100,
  "num_txs": 1000,
  "num_idle": 20,
  "tx_type": "simple-transfer",
  "batch_size": 1,
  "validator_generate_load": true,
  "config_patch": {
    "mempool": {"size": 50000},
    "consensus": {"timeout_commit": "1s"}
  },
  "app_patch": {
    "mempool": {"max-txs": 50000}
  },
  "genesis_patch": {}
}
```

**Step 2: Build with embedded data**

```bash
cd testground
docker build -t cronos-testground:latest -f Dockerfile .. --build-arg EMBED_DATA=true
```

#### Options File Reference

| Field | Default | Description |
|-------|---------|-------------|
| `outdir` | `/data` | Output directory in container (don't change) |
| `validators` | `3` | Number of validators |
| `fullnodes` | `0` | Number of full nodes |
| `num_accounts` | `100` | Test accounts per node |
| `num_txs` | `1000` | Transactions per account |
| `num_idle` | `20` | Idle blocks after test |
| `tx_type` | `simple-transfer` | `simple-transfer` or `erc20-transfer` |
| `batch_size` | `1` | Transactions per batch (use `100` for batch tests) |
| `validator_generate_load` | `true` | Whether validators generate load |
| `config_patch` | `{}` | CometBFT config.toml overrides |
| `app_patch` | `{}` | Cronos app.toml overrides |
| `genesis_patch` | `{}` | genesis.json overrides |
| `node_overrides` | `{}` | Per-node overrides keyed by `global_seq` (see below) |

#### Per-Node Overrides (`node_overrides`)

You can apply different settings to individual validators or fullnodes by adding a `node_overrides` map. Keys are the `global_seq` index as a string (validators are `"0"`, `"1"`, ..., fullnodes continue after). Values are dicts that get deep-merged on top of the defaults.

Overridable fields per node:
- **Config**: `config_patch`, `app_patch`
- **Load**: `num_accounts`, `num_txs`, `tx_type`, `batch_size`
- **Behavior**: `validator_generate_load`, `num_idle`

Example -- validator 0 runs sequential execution while others use block-stm, and validator 1 generates more load:

```json
{
  "outdir": "/data",
  "validators": 3,
  "num_accounts": 10000,
  "num_txs": 5,
  "batch_size": 100,
  "app_patch": { "evm": { "block-stm-workers": 8 } },
  "node_overrides": {
    "0": {
      "app_patch": { "evm": { "block-executor": "sequential" } }
    },
    "1": {
      "num_accounts": 20000,
      "num_txs": 10
    }
  }
}
```

Nodes without an entry in `node_overrides` use the top-level defaults unchanged.

#### CometBFT Config Options (`config_patch`)

The benchmark tool applies these defaults automatically:
- `db_backend`: `rocksdb`
- `mempool.recheck`: `false`
- `mempool.size`: `50000`
- `consensus.timeout_commit`: `1s`
- `tx_index.indexer`: `null`

Example overrides:

```json
"config_patch": {
  "mempool": {"size": 100000},
  "consensus": {"timeout_commit": "500ms"}
}
```

#### Cronos App Options (`app_patch`)

The benchmark tool applies these defaults automatically:
- `memiavl.enable`: `true`
- `memiavl.cache-size`: `0`
- `evm.block-executor`: `block-stm`
- `evm.block-stm-workers`: `0`
- `evm.block-stm-pre-estimate`: `true`
- `mempool.max-txs`: `50000`
- `json-rpc.enable-indexer`: `true`

Example overrides:

```json
"app_patch": {
  "mempool": {"max-txs": -1},
  "evm": {"block-executor": "sequential"}
}
```

#### Genesis Options (`genesis_patch`)

Example - increase block gas limit:

```json
"genesis_patch": {
  "consensus": {"params": {"block": {"max_gas": "263000000"}}}
}
```

#### Example Configurations

**ERC20 Transfer with 5 validators:**

```json
{
  "outdir": "/data",
  "validators": 5,
  "fullnodes": 0,
  "num_accounts": 100,
  "num_txs": 1000,
  "tx_type": "erc20-transfer",
  "batch_size": 1,
  "validator_generate_load": true
}
```

**Batch Simple Transfer (high throughput):**

```json
{
  "outdir": "/data",
  "validators": 3,
  "fullnodes": 0,
  "num_accounts": 100,
  "num_txs": 1000,
  "tx_type": "simple-transfer",
  "batch_size": 100,
  "validator_generate_load": true,
  "config_patch": {
    "mempool": {"size": 100000}
  },
  "app_patch": {
    "mempool": {"max-txs": -1}
  }
}
```

### Method 2: Generate Data Then Patch Image (Faster)

This method is faster because it reuses an existing base image and only regenerates test data.

**Step 1: Edit `testground/benchmark-options.json`** (same file as Method 1)

Make sure `outdir` is set to `/data/out` for this method:

```json
{
  "outdir": "/data/out",
  "validators": 3,
  ...
}
```

**Step 2: Generate test data using the options file**

```bash
# Clean up any existing data
rm -rf /tmp/data/out

# Generate data using options file
docker run --rm \
  -v /tmp/data:/data \
  cronos-testground:latest \
  stateless-testcase generic-gen "$(cat testground/benchmark-options.json)"
```

**Step 3: Patch the image with generated data**

```bash
cd /tmp/data
cat > Dockerfile.patched << 'EOF'
FROM cronos-testground:latest
ADD ./out /data
EOF
docker build -t cronos-testground:patched -f Dockerfile.patched .
docker tag cronos-testground:patched cronos-testground:latest
```

**All-in-one script:**

```bash
# From repo root directory
rm -rf /tmp/data/out

# Generate data from options file
docker run --rm -v /tmp/data:/data cronos-testground:latest \
  stateless-testcase generic-gen "$(jq '.outdir = "/data/out"' testground/benchmark-options.json)"

# Patch image
cd /tmp/data
echo 'FROM cronos-testground:latest
ADD ./out /data' > Dockerfile.patched
docker build -t cronos-testground:latest -f Dockerfile.patched .
```

### Method 3: Using Nix (patchimage command)

```bash
$ nix run github:crypto-org-chain/cronos#stateless-testcase patchimage cronos-testground:latest /tmp/data/out
```

## Run With Docker Compose

```bash
$ mkdir /tmp/outputs
$ jsonnet -S testground/benchmark/compositions/docker-compose.jsonnet \
  --ext-str outputs=/tmp/colima \
  --ext-code nodes=3 \
  | docker-compose -f /dev/stdin up --remove-orphans --force-recreate
```

It'll collect the node data files to the `/tmp/outputs` directory.

## Run In Cluster

Please use [cronos-testground](https://github.com/crypto-org-chain/cronos-testground) to run the benchmark in k8s cluster.

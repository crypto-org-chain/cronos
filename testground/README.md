# Testground

The implementation is inspired by [testground](https://github.com/testground/testground), but we did a lot of simplifications to make it easier to deploy:

- No centralized sync service, each node are assigned an unique continuous integer identifier, and node's hostname can be derived from that, that's how nodes discover each other and build the network.
- Don't support networking configuration, but we might implement it in the future.

## Build Image

>  Prerequisites: nix, for macOS also need [linux remote builder](https://nix.dev/manual/nix/2.22/advanced-topics/distributed-builds.html)

You can test with the prebuilt images in [github registry](https://github.com/crypto-org-chain/cronos/pkgs/container/cronos-testground), or build the image locally:

```bash
$ nix build .#testground-image
# for apple silicon mac: nix build .#legacyPackages.aarch64-linux.testground-image
# for x86 mac: nix build .#legacyPackages.x86_64-linux.testground-image
$ docker load < ./result
Loaded image: cronos-testground:<imageID>
$ docker tag cronos-testground:<imageID> ghcr.io/crypto-org-chain/cronos-testground:latest
```

Or one liner like this:

```bash
docker load < $(nix build .#legacyPackages.aarch64-linux.testground-image --no-link --print-out-paths) \
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
  --num-txs 400 \
  --app-patch '{"mempool.max-txs": -1}' \
  --config-patch '{"mempool.size": 100000}' \
  --tx-type erc20-transfer \
  --genesis-patch '{"consensus": {"params": {"block": {"max_gas": "263000000"}}}}'
```

* `validators`/`fullnodes` is the number of validators/full nodes.
* `num_accounts` is the number of test accounts for each full node.
* `num_txs` is the number of test transactions to be sent for each test account.
* `config`/`app` is the config patch for config/app.toml.
* `genesis` is the patch for genesis.json.

## Embed the data directory

Embed the data directory into the image, it produce a new image:

```bash
$ nix run github:crypto-org-chain/cronos#stateless-testcase patchimage cronos-testground:latest /tmp/data/out
```

## Run With Docker Compose

```bash
$ mkdir /tmp/outputs
$ jsonnet -S testground/benchmark/compositions/docker-compose.jsonnet \
  --ext-str outputs=/tmp/outputs \
  --ext-code nodes=3 \
  | docker-compose -f /dev/stdin up --remove-orphans --force-recreate
```

It'll collect the node data files to the `/tmp/outputs` directory.

## Run In Cluster

Please use [cronos-testground](https://github.com/crypto-org-chain/cronos-testground) to run the benchmark in k8s cluster.

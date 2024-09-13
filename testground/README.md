# Testground

[Testground documentation](https://docs.testground.ai/)

## Build Image

>  Prerequisites: nix, for macOS also need [linux remote builder](https://nix.dev/manual/nix/2.22/advanced-topics/distributed-builds.html)

You can test with the prebuilt images in [github registry](https://github.com/crypto-org-chain/cronos/pkgs/container/cronos-testground), or build the image locally:

```bash
$ nix build .#testground-image
# for mac: nix build .#legacyPackages.aarch64-linux.testground-image
$ docker load < ./result
Loaded image: cronos-testground:<imageID>
$ docker tag cronos-testground:<imageID> ghcr.io/crypto-org-chain/cronos-testground:latest
```

## Run Test

### Install Testground

```bash
$ git clone https://github.com/testground/testground.git
$ cd testground
# compile Testground and all related dependencies
$ make install
```

It'll install the `testground` binary in your `$GOPATH/bin` directory, and build several docker images.

### Run Testground Daemon

```bash
$ TESTGROUND_HOME=$PWD/data testground daemon
```

Keep the daemon process running during the test.

### Run Test Plan

Import the test plan before the first run:

```bash
$ TESTGROUND_HOME=$PWD/data testground plan import --from /path/to/cronos/testground/benchmark
```

Run the benchmark test plan in local docker environment:

```bash
$ TESTGROUND_HOME=$PWD/data testground run composition -f /path/to/cronos/testground/benchmark/compositions/local.toml --wait
```

### macOS

If you use `colima` as docker runtime on macOS, create the symbolic link `/var/run/docker.sock`:

```bash
$ sudo ln -s $HOME/.colima/docker.sock /var/run/docker.sock
```

And mount the related directories into the virtual machine:

```toml
mounts:
  - location: /var/folders
    writable: false
  - location: <TESTGROUND_HOME>
    writable: true
```



# Stateless Mode

To simplify cluster setup, we are introducing a stateless mode.

## Generate data files locally

You need to have the `cronosd` in `PATH`.

```bash
$ nix run github:crypto-org-chain/cronos#stateless-testcase -- gen /tmp/data/out \
  --hostname_template "testplan-{index}" \
  --options '{
    "validators": 3,
    "fullnodes": 7,
    "num_accounts": 10,
    "num_txs": 1000,
    "config": {
      "mempool.size": 10000
    },
    "app": {
      "evm.block-stm-pre-estimate": true
    },
    "genesis": {
      "consensus.params.block.max_gas": 163000000
    }
  }'
```

* `hostname_template` is the hostname of each node that can communicate.
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

## Run In Local Docker

```bash
$ mkdir /tmp/outputs
$ jsonnet -S testground/benchmark/compositions/docker-compose.jsonnet | docker-compose -f /dev/stdin up
```

It'll collect the node data files to the `/tmp/outputs` directory.

## Run In Cluster

TODO

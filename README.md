<!--
parent:
  order: false
-->


<div align="center">
  <h1> <img src="./assets/cronos.svg" alt="Cronos Logo" width="300px" /> </h1>
</div>
<br />

<p align="center">
  <a href="https://github.com/crypto-org-chain/cronos/actions/workflows/build.yml"><img label="Build Status" src="https://github.com/crypto-org-chain/cronos/actions/workflows/build.yml/badge.svg" /></a>
  <a href="https://codecov.io/gh/crypto-org-chain/cronos"><img label="Code Coverage" src="https://codecov.io/gh/crypto-org-chain/cronos/branch/main/graph/badge.svg" /></a>
  <a href="https://discord.gg/pahqHz26q4"><img label="Discord" src="https://img.shields.io/discord/783264383978569728.svg?color=7289da&label=Cronos&logo=discord&style=flat-square" /></a>
</p>

## Table of Contents

- [Table of Contents](#table-of-contents)
- [1. Description](#1-description)
- [2. Contributing](#2-contributing)
- [3. License](#3-license)
- [4. Documentation](#4-documentation)
- [5. Build full node](#5-build-full-node)
- [6. Start a local Development Network and Node](#6-start-a-local-development-network-and-node)
- [7. Send Your First Transaction](#7-send-your-first-transaction)
- [8. Testing](#8-testing)
- [9. Pystarport Quick Start](#9-pystarport-quick-start)
  - [install latest python (for linux)](#install-latest-python-for-linux)
  - [set path (for linux or for mac)](#set-path-for-linux-or-for-mac)
  - [install pystarport](#install-pystarport)
  - [quick start](#quick-start)
  - [get status](#get-status)
  - [stop all](#stop-all)
- [10. Useful links](#10-useful-links)

<a id="description" />

## 1. Description

**Cronos** is the Crypto.org EVM chain that aims to massively scale the DeFi ecosystem.

<a id="contributing" />

## 2. Contributing

Please abide by the [Code of Conduct](CODE_OF_CONDUCT.md) in all interactions,
and the [contributing guidelines](CONTRIBUTING.md) when submitting code.

<a id="license" />

## 3. License

[Apache 2.0](./LICENSE)

<a id="documentation" />

## 4. Documentation

[Technical documentation](http://cronos.org/docs).

<a id="build" />

## 5. Build full node

```bash
# COSMOS_BUILD_OPTIONS=rocksdb make install
make build
```

<a id="start-local-full-node" />

## 6. Start a local Development Network and Node

Please follow this [documentation](https://cronos.org/docs/getting-started/local-devnet.html#devnet-running-latest-development-node) to run a local devnet.

<a id="send-first-transaction" />

## 7. Send Your First Transaction

After setting the local devnet, you may interact with the your local blockchain by following this [documentation](https://cronos.org/docs/getting-started/local-devnet.html#interact-with-the-chain).

<a id="testing" />

## 8. Testing

There are different tests that can be executed in the following ways:

- unit tests: `make test`
- [integration tests](./docs/integration-test.md)

### CI Testing
we use `Nix` as our CI testing environment and use `gomod2nix` to convert go modules into nix packages.
Therefore, to install `gomod2nix` is required:
```
go install github.com/nix-community/gomod2nix@latest
```
And then, you can run:
```
gomod2nix generate
```
to update `gomod2nix.toml` if any go package has changed.

<a id="pystarport" />

## 9. Pystarport Quick Start

you can install pystarport to manage nodes for development.

### install latest python (for linux)

python version should be 3.8 or above.
you can install python like this.

```
git clone git@github.com:python/cpython.git
cd cpython
git checkout tags/v3.9.5
./configure
make
sudo make install
```

### set path (for linux or for mac)
in some cases, if there are multiple python versions, pystarport cannot be found.
then adjust python path.
also `$HOME/.local/bin` should be included to the PATH.

```
export PATH=/usr/local/bin:$HOME/.local/bin:$PATH
```

### install pystarport

```
python3 -m pip install pystarport
```

### quick start

run two nodes devnet

```
pystarport serve --config ./scripts/cronos-devnet.yaml
```

### get status

```
pystarport supervisorctl status
```

### stop all

```
pystarport supervisorctl stop all
```

---

<a id="useful-links" />

## 10. Useful links

- [Project Website](http://cronos.org/)
- [Technical Documentation](http://cronos.org/docs)
- Community chatrooms (non-technical): [Discord](https://discord.gg/nsp9JTC) [Telegram](https://t.me/CryptoComOfficial)
- Developer community channel (technical): [![Support Server](https://img.shields.io/discord/783264383978569728.svg?color=7289da&label=Cronos&logo=discord&style=flat-square)](https://discord.gg/pahqHz26q4)
- [Ethermint](https://github.com/evmos/ethermint) by Tharsis
- [Cosmos SDK documentation](https://docs.cosmos.network)
- [Cosmos Discord](https://discord.gg/W8trcGV)
- [Pystarport](https://github.com/crypto-com/pystarport/blob/main/README.md)
- Test

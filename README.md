<!--
parent:
  order: false
-->

<div align="center">
  <h1> <img src="./assets/cronos.svg" alt="Cronos Logo" width="300px" /> </h1>
</div>

Cronos is the Crypto.org EVM chain that aims to massively scale the DeFi ecosystem.

## About this repository

This is the repository for releasing latest Cronos Testnet versions. For issues reporting and tracking please refer to [Ethermint repository](https://github.com/tharsis/ethermint) from Tharsis.

## Report Issues

Issues feature is disabled in this repository. Please report issue on [Ethermint repository](https://github.com/tharsis/ethermint/issues).

## Documentation

Technical documentation can be found in this [Github repository](https://github.com/crypto-org-chain/cronos-docs) (you can read it in [this hosted version](https://cronos.crypto.org/docs)). 
## License

[Apache 2.0](./LICENSE)

## Useful links

- [Project Website](http://cronos.crypto.org/)
- [Technical Documentation](http://cronos.crypto.org/docs)
- Community chatrooms (non-technical): [Discord](https://discord.gg/nsp9JTC) [Telegram](https://t.me/CryptoComOfficial)
- Developer community channel (technical): [![Support Server](https://img.shields.io/discord/783264383978569728.svg?color=7289da&label=Crypto.org Chain =discord =flat-square)](https://discord.gg/pahqHz26q4)
- [Ethermint](https://github.com/tharsis/ethermint) by Tharsis
- [Cosmos SDK documentation](https://docs.cosmos.network)
- [Cosmos Discord](https://discord.gg/W8trcGV)

## Ethermint

This project is powered by [Ethermint](https://github.com/tharsis/ethermint). You can find the original Ethermint README below:

<div align="center">
  <h1> Ethermint </h1>
</div>

<div align="center">
  <a href="https://github.com/tharsis/ethermint/releases/latest">
    <img alt="Version" src="https://img.shields.io/github/tag/tharsis/ethermint.svg" />
  </a>
  <a href="https://github.com/tharsis/ethermint/blob/main/LICENSE">
    <img alt="License: Apache-2.0" src="https://img.shields.io/github/license/tharsis/ethermint.svg" />
  </a>
  <a href="https://pkg.go.dev/github.com/tharsis/ethermint?tab=doc">
    <img alt="GoDoc" src="https://godoc.org/github.com/tharsis/ethermint?status.svg" />
  </a>
  <a href="https://goreportcard.com/report/github.com/tharsis/ethermint">
    <img alt="Go report card" src="https://goreportcard.com/badge/github.com/tharsis/ethermint"/>
  </a>
</div>
<div align="center">
  <a href="https://discord.gg/AzefAFd">
    <img alt="Discord" src="https://img.shields.io/discord/669268347736686612.svg" />
  </a>
  <a href="https://github.com/tharsis/ethermint/actions?query=workflow%3ABuild">
    <img alt="Build Status" src="https://github.com/tharsis/ethermint/workflows/Build/badge.svg" />
  </a>
  <a href="https://github.com/tharsis/ethermint/actions?query=workflow%3ALint">
    <img alt="Lint Status" src="https://github.com/tharsis/ethermint/workflows/Lint/badge.svg" />
  </a>
  <a href="https://codecov.io/gh/tharsis/ethermint">
    <img alt="Code Coverage" src="https://codecov.io/gh/tharsis/ethermint/branch/main/graph/badge.svg" />
  </a>
</div>

Ethermint is a scalable, high-throughput Proof-of-Stake blockchain that is fully compatible and
interoperable with Ethereum. It's build using the the [Cosmos SDK](https://github.com/cosmos/cosmos-sdk/) which runs on top of [Tendermint Core](https://github.com/tendermint/tendermint) consensus engine.

> **WARNING:** Ethermint is under VERY ACTIVE DEVELOPMENT and should be treated as pre-alpha software. This means it is not meant to be run in production, its APIs are subject to change without warning and should not be relied upon, and it should not be used to hold any value. We will remove this warning when we have a release that is stable, secure, and properly tested.

**Note**: Requires [Go 1.15+](https://golang.org/dl/)

## Quick Start

To learn how the Ethermint works from a high-level perspective, go to the [Introduction](./docs/intro/overview.md) section from the documentation.

For more, please refer to the [Ethermint Docs](./docs/), which are also hosted on [docs.ethermint.zone](https://docs.ethermint.zone/).

## Tests

Unit tests are invoked via:

```bash
make test
```

To run JSON-RPC tests, execute:

```bash
make test-rpc
```

There is also an included Ethereum mainnet exported blockchain file in `importer/blockchain`
that includes blocks up to height `97638`. To execute and test a full import of
these blocks using the EVM module, execute:

```bash
make test-import
```

You may also provide a custom blockchain export file to test importing more blocks
via the `--blockchain` flag. See `TestImportBlocks` for further documentation.

### Community

The following chat channels and forums are a great spot to ask questions about Ethermint:

- [Cosmos Discord](https://discord.gg/W8trcGV)
- [Cosmos Forum](https://forum.cosmos.network)

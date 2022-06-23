

# Pioneer11 Relayer Deployment Guide

This guide is intended to assist the community validators with setting up Gravity Relayer between `Cronos Pioneer11` and `Ethereum Goerli` testnet.


## Prerequisites

### Ethereum node

You need to have access to the EVM RPC endpoint of an Ethereum node or host your own node with [go-ethereum](https://github.com/ethereum/go-ethereum/) or [openethereum](https://github.com/openethereum/openethereum).

You can use a nodes as a service provider as discussed [here](https://ethereum.org/en/developers/docs/nodes-and-clients/nodes-as-a-service/).


### Binaries

- `gorc`, the gravity bridge orchestrator cli, build instructions can be found [here](gorc-build.md). Alternatively, you can download Linux x86_64 binary from [here](https://github.com/crypto-org-chain/gravity-bridge/releases/tag/v2.0.0-cronos-alpha0)

- Above binaries setup in `PATH`.

## Generate Relayer Keys

You need to prepare one Ethereum account for the relayer. You should transfer some funds to the account, so the relayer can cover the gas fees of message relaying later on.

Please follow the [gorc-keystores](gorc-keystores.md) guide for this step. Note that you will not need a Cronos account to set up the relayer.

## Transfer funds to Relayer accounts

You should transfer funds to the Ethereum account generated earlier. Gravity Bridge is deployed between the `Cronos Pioneer11` and the `Ethereum Goerli` testnet.


## Trial Run Relayer

In order to run the relayer, you will need to set RELYAER_API_URL environment variable to point to Cronos public relayer API:

```bash
	export RELYAER_API_URL=https://cronos.org/pioneer11/relayer
```

To read more about the relayer modes, you can check out [gravity-bridge-relayer-modes.md](gravity-bridge-relayer-modes.md).

To run the relayer:

```bash
gorc -c gorc.toml relayer start \
		--ethereum-key "relayerKeyName" \
		--mode Api
```

The relayer is running now.

## Run Relayer as a Service (Linux only)

To set up the Relayer as a service, you can run:

```bash
	bash <(curl -s -L https://raw.githubusercontent.com/crypto-org-chain/cronos/main/docs/gravity-bridge/systemd/setup-gorc-service.sh) -t relayer
```

You will be prompted for your ethereum key name set up earlier. After the service is created, you can run:

```bash
	sudo systemctl start gorc
```

To view the logs:

```bash
	journalctl -u gorc -f
```


# Gravity Bridge Testnet Relayer Deployment Guide

This guide is intended to assist the community validators with setting up Gravity Relayer between `Cronos Gravity Testnet2` and `Ethereum` Kovan testnet.


## Prerequisites

### Ethereum node

You need to have access to the EVM RPC endpoint of an Ethereum node. Or host your own node with [go-ethereum](https://github.com/ethereum/go-ethereum/) or [openethereum](https://github.com/openethereum/openethereum).

### Binaries

- `gorc`, the gravity bridge relayer cli, build instructions can be found [here](gorc-build.md).

- Above binaries setup in `PATH`.

## Generate Relayer Keys

You need to prepare one Ethereum account for the relayer. You should transfer some funds to the account, so the relayer can cover the gas fees of message relaying later on.

Please follow the [gorc-keystores](gorc-keystores.md) guide for this step. Note that you will not need a Cronos account to set up the relayer.

## Transfer funds to Relayer accounts

You should transfer funds to the Ethereum account generated earlier. Gravity Bridge is deployed between the `Cronos Gravity Testnet2` and the `Ethereum` Kovan testnet.


## Trial Run Relayer

```bash
gorc -c gorc.toml relayer start \
		--ethereum-key "relayerKeyName"
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
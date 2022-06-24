

# Pioneer11 Orchestrator Deployment Guide

This guide is intended to assist the community validators with setting up Gravity Orchestrator and Relayer (jointly, in one process) between `Cronos Pioneer11` and `Ethereum Goerli` testnet. The default orchestrator start command includes running a relayer. However, they are two different processes. You can read more about Gravity Bridge [here](https://blog.althea.net/how-gravity-works/).

## Prerequisites

### Validator

You should have a validator running in `Cronos Pioneer11` network.

### Ethereum node

You need to have access to the EVM RPC endpoint of an Ethereum node or host your own node with [go-ethereum](https://github.com/ethereum/go-ethereum/) or [openethereum](https://github.com/openethereum/openethereum).

You can use a nodes as a service provider as discussed [here](https://ethereum.org/en/developers/docs/nodes-and-clients/nodes-as-a-service/).


### Binaries

-  `cronosd` version: `0.8.0` , the cronos node binary found at https://github.com/crypto-org-chain/cronos/releases/tag/v0.8.0-gravaity-alpha0. You will need to use one of the testnet binaries according to your OS/ Arch.

- `gorc`, the gravity bridge orchestrator cli, build instructions can be found [here](gorc-build.md). Alternatively, you can download Linux x86_64 binary from [here](https://github.com/crypto-org-chain/gravity-bridge/releases/tag/v2.0.0-cronos-alpha0)

- Above binaries setup in `PATH`.

## Generate Orchestrator Keys

You need to prepare two accounts for the orchestrator, one for ethereum and one for cronos. You should transfer some funds to these accounts, so the orchestrator can cover the gas fees of message relaying later on.

Please follow the [gorc-keystores](gorc-keystores.md) guide for this step.

## Transfer funds to orchestrator accounts

You should transfer funds to the Ethereum and Cronos accounts generated earlier. Gravity Bridge is deployed between the `Cronos Pioneer11` and the `Ethereum Goerli` testnet.


## Sign Validator Address


### Prerequisites:

1. Get **validator address**:

	If you have your validator key set up locally, you can run:

	```bash
	cronosd keys show $val_key_name --bech val --output json | jq .address
	```

	Sample out:
	`"tcrcvaloper18d5ne2f2xdge9s4yw0wr6h8gpvg5p7lec4eefk"`

2. Get **validator account address**:

	If you have your validator key set up locally, you can run:

	```bash
	cronosd keys show $val_key_name --output json | jq .address
	```

	Sample out:
		`"tcrc18d5ne2f2xdge9s4yw0wr6h8gpvg5p7lep8zx6p"`

3. Get validator current `nonce`:

	```bash
	cronosd query auth account $val_account_add_from_2 --output json | jq .base_account.sequence
	```

  Sample out:
	"15"

4. Have an Ethereum private key. To create a new one, refer to [Creating an Ethereum account](#creating-an-ethereum-account)

### Generating the signature:

To register the orchestrator with the validator, you need to sign a protobuf encoded message using the orchestrator's Ethereum key, and send it to a Cronos validator to register it.

To get the signature, we will use `gorc` as follows:

```bash
gorc sign-delegate-keys orch_eth $val_address $nonce
```

Sample out:

`0x530742a07ee3bed639b91fe6d9a7ed9bfb4352183eafd332fba431dcb4721ebb1a5d058018a71dd51051aceaa69e1bbc8763336da26de5bbae30c5b624d7ec781b`

Note that:
1. orch_eth was configured in [here](#creating-an-ethereum-account).
2. `$val_address` and `$nonce` were obtained from the Prerequisites section.


## Register Orchestrator With Cronos Validator


At last, send the orchestrator's Ethereum address, Cronos address, and the signature we just signed above to a Cronos validator, the validator should send a `set-delegate-keys` transaction to cronos network to register the binding:


```bash

cronosd tx gravity set-delegate-keys $val_address  $orchestrator_cronos_address  $orchestrator_eth_address  $signature --from $val_account_address

```

You might also need to set `--chain-id pioneereleventestnet_340-1` and `--fees`.


## Trial Run Orchestrator

In order to run the orchestrator, you will need to set RELAYER_API_URL environment variable to point to Cronos public relayer API:

```bash
	export RELAYER_API_URL=https://cronos.org/pioneer11/relayer
```

To read more about the relayer modes, you can check out [gravity-bridge-relayer-modes.md](gravity-bridge-relayer-modes.md).

To run the orchestrator:

```bash
gorc -c gorc.toml orchestrator start \
		--cosmos-key="orch_cro" \
		--ethereum-key="orch_eth" \
		--mode Api
```

The orchestrator is running now.

**Important**: By default, starting the orchestrator as shown above will also start the relayer. If you want to run the orchestrator without the relayer, you can pass `--orchestrator-only`. Alternatively, if you want to run the relayer without the orchestrator, please follow [relayer-only-deployment-guide](pioneer11-relayer-only-deployment-guide.md).

## Run gorc as a Service (Linux only)

To set up the Orchestrator (and relayer) as a service, you can run:

```bash
	bash <(curl -s -L https://raw.githubusercontent.com/crypto-org-chain/cronos/main/docs/gravity-bridge/systemd/setup-gorc-service.sh) -t orchestrator
```

You will be prompted for your key names set up earlier. After the service is created, you can run:

```bash
	sudo systemctl start gorc
```

To view the logs:

```bash
	journalctl -u gorc -f
```
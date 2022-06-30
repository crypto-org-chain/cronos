

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

In order to run the relayer, you will need to set RELAYER_API_URL environment variable to point to Cronos public relayer API:

```bash
export RELAYER_API_URL=https://cronos.org/pioneer11/relayer/relayer
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

## Reference:

### Contracts

```
CronosGravity: 0x56C7354887f8d00b5f9945Edb1430D7168F348F5 (on Goerli) - To be used in `gorc.toml`
Eth Gravity Wrapper: 0x2C962ecb54D53B54144b7f297158FA23e3abb871 (on Goerli)
CroBridge: 0x38F05eb0c209c4c9Fe2D6E237f03ec503f65F088 (on Pioneer11)
```

Here are the deployed token mappings:

| ERC20 token | Goerli  | Pioneer11  |
| ------- | --- | --- |
| USDC | 0xD87Ba7A50B2E7E660f678A895E4B72E7CB4CCd9C | 0x8a8DfedBF6650737DFf63c2f455ecC54AcEcF197 |
| WETH | 0xB4FBF271143F4FBf7B91A5ded31805e42b2208d6 | 0x17774909725bA203B8501C1DEb22F2495584197e |
| USDT | 0xe802376580c10fE23F027e1E19Ed9D54d4C9311e | 0xA5e7cD85b15586ecb8DA34AcEE42FF83ABcB555b |
| WBTC | 0xC04B0d3107736C32e19F1c62b2aF67BE61d63a05 | 0x7825cB7feEAD896241f748c89550F3D01AF51e48 |
| DAI  | 0xdc31Ee1784292379Fbb2964b3B9C4124D8F89C60 | 0x71339a9C403383c3E18712130615d369Ff9a7124 |

### Code

1. CronosGravity :
   - https://github.com/crypto-org-chain/gravity-bridge/blob/v2.0.0-cronos-alpha0/solidity/contracts/CronosGravity.sol

2. Eth Gravity Wrapper :
   -  https://github.com/crypto-org-chain/gravity-bridge/blob/v2.0.0-cronos-alpha0/solidity/contracts/EthGravityWrapper.sol

3. CroBridge :
   - https://github.com/crypto-org-chain/cronos/blob/v0.8.0-gravity-alpha0/integration_tests/contracts/contracts/CroBridge.sol


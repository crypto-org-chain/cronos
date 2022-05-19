

# Gravity Bridge Testnet Deployment Guide

This guide is intended to assist the community validators with setting up Gravity Bridge between `Cronos Gravity Testnet2` and `Ethereum` Kovan testnet.


## Prerequisites

### Validator

You should have a validator running in `Cronos Gravity Testnet2` network.

### Ethereum node

You need to have access to the EVM RPC endpoint of an Ethereum node. Or host your own node with [go-ethereum](https://github.com/ethereum/go-ethereum/) or [openethereum](https://github.com/openethereum/openethereum).

### Binaries

-  `cronosd` version: `0.7.0` , the cronos node binary found at https://github.com/crypto-org-chain/cronos/releases/tag/v0.7.0.

- `gorc`, the gravity bridge orchestrator cli, built from the [crypto-org fork](https://github.com/crypto-org-chain/gravity-bridge/tree/v2.0.0-cronos/orchestrator/gorc).

- Above essential binaries setup in `PATH`.

## Generate Orchestrator Keys

You need to prepare two accounts for the orchestrator, one for ethereum and one for cronos. You should transfer some funds to these accounts, so the orchestrator can cover the gas fees of message relaying later.

### Creating the config:

We will use the below `config.toml` to specify the directory where the keys will be generated and some of the configs needed to run the orchestrator. Creating a `gorc.toml` file in the same directory as `gorc` and paste the following config:


```toml
keystore = "/tmp/keystore"

[gravity]
contract = "0x0000000000000000000000000000000000000000" # TO BE UPDATED - gravity contract address on Ethereum network
fees_denom = "basetcro"

[ethereum]
key_derivation_path = "m/44'/60'/0'/0/0"
rpc = "http://localhost:8545" # TO BE UPDATED - EVM RPC of Ethereum node

[cosmos]
gas_price = { amount = 5000000000000, denom = "basetcro" }
grpc = "http://localhost:9090" # TO BE UPDATED - GRPC of Cronos node
key_derivation_path = "m/44'/60'/0'/0/0"
prefix = "tcrc"

[metrics]
listen_addr = "127.0.0.1:3000"
```

The keys below will be created in `/tmp/keystore` directory.


### Creating a Cronos account:

```shell
gorc -c gorc.toml keys cosmos add orch_cro
```

Sample output:
```
**Important** record this bip39-mnemonic in a safe place:
lava ankle enlist blame vast blush proud split position just want cinnamon virtual velvet rubber essence picture print arrest away size tip exotic crouch
orch_cro        tcrc1ypvpyjcny3m0wl5hjwld2vw8gus2emtzmur4he
```

### Creating an Ethereum account:

Using the `gorc` binary, you can run:

```shell
gorc -c gorc.toml keys eth add orch_eth
```

Sample out:
```
**Important** record this bip39-mnemonic in a safe place:
more topic panther diesel grace chaos stereo timber tired settle target carbon scare essence hobby worry sword vibrant fruit update acquire release art drift
0x838a3EC266ddb27f5924989505cBFa15fAf88603
```
The second line is the mnemonic and the third one is the public address.

To get the private key (optional), in Python shell:

```python
from eth_account import Account
Account.enable_unaudited_hdwallet_features()
my_acct = Account.from_mnemonic("mystery exotic patch broom sweet sense grocery carpet assist oxygen fault peanut muffin hole popular excite apart fetch lens palace soccer paddle gaze focus") # please use your own mnemnoic
print(my_acct.privateKey.hex()) # Ethereum private key. Keep private and secure e.g. '0xe9580d74831b9611c9680ecde4ea016dee55643fe86901708bafd90a8ef716b6'
```
Note that `eth_account` python package needs to be installed.

## Transfer funds to orchestrator accounts

You should transfer funds to the Ethereum and Cronos accounts generated earlier. Gravity Bridge is deployed between the `Cronos Gravity Testnet2` and the `Ethereum` Kovan testnet.


## Sign Validator Address


### Prerequisites:

1. Get **validator address**:

	If you have your validator key set up locally, you can run:

	```shell
	cronosd keys show $val_key_name --bech val --output json | jq .address
	```

	Sample out:
	`"tcrcvaloper18d5ne2f2xdge9s4yw0wr6h8gpvg5p7lec4eefk"`

2. Get **validator account address**:

	If you have your validator key set up locally, you can run:

	```shell
	cronosd keys show $val_key_name --output json | jq .address
	```

	Sample out:
		`"tcrc18d5ne2f2xdge9s4yw0wr6h8gpvg5p7lep8zx6p"`

3. Get validator current `nonce`:

	```shell
	cronosd query auth account $val_account_add_from_2 --output json | jq .base_account.sequence
	```

  Sample out:
	"15"

4. Have an Ethereum private key. To create a new one, refer to [Creating an Ethereum account](#creating-an-ethereum-account)

### Generating the signature:

To register the orchestrator with the validator, you need to sign a protobuf encoded message using the orchestrator's Ethereum key, and send it to a Cronos validator to register it.

To get the signature, we will use `gorc` as follows:

```shell
gorc sign-delegate-keys orch_eth $val_address $nonce
```

Sample out:

`0x530742a07ee3bed639b91fe6d9a7ed9bfb4352183eafd332fba431dcb4721ebb1a5d058018a71dd51051aceaa69e1bbc8763336da26de5bbae30c5b624d7ec781b`

Note that:
1. orch_eth was configured in [here](#creating-an-ethereum-account).
2. `$val_address` and `$nonce` were obtained from the Prerequisites section.


## Register Orchestrator With Cronos Validator


At last, send the orchestrator's Ethereum address, Cronos address, and the signature we just signed above to a Cronos validator, the validator should send a `set-delegate-keys` transaction to cronos network to register the binding:


```shell

cronosd tx gravity set-delegate-keys $val_address  $orchestrator_cronos_address  $orchestrator_eth_address  $signature --from $val_account_address

```

You might also need to set `--chain-id cronosgravitytestnet_340-2` and `--fees`.


## Run Orchestrator

```shell
./gorc -c ./gorc.toml orchestrator start \
		--cosmos-key="orch_cro" \
		--ethereum-key="orch_eth"
```

The orchestrator is running now.

  
# Gravity Bridge Deployment Guide

This guide is intended to assist the community validators with setting up crypto.org Gravity Bridge against `Cronos Testnet 3`  


## Prerequisites

### Validator

You should have a validator running in the `Cronos Testnet 3` network.

### Binaries

-  `cronosd` version: `0.5.5-testnet` , the cronos node binary found at https://github.com/crypto-org-chain/cronos/releases/tag/v0.5.5-testnet.

-  `orchestrator`, the gravity bridge orchestrator cli, build from the [crypto-org fork](https://github.com/crypto-org-chain/gravity-bridge/tree/cronos/orchestrator/orchestrator).

- `gorc`, the gravity bridge orchestrator cli, build from the [crypto-org fork](https://github.com/crypto-org-chain/gravity-bridge/tree/cronos/orchestrator/gorc).

- Above essential binaries setup in `PATH`.

## Generate Orchestrator Keys

You need to prepare two accounts for the orchestrator, one for ethereum and one for cronos, you can also use the same private key for both accounts. You should transfer some funds to these accounts, so the orchestrator can cover the gas fees of messages relaying later.

### Creating a Cronos account:

```shell
cronosd keys add $local_key_name
```

Sample output:
```
- name: abcd
  type: local
  address: tcrc18d5ne2f2xdge9s4yw0wr6h8gpvg5p7lep8zx6p
  pubkey: '{"@type":"/ethermint.crypto.v1.ethsecp256k1.PubKey","key":"AwsZQ1HV/x4vfcXIJDeiYmdq1n1G/8tbtSFAsKZ+HLy2"}'
  mnemonic: ""
```

### Creating an Ethereum account:

Using the `gorc` binary, you can run:

```shell
gorc keys eth add eth1
```

Sample out:
```
**Important** record this bip39-mnemonic in a safe place:
mystery exotic patch broom sweet sense grocery carpet assist oxygen fault peanut muffin hole popular excite apart fetch lens palace soccer paddle gaze focus
0xA31C416cA9e21e8EDD7682C8b7D656289e52D1eb
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

You should transfer funds to the Ethereum and Cronos accounts generated earlier. `Cronos Testnet 3` Gravity Bridge is deployed between the testnet and the `Kovan` Ethereum testnet. 


## Sign Validator Address


### Prerequisites:

1. Get **validator address**:

	If you have your key set up locally, you can run:

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
gorc sign-delegate-keys eth1 $val_address $nonce
```

Sample out:

`0x530742a07ee3bed639b91fe6d9a7ed9bfb4352183eafd332fba431dcb4721ebb1a5d058018a71dd51051aceaa69e1bbc8763336da26de5bbae30c5b624d7ec781b`

Note that:
1. eth1 was configured in [here](#creating-an-ethereum-account).
2. `$val_address` and `$nonce` were obtained from the Prerequisites section.
  

## Register Orchestrator With Cronos Validator


At last, send the orchestrator's Ethereum address, Cronos address, and the signature we just signed above to a Cronos validator, the validator should send a `set-delegate-keys` transaction to cronos network to register the binding:


```shell

cronosd tx gravity set-delegate-keys $val_address  $orchestrator_cronos_address  $orchestrator_eth_address  $signature --from $val_account_address

```

You might also need to set `--chain-id cronostestnet_338-3` and `--fees`. 


## Run Orchestrator


```shell

orchestrator --cosmos-phrase="{mnemonic_phrase_of_cronos_acount}" \

--ethereum-key={private_key_of_ethereum_account} \

--cosmos-grpc=http://localhost:{cronos_grpc_port} \

--ethereum-rpc={ethereum_web3_endpoint} \

--address-prefix=tcrc --fees=basetcro \

--contract-address={gravity_contract_address} \

--metrics-listen 127.0.0.1:3000 --hd-wallet-path="m/44'/60'/0'/0/0"

```


After all the orchestrator processes run, the gravity bridge between ethereum and cronos is setup succesfully.
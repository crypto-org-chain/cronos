# Gravity Bridge Dev Setup Guide

## Prerequisite

### Binaries

- `geth`, the go-ethereum binary.
- `cronosd`, the cronos node binary.
- `orchestrator`, the gravity bridge orchestrator cli, build from the [crypto-org fork](https://github.com/crypto-org-chain/gravity-bridge/tree/cronos/orchestrator/orchestrator).
- `pystarport`, a tool to run local cosmos devnet.
- `start-geth`/`start-cronos`, convenient scripts to start the local devnets.

Clone cronos repo locally and run `nix-shell integration_tests/shell.nix` in it, you'll get a virtual shell with the
above essential binaries setup in `PATH`.

### Ethereum Testnet

You can either use a public testnet, or run `start-geth /tmp/test-geth` to get a local Ethereum testnet.

You should own some funds in this testnet, for the local testnet, you can get the funds using this mnemonic words:
`visit craft resemble online window solution west chuckle music diesel vital settle comic tribe project blame bulb armed
flower region sausage mercy arrive release`.

### Cronos Testnet

You can either use a public cronos testnet (that have embed the gravity-bridge module), or run `start-cronos
/tmp/test-cronos` to get a local Cronos testnet.

You should own some funds in this testnet, for the local testnet, you'll get the funds with the same private key as
above.

## Generate Orchestrator Keys

You need to prepare two accounts for the orchestrator, one for ethereum and one for cronos, you can also use a same
private key for both accounts. You should transfer some funds to these accounts, so the orchestrator can cover the gas
fees of messages relaying later.

## Sign Validator Address

To register the orchestrator with the validator, you need to sign a protobuf encoded message using the orchestrator's
ethereum key, and send it to a cronos validator to register it.

The protobuf message is like this:

```protobuf
message DelegateKeysSignMsg {
  // The valoper prefixed cronos validator address
  string validator_address = 1;
  // Current nonce of the validator account
  uint64 nonce = 2;
}
```

Use your favorite protobuf library to encode the message, and use your favorite web3 library to do the messge signing,
for example, this is how it could be done in python:

```python
msg = DelegateKeysSignMsg(validator_address=val_addr, nonce=nonce)
sign_bytes = eth_utils.keccak(msg.SerializeToString())

acct = eth_account.Account.from_key(...)
signed = acct.sign_message(eth_account.messages.encode_defunct(sign_bytes))
return eth_utils.to_hex(signed.signature)
```

## Register Orchestrator With Cronos Validator

At last, send the orchestrator's ethereum address, cronos address, and the signature we just signed above to a Cronos
validator, the validator should send a `set-delegate-keys` transaction to cronos network to register the binding:

```shell
$ cronosd tx gravity set-delegate-keys $val_address $orchestrator_cronos_address $orchestrator_eth_address $signature
```

## Deploy Gravity Contract On Ethereum

The gravity contract can only be deployed after majority validators (66% voting powers) have registered the
orchestrator. And before deploy gravity contract, we need to prepare the [parameters for the
constructor](https://github.com/PeggyJV/gravity-bridge/blob/cfd55296dfb981dd7a18cefa2da9e21410fa0403/solidity/contracts/Gravity.sol#L561)
first:

- `gravity_id`. Run command `cronosd q gravity params | jq ".params.gravity_id"`
- `threshold`, constant `2834678415`, which is just `int(2 ** 32 * 0.66)`.
- `eth_addresses` and `powers`:
  - Query signer set by running command: `cronosd q gravity latest-signer-set-tx | jq ".signer_set.signers"`
  - Sum up the `power` field to get `powers`
  - Collect the `ethereum_address` field into a list to get `eth_addresses`

At last, use your favorite web3 library/tool to deploy the gravity contract with the above parameters in the ethereum
testnet, the compiled artifacts of the contract (`Gravity.json`) can be found in [gravity-bridge's
releases](https://github.com/PeggyJV/gravity-bridge/releases).

## Run Orchestrator

```shell
$ orchestrator --cosmos-phrase="{mnemonic_phrase_of_cronos_acount}" \
    --ethereum-key={private_key_of_ethereum_account} \
    --cosmos-grpc=http://localhost:{cronos_grpc_port} \
    --ethereum-rpc={ethereum_web3_endpoint} \
    --address-prefix=crc --fees=basetcro \
    --contract-address={gravity_contract_address} \
    --metrics-listen 127.0.0.1:3000 --hd-wallet-path="m/44'/60'/0'/0/0"
```

After all the orchestrator processes run, the gravity bridge between ethereum and cronos is setup succesfully.

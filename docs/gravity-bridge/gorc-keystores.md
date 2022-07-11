## Gravity Orchestrator Keystores

gorc supports multiple keystores to add, restore, and retrieve keys used for orchestrators and relayers.

This guide shows how to configure the different keystores with gorc.

### Prerequisites

 - `gorc`, the gravity bridge orchestrator cli, build instructions can be found [here](gorc-build.md). Alternatively, you can download Linux x86_64 binary from [here](https://github.com/crypto-org-chain/gravity-bridge/releases/tag/v2.0.0-cronos-alpha0).
 - Above binaries setup in `PATH`.


### Filesystem Keystore

####  Creating the config:

We will use the below `config.toml` to specify the keystore where the keys will be generated and retrieved, along with some of the configs needed to run the orchestrator/ relayer. Create a `gorc.toml` file in the same directory as `gorc` and paste the following config:

```toml
keystore = "/tmp/keystore"

[gravity]
contract = "0x0000000000000000000000000000000000000000" # TODO - gravity contract address on Ethereum network
fees_denom = "basetcro"

[ethereum]
key_derivation_path = "m/44'/60'/0'/0/0"
rpc = "http://localhost:8545" # TODO - EVM RPC of Ethereum node

[cosmos]
gas_price = { amount = 5000000000000, denom = "basetcro" } # TODO basecro for mainnet, basetcro for testnet
grpc = "http://localhost:9090" # TODO - GRPC of Cronos node
key_derivation_path = "m/44'/60'/0'/0/0"
prefix = "tcrc" # TODO - crc for mainnet, tcrc for testnet

[metrics]
listen_addr = "127.0.0.1:3000"
```

The keys will be created in `/tmp/keystore` directory.

### AWS Secret Manager Keystore

#### EC2 instance with proper role:

An EC2 instance with an appropriate role and permissions is needed in order to generate the orchestrator keys and automatically create the keys (PEM format) in AWS Secrets Manager.

The role should have a policy as follows:
```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "secretsmanager:GetSecretValue",
                "secretsmanager:DescribeSecret",
                "secretsmanager:PutSecretValue",
                "secretsmanager:CreateSecret"
            ],
            "Resource": [
                "arn:aws:secretsmanager:$region:$account-id:secret:$keys-prefix*",
            ]
        }
    ]
}
```
You will need to replace the following variables above:

- `$region`: region where you want to store the keys at
- `$account-id`: AWS account id
- `$keys-prefix`: prefix for the keys created (e.g. cronos-testnet3-orch*). Then you'd be able to create keys `cronos-testnet3-orch1` and `cronos-testnet3-orch2`


####  Creating the config:

We will use the below `config.toml` to specify the keystore where the keys will be generated and retrieved, along with some of the configs needed to run the orchestrator/ relayer. Create a `gorc.toml` file in the same directory as `gorc` and paste the following config:

```toml
keystore = "Aws"

[gravity]
contract = "0x0000000000000000000000000000000000000000" # TODO - gravity contract address on Ethereum network
fees_denom = "basetcro"

[ethereum]
key_derivation_path = "m/44'/60'/0'/0/0"
rpc = "http://localhost:8545" # TODO - EVM RPC of Ethereum node

[cosmos]
gas_price = { amount = 5000000000000, denom = "basetcro" } # TODO basecro for mainnet, basetcro for testnet
grpc = "http://localhost:9090" # TODO - GRPC of Cronos node
key_derivation_path = "m/44'/60'/0'/0/0"
prefix = "tcrc" # TODO - crc for mainnet, tcrc for testnet

[metrics]
listen_addr = "127.0.0.1:3000"
```

### Creating accounts

#### Creating a Cronos account:

```shell
gorc -c gorc.toml keys cosmos add orch_cro
```

Sample output:
```
**Important** record this bip39-mnemonic in a safe place:
lava ankle enlist blame vast blush proud split position just want cinnamon virtual velvet rubber essence picture print arrest away size tip exotic crouch
orch_cro        tcrc1ypvpyjcny3m0wl5hjwld2vw8gus2emtzmur4he
```

The second line is the mnemonic (for security reasons, this will not be printed out for AWS Secret Manager keystore) and the third one is the public address.

#### Creating an Ethereum account:

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
The second line is the mnemonic (for security reasons, this will not be printed out for AWS Secret Manager keystore) and the third one is the public address.

To get the private key (optional), in Python shell:

```python
from eth_account import Account
Account.enable_unaudited_hdwallet_features()
my_acct = Account.from_mnemonic("mystery exotic patch broom sweet sense grocery carpet assist oxygen fault peanut muffin hole popular excite apart fetch lens palace soccer paddle gaze focus") # please use your own mnemnoic
print(my_acct.privateKey.hex()) # Ethereum private key. Keep private and secure e.g. '0xe9580d74831b9611c9680ecde4ea016dee55643fe86901708bafd90a8ef716b6'
```
Note that `eth_account` python package needs to be installed.
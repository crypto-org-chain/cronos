#!/bin/bash

make install

CHAINID="cronos_777-1"
MONIKER="localtestnet"

# localKey address 0x7cb61d4117ae31a12e393a1cfa3bac666481d02e
# crc10jmp6sgh4cc6zt3e8gw05wavvejgr5pw8v2q7j
VAL_KEY="localkey"
VAL_MNEMONIC="gesture inject test cycle original hollow east ridge hen combine junk child bacon zero hope comfort vacuum milk pitch cage oppose unhappy lunar seat"

# user1 address 0xc6fe5d33615a1c52c08018c47e8bc53646a0e101
# crc1cml96vmptgw99syqrrz8az79xer2pcgpj22459
USER1_KEY="user1"
USER1_MNEMONIC="copper push brief egg scan entry inform record adjust fossil boss egg comic alien upon aspect dry avoid interest fury window hint race symptom"

# user2 address 0x963ebdf2e1f8db8707d05fc75bfeffba1b5bac17
# crc1jcltmuhplrdcwp7stlr4hlhlhgd4htqhyz4ack
USER2_KEY="user2"
USER2_MNEMONIC="maximum display century economy unlock van census kite error heart snow filter midnight usage egg venture cash kick motor survey drastic edge muffin visual"

# remove existing daemon and client
rm -rf ~/.cronos*
# Import keys from mnemonics
echo $VAL_MNEMONIC | cronosd keys add $VAL_KEY --recover --keyring-backend test --algo "eth_secp256k1"
echo $USER1_MNEMONIC | cronosd keys add $USER1_KEY --recover --keyring-backend test --algo "eth_secp256k1"
echo $USER2_MNEMONIC | cronosd keys add $USER2_KEY --recover --keyring-backend test  --algo "eth_secp256k1"

cronosd init $MONIKER --chain-id $CHAINID

# Set fast block time for testing (100ms)
sed -i.bak 's/timeout_commit = ".*"/timeout_commit = "100ms"/' $HOME/.cronos/config/config.toml

# Set gas limit in genesis
#cat $HOME/.cronos/config/genesis.json | jq '.consensus_params["block"]["max_gas"]="10000000"' > $HOME/.cronos/config/tmp_genesis.json && mv $HOME/.cronos/config/tmp_genesis.json $HOME/.cronos/config/genesis.json
# cat $HOME/.cronos/config/genesis.json | jq '.app_state["cronos"]["params"]["ibc_cro_denom"]="stake"' > $HOME/.cronos/config/tmp_genesis.json && mv $HOME/.cronos/config/tmp_genesis.json $HOME/.cronos/config/genesis.json
# cat $HOME/.cronos/config/genesis.json | jq '.app_state["gravity"]["params"]["ethereum_blacklist"]=["0x000000000000000000000000000000000000dEaD"]' > $HOME/.cronos/config/tmp_genesis.json && mv $HOME/.cronos/config/tmp_genesis.json $HOME/.cronos/config/genesis.json
cat $HOME/.cronos/config/genesis.json | jq '.consensus["params"]["block"]["max_gas"]="10000000"' > $HOME/.cronos/config/tmp_genesis.json && mv $HOME/.cronos/config/tmp_genesis.json $HOME/.cronos/config/genesis.json

# Allocate genesis accounts (cosmos formatted addresses)
cronosd genesis add-genesis-account "$(cronosd keys show $VAL_KEY -a --keyring-backend test)" 1000000000000000000000aphoton,1000000000000000000stake --keyring-backend test
cronosd genesis add-genesis-account "$(cronosd keys show $USER1_KEY -a --keyring-backend test)" 1000000000000000000000aphoton,1000000000000000000stake --keyring-backend test
cronosd genesis add-genesis-account "$(cronosd keys show $USER2_KEY -a --keyring-backend test)" 1000000000000000000000aphoton,1000000000000000000stake --keyring-backend test

# Sign genesis transaction
cronosd genesis gentx $VAL_KEY 1000000000000000000stake --amount=1000000000000000000000aphoton --chain-id $CHAINID --keyring-backend test

# Collect genesis tx
cronosd genesis collect-gentxs

# Run this to ensure everything worked and that the genesis file is setup correctly
cronosd genesis validate

echo "blocked-addresses = ['crc16z0herz998946wr659lr84c8c556da55dc34hh']" >> $HOME/.cronos/config/app.toml

# Start the node (remove the --pruning=nothing flag if historical queries are not needed)
cronosd start --pruning=nothing --rpc.unsafe --log_level info --minimum-gas-prices 200000aphoton \
  --p2p.laddr tcp://127.0.0.1:26650 \
  --rpc.laddr tcp://127.0.0.1:26657 \
  --grpc.address 127.0.0.1:26653 \
  --api.enable \
  --json-rpc.address 127.0.0.1:26695 \
  --json-rpc.ws-address 127.0.0.1:26696 \
  --json-rpc.api eth,txpool,personal,net,debug,web3 \
  --da-ibc-version=v1

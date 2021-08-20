#!/bin/sh
exists() {
    [ -e "$1" ]
}

CHAINID="cronos_777-1"

for i in `seq 0 1`
do
    echo "setup node$i"
    if ! exists ./data/$CHAINID/node$i/UTC* ; then
        ./build/cronosd eth_keys add --home="./data/$CHAINID/node$i" --passphrase default
    fi
    KEYSTORE=`ls -1 ./data/$CHAINID/node$i/UTC* | head -n 1`
    VAL_ADDR=`./build/cronosd keys show -a validator --bech val --home "data/$CHAINID/node$i"`
    ACC_ADDR=`./build/cronosd keys show -a validator --bech acc --home "data/$CHAINID/node$i"`
    ETH_ADDR="0x"`jq -r ".address" $KEYSTORE`
    NONCE=`./build/cronosd q auth account $ACC_ADDR --output json | jq -r ".base_account.sequence"`
    echo $NONCE
    SIGNATURE=`./integration_tests/helper.py sign_validator $KEYSTORE $VAL_ADDR $NONCE --passphrase default`
    ETH_PRIV=`./integration_tests/helper.py decrypt_keystore $KEYSTORE --passphrase default`
    ./build/cronosd tx gravity set-delegate-keys $VAL_ADDR $ACC_ADDR $ETH_ADDR $SIGNATURE \
        --home "./data/$CHAINID/node$i" --from validator -y
done

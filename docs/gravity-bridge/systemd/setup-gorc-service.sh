#!/usr/bin/env bash

set -e

# help section
usage()
{
cat << EOF
usage: bash ./setup-gorc-service -t type
-t    | --type              (Required)            type of gorc service to create (orchestrator or relayer)
-h    | --help                                    brings up this menu
EOF
}

# parse arguments
while [ "$1" != "" ]; do
    case $1 in
        -t | --type )
            shift
            type=$1
        ;;
        -h | --help )    usage
            exit
        ;;
        * )              usage
            exit 1
    esac
    shift
done

if [[ ! "$OSTYPE" == "linux-gnu"* ]]; then
    echo -e "\033[31mCan only create /etc/systemd/system/gorc.service for linux\033[0m" 1>&2
    exit 1
fi

# check for required args
if [ -z "$type" ]; then
    echo "type of service is required, provide it with the flag: -t type"
    exit 1
fi

if [[ ! "$type" =~ ^(relayer|orchestrator)$ ]]; then
    echo "type must be one of (relayer, orchestrator). $type is not supported"
    exit 1
fi

BASEDIR=/tmp

check_gorc_setup() {
    GORC_BINARY=$(which gorc || (echo -e "\033[31mPlease add gorc to PATH\033[0m" 1>&2 && exit 1))
    GORC_USER=$USER
    GORC_BINARY_DIR=$(dirname $(which gorc))
    GORC_USER_HOME=$(eval echo "~$USER")
}

download_service() {
    curl -s https://raw.githubusercontent.com/crypto-org-chain/cronos/main/docs/gravity-bridge/systemd/gorc.service.template -o $BASEDIR/gorc.service.template
}

gather_relayer_info() {
    read -p 'Please input relayer ethereum key name: ' ethKey
    echo "eth key: $ethKey"
    GORC_START_COMMAND="relayer start --mode Api --ethereum-key \"$ethKey\""
}

gather_orchestrator_info() {
    read -p 'Please input orchestrator ethereum key name: ' ethKey
    read -p 'Please input orchestrator cronos key name: ' croKey
    echo "cronos key: $croKey eth key: $ethKey"
    GORC_START_COMMAND="orchestrator start --mode Api --cosmos-key=\"$croKey\" --ethereum-key=\"$ethKey\""
}

setup_service() {
    sed "s#<GORC_BINARY>#$GORC_BINARY#g; s#<GORC_USER>#$GORC_USER#g; s#<GORC_BINARY_DIR>#$GORC_BINARY_DIR#g; s#<GORC_USER_HOME>#$GORC_USER_HOME#g; s#<GORC_START_COMMAND>#$GORC_START_COMMAND#g" $BASEDIR/gorc.service.template > $BASEDIR/gorc.service
    echo -e "\033[32mGenerated $BASEDIR/gorc.service\033[0m"

    sudo cp $BASEDIR/gorc.service /etc/systemd/system/gorc.service
    sudo systemctl daemon-reload
    sudo systemctl enable gorc.service
    echo -e "\033[32mCreated /etc/systemd/system/gorc.service\033[0m"
}


check_gorc_setup
download_service

if [ "$type" == "relayer" ]; then
    gather_relayer_info
elif [ "$type" == "orchestrator" ]; then
    gather_orchestrator_info
fi

setup_service

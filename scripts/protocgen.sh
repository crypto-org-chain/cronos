#!/usr/bin/env bash

set -eo pipefail

echo "Generating gogo proto code"
cd proto
proto_dirs=$(find . -path -prune -o -name '*.proto' -print0 | xargs -0 -n1 dirname | sort | uniq)
for dir in $proto_dirs; do
  for file in $(find "${dir}" -maxdepth 1 -name '*.proto'); do
    if grep "option go_package" $file &> /dev/null ; then
      buf generate --template buf.gen.gogo.yaml $file
    fi
  done
done

cd ..

# move proto files to the right places
cp -r github.com/crypto-org-chain/cronos/* ./
cp -r github.com/crypto-org-chain/cronos/memiavl/* ./memiavl/

# move attestation proto files to relayer folder (handle v2 path)
if [ -d "github.com/crypto-org-chain/cronos/v2/relayer/types" ]; then
  mkdir -p ./relayer/types
  cp -r github.com/crypto-org-chain/cronos/v2/relayer/types/* ./relayer/types/
  echo "Moved attestation proto files to relayer/types"
elif [ -d "github.com/crypto-org-chain/cronos/relayer/types" ]; then
  mkdir -p ./relayer/types
  cp -r github.com/crypto-org-chain/cronos/relayer/types/* ./relayer/types/
  echo "Moved attestation proto files to relayer/types"
fi

rm -rf github.com

# TODO uncomment go mod tidy after upgrading to ghcr.io/cosmos/proto-builder v0.12.0
# go mod tidy



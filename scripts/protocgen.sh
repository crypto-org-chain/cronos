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

# optionally sync memiavl protos with a local cronos-store checkout
TARGET_STORE_DIR="${CRONOS_STORE_PROTO_OUT:-}"
if [[ -z "${TARGET_STORE_DIR}" && -d "../cronos-store" ]]; then
  TARGET_STORE_DIR="../cronos-store"
fi
if [[ -n "${TARGET_STORE_DIR}" ]]; then
  echo "Syncing memiavl generated code to ${TARGET_STORE_DIR}"
  mkdir -p "${TARGET_STORE_DIR}/memiavl"
  cp -r github.com/crypto-org-chain/cronos-store/memiavl/* "${TARGET_STORE_DIR}/memiavl/"
fi

rm -rf github.com

# TODO uncomment go mod tidy after upgrading to ghcr.io/cosmos/proto-builder v0.12.0
# go mod tidy

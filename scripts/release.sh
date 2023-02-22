#!/bin/bash
set -e

baseurl="."
build_type="tarball"
build_platform="$(nix eval --impure --raw --expr 'builtins.currentSystem')"
GITHUB_REF_NAME=${GITHUB_REF_NAME:=devel}

build() {
    network=$1
    host="$2"
    name="$3"
    pkg="cronosd${network}-${build_type}"
    if [[ "$host" == "native" ]]; then
        FLAKE="${baseurl}#${pkg}"
    else
        FLAKE="${baseurl}#legacyPackages.${build_platform}.pkgsCross.${host}.cronos-matrix.${pkg}"
    fi
    echo "building $FLAKE"
    nix build -L "$FLAKE"
    cp result "cronos_${GITHUB_REF_NAME:1}${network}_${name}.tar.gz"
}

if [[ "$build_platform" == "x86_64-linux" ]]; then
    hosts="Linux_x86_64,native Linux_arm64,aarch64-multiplatform Windows_x86_64,mingwW64"
elif [[ "$build_platform" == "aarch64-linux" ]]; then
    hosts="Linux_arm64,native Linux_x86_64,gnu64 Windows_x86_64,mingwW64"
elif [[ "$build_platform" == "x86_64-darwin" ]]; then
    hosts="Darwin_x86_64,native Darwin_arm64,aarch64-darwin"
else
    echo "don't support build platform: $build_platform" 
    exit 1
fi

for network in "" "-testnet"; do
    for t in $hosts; do
        IFS=',' read name host <<< "${t}"
        build "$network" "$host" "$name"
    done
done

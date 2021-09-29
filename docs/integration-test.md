# Integration Test

## Clone the cronos repository
```shell
git clone https://github.com/crypto-org-chain/cronos.git
```

## Install [nix](https://nixos.org/download.html)
```shell
curl -L https://nixos.org/nix/install | sh -s
```

## For macOS (>= 10.15), you need to run the following command instead
```shell
curl -L https://nixos.org/nix/install | sh -s -- --darwin-use-unencrypted-nix-store-volume
```

## To ensure that the necessary environment variables are set, please add the line to your shell profile (e.g. `~/.profile`)
```shell
source ~/.nix-profile/etc/profile.d/nix.sh
```

## Install [cachix](https://github.com/cachix/cachix)
```shell
nix-env -iA cachix -f https://cachix.org/api/v1/install
```

## Configure a binary cache by writing nix.conf and netrc files
```shell
cachix use cronos
nix path-info --all | grep -v '.drv$' > /tmp/store-path-pre-build
```

## Run the Integration Test
```shell
make run-integration-tests
```

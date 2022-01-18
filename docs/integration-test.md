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
cachix use dapp  # it's necessary to use dapp's binary cache on new macos system.
```

## Run All Integration Tests
```shell
make run-integration-tests
```

## Customize The Test Runner

We use `pytest` to discover the test cases and run them, follow [pytest doc](https://docs.pytest.org/en/6.2.x/contents.html) for more options.

You can invoke `pytest` after entering the nix shell:

```shell
$ nix-shell integration_tests/shell.nix
<nix-shell> $ pytest
```

You can use `-k` to select test cases by patterns in name:

```shell
<nix-shell> $ # run against cronos only
<nix-shell> $ pytest -k cronos
<nix-shell> $ # run against geth only
<nix-shell> $ pytest -k geth
```

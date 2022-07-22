# Integration Test

## Clone the cronos repository
```shell
git clone https://github.com/crypto-org-chain/cronos.git
```

## Install [nix](https://nixos.org/download.html)

https://nixos.org/download.html

Multi-user installation:

```shell
sh <(curl -L https://nixos.org/nix/install) --daemon
```

Make sure the following line has been added to your shell profile (e.g. `~/.profile`):

```shell
source ~/.nix-profile/etc/profile.d/nix.sh
```

Then re-login shell, the nix installation is completed.

## Install [cachix](https://github.com/cachix/cachix)

```shell
nix-env -iA cachix -f https://cachix.org/api/v1/install
```

## Configure Binary Caches

Binary caches will save a lot of build times.

```shell
cachix use cronos
cachix use dapp  # it's necessary to use dapp's binary cache on new macos system.
```

## Run All Integration Tests
```shell
make run-integration-tests
```

## Customize The Test Runner

To customize the test runner, you can also issue commands separately.

### Enter `nix-shell`

It'll prepare all dependencies.

```shell
$ nix-shell integration_tests/shell.nix
<nix-shell> $
```

### Compile Test Contracts

```shell
$ cd integration_tests/contracts
$ HUSKY_SKIP_INSTALL=1 npm install
$ npm run typechain
$ cd ../../
```

### Run `pytest`

We use `pytest` to discover the test cases and run them, follow [pytest doc](https://docs.pytest.org/en/6.2.x/contents.html) for more options.

You can invoke `pytest` after entering the nix shell:

```shell
$ cd integration_tests
$ pytest -s -vv
```

You can use `-k` to select test cases by patterns in name:

```shell
$ cd integration_tests
$ pytest -k test_basic
```

Some test cases will run on both `geth` and `cronos`, you can also select the platform to run using `-k`:

```shell
$ cd integration_tests
$ # run against cronos only
$ pytest -k cronos
$ # run against geth only
$ pytest -k geth
```

### Print `test config`
```shell
$ jsonnet integration_tests/configs/default.jsonnet | yq -P 
```
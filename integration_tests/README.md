Integration tests run against both geth and ethermint. Geth runs in dev mode, ethermint devnet is start by pystarport.

## Run the tests

```shell
$ nix-shell
<nix-shell> $ pytest
```

```shell
<nix-shell> $ # run against cronos only
<nix-shell> $ pytest -k cronos
<nix-shell> $ # run against geth only
<nix-shell> $ pytest -k geth
```

## Fixtures

- `w3`, a web3 instance connected to a local devnet, it's parameterized over network type, cronos and geth, both are started with same genesis state.

- `cluster`, a cosmos api wrapper connected to a local cronos devnet, same as the one used in chain-main project.

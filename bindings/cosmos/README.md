# Generate bindings

```
solc08 --abi --bin contracts/src/CosmosTypes.sol -o build --overwrite
abigen --pkg lib --abi build/CosmosTypes.abi --bin build/CosmosTypes.bin --out ./bindings/cosmos/lib/cosmos_types.abigen.go --type CosmosTypes

solc08 --abi --bin contracts/src/Relayer.sol -o build --overwrite
abigen --pkg relayer --abi build/IRelayerModule.abi --bin build/IRelayerModule.bin --out ./bindings/cosmos/precompile/relayer/i_relayer_module.abigen.go --type RelayerModule
```
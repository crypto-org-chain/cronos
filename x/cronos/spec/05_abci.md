<!--
order: 5
-->

# ABCI

## InitGenesis

`InitGenesis` initializes the Cronos module genesis state by setting the `GenesisState` fields to the
store. In particular it sets the parameters and token mapping state.

## ExportGenesis

The `ExportGenesis` ABCI function exports the genesis state of the Cronos module. In particular, it
iterates all token mappings to genesis.

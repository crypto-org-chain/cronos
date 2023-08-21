package app

import (
	"encoding/json"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/simapp"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	evmtypes "github.com/evmos/ethermint/x/evm/types"
)

// StateFn returns the initial application state using a genesis or the simulation parameters.
// It panics if the user provides files for both of them.
// If a file is not given for the genesis or the sim params, it creates a randomized one.
func StateFn(cdc codec.JSONCodec, simManager *module.SimulationManager) simtypes.AppStateFn {
	var bondDenom string
	return simapp.AppStateFnWithExtendedCbs(
		cdc,
		simManager,
		NewDefaultGenesisState(cdc),
		func(moduleName string, genesisState interface{}) {
			if moduleName == stakingtypes.ModuleName {
				stakingState := genesisState.(*stakingtypes.GenesisState)
				bondDenom = stakingState.Params.BondDenom
			}
		},
		func(rawState map[string]json.RawMessage) {
			evmStateBz, ok := rawState[evmtypes.ModuleName]
			if !ok {
				panic("evm genesis state is missing")
			}

			evmState := new(evmtypes.GenesisState)
			cdc.MustUnmarshalJSON(evmStateBz, evmState)

			// we should replace the EvmDenom with BondDenom
			evmState.Params.EvmDenom = bondDenom

			// change appState back
			rawState[evmtypes.ModuleName] = cdc.MustMarshalJSON(evmState)
		},
	)
}

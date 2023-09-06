package app

import (
	"encoding/json"

	"github.com/cosmos/cosmos-sdk/codec"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	evmtypes "github.com/evmos/ethermint/x/evm/types"
)

func StateFn(cdc codec.JSONCodec, simManager *module.SimulationManager) simtypes.AppStateFn {
	var bondDenom string
	return simtestutil.AppStateFnWithExtendedCbs(
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

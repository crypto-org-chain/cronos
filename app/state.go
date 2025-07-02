package app

import (
	"encoding/json"

	evmtypes "github.com/evmos/ethermint/x/evm/types"

	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func StateFn(app *App) simtypes.AppStateFn {
	var bondDenom string
	return simtestutil.AppStateFnWithExtendedCbs(
		app.AppCodec(),
		app.SimulationManager(),
		app.DefaultGenesis(),
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
			app.AppCodec().MustUnmarshalJSON(evmStateBz, evmState)

			// we should replace the EvmDenom with BondDenom
			evmState.Params.EvmDenom = bondDenom

			// change appState back
			rawState[evmtypes.ModuleName] = app.AppCodec().MustMarshalJSON(evmState)
		},
	)
}

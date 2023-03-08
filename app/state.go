package app

import (
	"encoding/json"
	"math/rand"
	"time"

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
	appStateFn := simapp.AppStateFn(cdc, simManager)
	return func(r *rand.Rand, accs []simtypes.Account, config simtypes.Config,
	) (appState json.RawMessage, simAccs []simtypes.Account, chainID string, genesisTimestamp time.Time) {
		appState, simAccs, chainID, genesisTimestamp = appStateFn(r, accs, config)

		rawState := make(map[string]json.RawMessage)
		err := json.Unmarshal(appState, &rawState)
		if err != nil {
			panic(err)
		}

		stakingStateBz, ok := rawState[stakingtypes.ModuleName]
		if !ok {
			panic("staking genesis state is missing")
		}

		stakingState := new(stakingtypes.GenesisState)
		err = cdc.UnmarshalJSON(stakingStateBz, stakingState)
		if err != nil {
			panic(err)
		}

		// we should get the BondDenom and make it the evmdenom.
		// thus simulation accounts could have positive amount of gas token.
		bondDenom := stakingState.Params.BondDenom

		evmStateBz, ok := rawState[evmtypes.ModuleName]
		if !ok {
			panic("evm genesis state is missing")
		}

		evmState := new(evmtypes.GenesisState)
		cdc.MustUnmarshalJSON(evmStateBz, evmState)

		// we should replace the EvmDenom with BondDenom
		evmState.Params.EvmDenom = bondDenom
		rawState[evmtypes.ModuleName] = cdc.MustMarshalJSON(evmState)

		appState, err = json.Marshal(rawState)
		if err != nil {
			panic(err)
		}
		return
	}
}

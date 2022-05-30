package simulation_test

import (
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/crypto-org-chain/cronos/x/cronos/simulation"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

// TestRandomizedGenState tests the normal scenario of applying RandomizedGenState.
// Abonormal scenarios are not tested here.
func TestRandomizedGenState(t *testing.T) {
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	s := rand.NewSource(1)
	r := rand.New(s)

	simState := module.SimulationState{
		AppParams:    make(simtypes.AppParams),
		Cdc:          cdc,
		Rand:         r,
		NumBonded:    3,
		Accounts:     simtypes.RandomAccounts(r, 3),
		InitialStake: 1000,
		GenState:     make(map[string]json.RawMessage),
	}

	simulation.RandomizedGenState(&simState)

	var cronosGenesis types.GenesisState
	simState.Cdc.MustUnmarshalJSON(simState.GenState[types.ModuleName], &cronosGenesis)

	require.Equal(t, simulation.GenIbcCroDenom(r), cronosGenesis.Params.GetIbcCroDenom())
	require.Equal(t, simulation.GenIbcTimeout(r), cronosGenesis.Params.GetIbcTimeout())
	require.Equal(t, simulation.GenCronosAdmin(r, &simState), cronosGenesis.Params.GetCronosAdmin())
	require.Equal(t, simulation.GenEnableAutoDeployment(r), cronosGenesis.Params.GetEnableAutoDeployment())

	require.Equal(t, len(cronosGenesis.ExternalContracts), 0)
	require.Equal(t, len(cronosGenesis.AutoContracts), 0)
}

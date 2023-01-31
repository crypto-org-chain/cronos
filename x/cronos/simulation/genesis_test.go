package simulation_test

import (
	"encoding/json"
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"
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
		InitialStake: sdkmath.NewInt(1000),
		GenState:     make(map[string]json.RawMessage),
	}

	simulation.RandomizedGenState(&simState)

	var cronosGenesis types.GenesisState
	simState.Cdc.MustUnmarshalJSON(simState.GenState[types.ModuleName], &cronosGenesis)

	require.Equal(t, "ibc/7939cb6694d2c422acd208a0072939487f6999eb9d18a44784045d87f3c67cf2", cronosGenesis.Params.GetIbcCroDenom())
	require.Equal(t, uint64(0x68255aaf95e94627), cronosGenesis.Params.GetIbcTimeout())
	require.Equal(t, "cosmos1tnh2q55v8wyygtt9srz5safamzdengsnqeycj3", cronosGenesis.Params.GetCronosAdmin())
	require.Equal(t, true, cronosGenesis.Params.GetEnableAutoDeployment())

	require.Equal(t, len(cronosGenesis.ExternalContracts), 0)
	require.Equal(t, len(cronosGenesis.AutoContracts), 0)
}

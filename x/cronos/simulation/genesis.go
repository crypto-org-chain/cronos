package simulation

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"

	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"

	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

const (
	ibcCroDenomKey          = "ibc_cro_denom"
	ibcTimeoutKey           = "ibc_timeout"
	cronosAdminKey          = "cronos_admin"
	enableAutoDeploymentKey = "enable_auto_deployment"
	maxCallbackGasKey       = "max_callback_gas"
)

func GenIbcCroDenom(r *rand.Rand) string {
	randDenom := make([]byte, 32)
	r.Read(randDenom)
	return fmt.Sprintf("ibc/%s", hex.EncodeToString(randDenom))
}

func GenIbcTimeout(r *rand.Rand) uint64 {
	timeout := r.Uint64()
	return timeout
}

func GenCronosAdmin(r *rand.Rand, simState *module.SimulationState) string {
	adminAccount, _ := simtypes.RandomAcc(r, simState.Accounts)
	return adminAccount.Address.String()
}

func GenEnableAutoDeployment(r *rand.Rand) bool {
	return r.Intn(2) > 0
}

func GenMaxCallbackGas(r *rand.Rand) uint64 {
	maxCallbackGas := r.Uint64()
	return maxCallbackGas
}

// RandomizedGenState generates a random GenesisState for the cronos module
func RandomizedGenState(simState *module.SimulationState) {
	// cronos params
	var (
		ibcCroDenom          string
		ibcTimeout           uint64
		cronosAdmin          string
		enableAutoDeployment bool
		maxCallbackGas       uint64
	)

	simState.AppParams.GetOrGenerate(
		ibcCroDenomKey, &ibcCroDenom, simState.Rand,
		func(r *rand.Rand) { ibcCroDenom = GenIbcCroDenom(r) },
	)

	simState.AppParams.GetOrGenerate(
		ibcTimeoutKey, &ibcTimeout, simState.Rand,
		func(r *rand.Rand) { ibcTimeout = GenIbcTimeout(r) },
	)

	simState.AppParams.GetOrGenerate(
		cronosAdminKey, &cronosAdmin, simState.Rand,
		func(r *rand.Rand) { cronosAdmin = GenCronosAdmin(r, simState) },
	)

	simState.AppParams.GetOrGenerate(
		enableAutoDeploymentKey, &enableAutoDeployment, simState.Rand,
		func(r *rand.Rand) { enableAutoDeployment = GenEnableAutoDeployment(r) },
	)

	simState.AppParams.GetOrGenerate(
		maxCallbackGasKey, &ibcTimeout, simState.Rand,
		func(r *rand.Rand) { maxCallbackGas = GenIbcTimeout(r) },
	)

	params := types.NewParams(ibcCroDenom, ibcTimeout, cronosAdmin, enableAutoDeployment, maxCallbackGas)
	cronosGenesis := &types.GenesisState{
		Params:            params,
		ExternalContracts: nil,
		AutoContracts:     nil,
	}

	bz, err := json.MarshalIndent(cronosGenesis, "", " ")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected randomly generated %s parameters:\n%s\n", types.ModuleName, bz)

	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(cronosGenesis)
}

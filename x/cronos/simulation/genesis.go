package simulation

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"

	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"

	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

const (
	ibcCroDenomKey          = "ibc_cro_denom"
	ibcTimeoutKey           = "ibc_timeout"
	ibcTimeoutHeightKey     = "ibc_timeout_height"
	cronosAdminKey          = "cronos_admin"
	enableAutoDeploymentKey = "enable_auto_deployment"
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

func GenIbcTimeoutHeight(r *rand.Rand) string {
	return fmt.Sprintf("%d-%d", r.Uint64(), r.Uint64())
}

func GenCronosAdmin(r *rand.Rand, simState *module.SimulationState) string {
	adminAccount, _ := simtypes.RandomAcc(r, simState.Accounts)
	return adminAccount.Address.String()
}

func GenEnableAutoDeployment(r *rand.Rand) bool {
	return r.Intn(2) > 0
}

// RandomizedGenState generates a random GenesisState for the cronos module
func RandomizedGenState(simState *module.SimulationState) {
	// cronos params
	var (
		ibcCroDenom          string
		ibcTimeout           uint64
		ibcTimeoutHeight     string
		cronosAdmin          string
		enableAutoDeployment bool
	)

	simState.AppParams.GetOrGenerate(
		simState.Cdc, ibcCroDenomKey, &ibcCroDenom, simState.Rand,
		func(r *rand.Rand) { ibcCroDenom = GenIbcCroDenom(r) },
	)

	simState.AppParams.GetOrGenerate(
		simState.Cdc, ibcTimeoutKey, &ibcTimeout, simState.Rand,
		func(r *rand.Rand) { ibcTimeout = GenIbcTimeout(r) },
	)

	simState.AppParams.GetOrGenerate(
		simState.Cdc, ibcTimeoutHeightKey, &ibcTimeoutHeight, simState.Rand,
		func(r *rand.Rand) { ibcTimeoutHeight = GenIbcTimeoutHeight(r) },
	)

	simState.AppParams.GetOrGenerate(
		simState.Cdc, cronosAdminKey, &cronosAdmin, simState.Rand,
		func(r *rand.Rand) { cronosAdmin = GenCronosAdmin(r, simState) },
	)

	simState.AppParams.GetOrGenerate(
		simState.Cdc, enableAutoDeploymentKey, &enableAutoDeployment, simState.Rand,
		func(r *rand.Rand) { enableAutoDeployment = GenEnableAutoDeployment(r) },
	)

	params := types.NewParams(ibcCroDenom, cronosAdmin, ibcTimeoutHeight, enableAutoDeployment, ibcTimeout)
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

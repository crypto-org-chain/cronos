package cronos

import (
	"fmt"

	"github.com/crypto-org-chain/cronos/v2/x/cronos/keeper"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the capability module's state from a provided genesis
// state.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) {
	if err := k.SetParams(ctx, genState.Params); err != nil {
		panic(fmt.Sprintf("Invalid cronos module params: %v\n", genState.Params))
	}

	for _, m := range genState.ExternalContracts {
		// Only allowed to bootstrap external token at genesis
		if !types.IsValidIBCDenom(m.Denom) && !types.IsValidGravityDenom(m.Denom) {
			panic(fmt.Sprintf("Invalid denom to map to contract: %s", m.Denom))
		}
		if !common.IsHexAddress(m.Contract) {
			panic(fmt.Sprintf("Invalid contract address: %s", m.Contract))
		}
		if err := k.SetExternalContractForDenom(ctx, m.Denom, common.HexToAddress(m.Contract)); err != nil {
			panic(err)
		}
	}

	for _, m := range genState.AutoContracts {
		// Only allowed to bootstrap external token at genesis
		if !types.IsValidIBCDenom(m.Denom) && !types.IsValidGravityDenom(m.Denom) {
			panic(fmt.Sprintf("Invalid denom to map to contract: %s", m.Denom))
		}
		if !common.IsHexAddress(m.Contract) {
			panic(fmt.Sprintf("Invalid contract address: %s", m.Contract))
		}
		k.SetAutoContractForDenom(ctx, m.Denom, common.HexToAddress(m.Contract))
	}

	// this line is used by starport scaffolding # genesis/module/init

	// this line is used by starport scaffolding # ibc/genesis/init
}

// ExportGenesis returns the capability module's exported genesis.
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	// this line is used by starport scaffolding # genesis/module/export

	// this line is used by starport scaffolding # ibc/genesis/export

	return &types.GenesisState{
		Params:            k.GetParams(ctx),
		ExternalContracts: k.GetExternalContracts(ctx),
		AutoContracts:     k.GetAutoContracts(ctx),
	}
}

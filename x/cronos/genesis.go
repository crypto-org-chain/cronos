package cronos

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/x/cronos/keeper"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
)

// InitGenesis initializes the capability module's state from a provided genesis
// state.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState types.GenesisState) {
	k.SetParams(ctx, genState.Params)

	for _, m := range genState.ExternalContracts {
		k.SetExternalContractForDenom(ctx, m.Denom, common.HexToAddress(m.Contract))
	}

	for _, m := range genState.AutoContracts {
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

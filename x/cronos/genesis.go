package cronos

import (
	"errors"
	"fmt"

	"github.com/crypto-org-chain/cronos/x/cronos/keeper"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
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
		// Only allow IBC, gravity, or cronos denoms at genesis, and for a source
		// (cronos) denom, require the contract to match the address embedded in it.
		if err := types.ValidateTokenMapping(m.Denom, m.Contract); err != nil {
			panic(err)
		}
		if err := k.SetExternalContractForDenom(ctx, m.Denom, common.HexToAddress(m.Contract)); err != nil {
			panic(err)
		}
	}

	for _, m := range genState.AutoContracts {
		// Only allow IBC, gravity, or cronos denoms at genesis, and for a source
		// (cronos) denom, require the contract to match the address embedded in it.
		if err := types.ValidateTokenMapping(m.Denom, m.Contract); err != nil {
			panic(err)
		}
		if err := k.SetAutoContractForDenom(ctx, m.Denom, common.HexToAddress(m.Contract)); err != nil {
			if errors.Is(err, types.ErrExternalMappingExists) || errors.Is(err, types.ErrDenomAlreadyMapped) {
				k.Logger(ctx).Info("skipping auto contract import, denom mapping already exists",
					"denom", m.Denom, "contract", m.Contract, "error", err)
				continue
			}
			panic(err)
		}
	}

	// this line is used by starport scaffolding # genesis/module/init

	// this line is used by starport scaffolding # ibc/genesis/init
}

// ExportGenesis returns the capability module's exported genesis.
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	// this line is used by starport scaffolding # genesis/module/export

	// this line is used by starport scaffolding # ibc/genesis/export

	// Auto and external contracts are mutually exclusive for non-source denoms:
	// SetExternalContractForDenom retires any auto mapping for the same denom.
	return &types.GenesisState{
		Params:            k.GetParams(ctx),
		ExternalContracts: k.GetExternalContracts(ctx),
		AutoContracts:     k.GetAutoContracts(ctx),
	}
}

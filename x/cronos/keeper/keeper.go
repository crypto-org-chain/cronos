package keeper

import (
	"fmt"

	evmTypes "github.com/tharsis/ethermint/x/evm/types"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"

	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	// this line is used by starport scaffolding # ibc/keeper/import
)

type (
	Keeper struct {
		cdc      codec.Codec
		storeKey sdk.StoreKey
		memKey   sdk.StoreKey

		// module specific parameter space that can be configured through governance
		paramSpace paramtypes.Subspace
		// evm parameter space
		evmParamSpace paramtypes.Subspace
		// update balance and accounting operations with coins
		bankKeeper types.BankKeeper
		// ibc transfer operations
		transferKeeper types.TransferKeeper

		// this line is used by starport scaffolding # ibc/keeper/attribute
	}
)

func NewKeeper(
	cdc codec.Codec,
	storeKey,
	memKey sdk.StoreKey,
	paramSpace paramtypes.Subspace,
	evmSpace paramtypes.Subspace,
	bankKeeper types.BankKeeper,
	transferKeeper types.TransferKeeper,
	// this line is used by starport scaffolding # ibc/keeper/parameter
) *Keeper {

	// set KeyTable if it has not already been set
	if !paramSpace.HasKeyTable() {
		paramSpace = paramSpace.WithKeyTable(types.ParamKeyTable())
	}

	if !evmSpace.HasKeyTable() {
		evmSpace = evmSpace.WithKeyTable(evmTypes.ParamKeyTable())
	}

	return &Keeper{
		cdc:            cdc,
		storeKey:       storeKey,
		memKey:         memKey,
		paramSpace:     paramSpace,
		evmParamSpace:  evmSpace,
		bankKeeper:     bankKeeper,
		transferKeeper: transferKeeper,
		// this line is used by starport scaffolding # ibc/keeper/return
	}
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

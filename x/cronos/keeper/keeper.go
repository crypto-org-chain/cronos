package keeper

import (
	"fmt"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"

	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	evmkeeper "github.com/tharsis/ethermint/x/evm/keeper"
	// this line is used by starport scaffolding # ibc/keeper/import
)

type (
	Keeper struct {
		cdc      codec.Codec
		storeKey sdk.StoreKey
		memKey   sdk.StoreKey

		// module specific parameter space that can be configured through governance
		paramSpace paramtypes.Subspace
		// update balance and accounting operations with coins
		bankKeeper types.BankKeeper
		// ibc transfer operations
		transferKeeper types.TransferKeeper
		// ethermint evm keeper
		evmKeeper *evmkeeper.Keeper

		// this line is used by starport scaffolding # ibc/keeper/attribute
	}
)

func NewKeeper(
	cdc codec.Codec,
	storeKey,
	memKey sdk.StoreKey,
	paramSpace paramtypes.Subspace,
	bankKeeper types.BankKeeper,
	transferKeeper types.TransferKeeper,
	evmKeeper *evmkeeper.Keeper,
	// this line is used by starport scaffolding # ibc/keeper/parameter
) *Keeper {

	// set KeyTable if it has not already been set
	if !paramSpace.HasKeyTable() {
		paramSpace = paramSpace.WithKeyTable(types.ParamKeyTable())
	}

	return &Keeper{
		cdc:            cdc,
		storeKey:       storeKey,
		memKey:         memKey,
		paramSpace:     paramSpace,
		bankKeeper:     bankKeeper,
		transferKeeper: transferKeeper,
		evmKeeper:      evmKeeper,
		// this line is used by starport scaffolding # ibc/keeper/return
	}
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// getExternalContractByDenom find the corresponding external contract for the denom,
func (k Keeper) getExternalContractByDenom(ctx sdk.Context, denom string) (common.Address, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.DenomToExternalContractKey(denom))
	if len(bz) == 0 {
		return common.Address{}, false
	}

	return common.BytesToAddress(bz), true
}

// getAutoContractByDenom find the corresponding auto-deployed contract for the denom,
func (k Keeper) getAutoContractByDenom(ctx sdk.Context, denom string) (common.Address, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.DenomToAutoContractKey(denom))
	if len(bz) == 0 {
		return common.Address{}, false
	}

	return common.BytesToAddress(bz), true
}

// GetContractByDenom find the corresponding contract for the denom,
// external contract is taken in preference to auto-deployed one
func (k Keeper) GetContractByDenom(ctx sdk.Context, denom string) (contract common.Address, found bool) {
	contract, found = k.getExternalContractByDenom(ctx, denom)
	if !found {
		contract, found = k.getAutoContractByDenom(ctx, denom)
	}
	return
}

// GetDenomByContract find native denom by contract address
func (k Keeper) GetDenomByContract(ctx sdk.Context, contract common.Address) (denom string, found bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ContractToDenomKey(contract.Bytes()))
	if len(bz) == 0 {
		return "", false
	}

	return string(bz), true
}

// SetExternalContractForDenom set the external contract for native denom, replace the old one if any existing.
func (k Keeper) SetExternalContractForDenom(ctx sdk.Context, denom string, address common.Address) error {
	// check the contract is not registered already
	_, found := k.GetDenomByContract(ctx, address)
	if found {
		return fmt.Errorf("the contract is already registered: %s", address.Hex())
	}

	store := ctx.KVStore(k.storeKey)
	existing, found := k.getExternalContractByDenom(ctx, denom)
	if found {
		// remove existing mapping
		store.Delete(types.ContractToDenomKey(existing.Bytes()))
	}
	store.Set(types.DenomToExternalContractKey(denom), address.Bytes())
	store.Set(types.ContractToDenomKey(address.Bytes()), []byte(denom))
	return nil
}

// GetExternalContracts returns all external contract mappings
func (k Keeper) GetExternalContracts(ctx sdk.Context) (out []types.TokenMapping) {
	store := ctx.KVStore(k.storeKey)
	iter := prefix.NewStore(store, types.KeyPrefixDenomToExternalContract).Iterator(nil, nil)
	for ; iter.Valid(); iter.Next() {
		out = append(out, types.TokenMapping{
			Denom:    string(iter.Key()),
			Contract: common.BytesToAddress(iter.Value()).Hex(),
		})
	}
	return
}

// GetAutoContracts returns all auto-deployed contract mappings
func (k Keeper) GetAutoContracts(ctx sdk.Context) (out []types.TokenMapping) {
	store := ctx.KVStore(k.storeKey)
	iter := prefix.NewStore(store, types.KeyPrefixDenomToAutoContract).Iterator(nil, nil)
	for ; iter.Valid(); iter.Next() {
		out = append(out, types.TokenMapping{
			Denom:    string(iter.Key()),
			Contract: common.BytesToAddress(iter.Value()).Hex(),
		})
	}
	return
}

// DeleteExternalContractForDenom delete the external contract mapping for native denom,
// returns false if mapping not exists.
func (k Keeper) DeleteExternalContractForDenom(ctx sdk.Context, denom string) bool {
	store := ctx.KVStore(k.storeKey)
	contract, found := k.getExternalContractByDenom(ctx, denom)
	if !found {
		return false
	}
	store.Delete(types.DenomToExternalContractKey(denom))
	store.Delete(types.ContractToDenomKey(contract.Bytes()))
	return true
}

// SetAutoContractForDenom set the auto deployed contract for native denom
func (k Keeper) SetAutoContractForDenom(ctx sdk.Context, denom string, address common.Address) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.DenomToAutoContractKey(denom), address.Bytes())
	store.Set(types.ContractToDenomKey(address.Bytes()), []byte(denom))
}

package keeper

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

const (
	CanChangeTokenMapping int64 = 1 << iota
	CanTurnBridge
)

func (k Keeper) SetPermissions(ctx sdk.Context, address sdk.AccAddress, permissions []byte) {
	store := ctx.KVStore(k.storeKey)
	store.Set(types.AdminToPermissionsKey(address), permissions)
}

func (k Keeper) GetPermissions(ctx sdk.Context, address sdk.AccAddress) []byte {
	store := ctx.KVStore(k.storeKey)
	return store.Get(types.AdminToPermissionsKey(address))
}

// HasPermission check if an account has a specific permission. by default cronos admin has all permissions
func (k Keeper) HasPermission(ctx sdk.Context, account sdk.AccAddress, permissionsToCheck int64) bool {
	admin := k.GetParams(ctx).CronosAdmin
	permission := k.GetPermissions(ctx, account)
	permissionBigInt := new(big.Int).SetBytes(permission)
	permissionsToCheckBigInt := big.NewInt(permissionsToCheck)
	mask := permissionBigInt.Int64() & permissionsToCheckBigInt.Int64()

	return (admin == account.String()) || (mask == permissionsToCheckBigInt.Int64())
}

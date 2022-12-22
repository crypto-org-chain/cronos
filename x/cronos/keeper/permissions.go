package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

const (
	CanChangeTokenMapping uint64                                  = 1 << iota // 1
	CanTurnBridge                                                             // 2
	All                   = CanChangeTokenMapping | CanTurnBridge             // 3
)

func (k Keeper) SetPermissions(ctx sdk.Context, address sdk.AccAddress, permissions uint64) {
	store := ctx.KVStore(k.storeKey)
	permissionsBytes := sdk.Uint64ToBigEndian(permissions)
	store.Set(types.AdminToPermissionsKey(address), permissionsBytes)
}

func (k Keeper) GetPermissions(ctx sdk.Context, address sdk.AccAddress) uint64 {
	store := ctx.KVStore(k.storeKey)
	permissionsBytes := store.Get(types.AdminToPermissionsKey(address))
	return sdk.BigEndianToUint64(permissionsBytes)
}

// HasPermission check if an account has a specific permission. by default cronos admin has all permissions
func (k Keeper) HasPermission(ctx sdk.Context, account sdk.AccAddress, permissionsToCheck uint64) bool {
	admin := k.GetParams(ctx).CronosAdmin
	permission := k.GetPermissions(ctx, account)
	mask := permission & permissionsToCheck
	return (admin == account.String()) || (mask == permissionsToCheck)
}

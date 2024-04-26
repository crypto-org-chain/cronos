package keeper

import (
	"context"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/crypto-org-chain/cronos/v2/x/e2ee/types"
)

type Keeper struct {
	storeKey storetypes.StoreKey
}

var (
	_ types.MsgServer   = Keeper{}
	_ types.QueryServer = Keeper{}
)

func NewKeeper(storeKey storetypes.StoreKey) Keeper {
	return Keeper{
		storeKey: storeKey,
	}
}

func (k Keeper) RegisterEncryptionKey(
	ctx context.Context,
	req *types.MsgRegisterEncryptionKey,
) (*types.MsgRegisterEncryptionKeyResponse, error) {
	addr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, err
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.KVStore(k.storeKey).Set(types.KeyPrefix(addr), []byte(req.Key))
	return &types.MsgRegisterEncryptionKeyResponse{}, nil
}

func (k Keeper) InitGenesis(
	ctx context.Context,
	state *types.GenesisState,
) error {
	for _, key := range state.Keys {
		if _, err := k.RegisterEncryptionKey(ctx, &types.MsgRegisterEncryptionKey{
			Address: key.Address,
			Key:     key.Key,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	iter := prefix.NewStore(sdkCtx.KVStore(k.storeKey), types.KeyPrefixEncryptionKey).Iterator(nil, nil)
	defer iter.Close()

	var keys []types.EncryptionKeyEntry
	for ; iter.Valid(); iter.Next() {
		address := sdk.AccAddress(iter.Key()).String()
		key := iter.Value()
		keys = append(keys, types.EncryptionKeyEntry{
			Address: address,
			Key:     string(key),
		})
	}
	return &types.GenesisState{Keys: keys}, nil
}

func (k Keeper) Key(ctx context.Context, req *types.KeyRequest) (*types.KeyResponse, error) {
	addr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, err
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	value := sdkCtx.KVStore(k.storeKey).Get(types.KeyPrefix(addr))
	return &types.KeyResponse{Key: string(value)}, nil
}

func (k Keeper) Keys(ctx context.Context, requests *types.KeysRequest) (*types.KeysResponse, error) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	var rsp types.KeysResponse
	for _, address := range requests.Addresses {
		addr, err := sdk.AccAddressFromBech32(address)
		if err != nil {
			return nil, err
		}
		value := store.Get(types.KeyPrefix(addr))
		rsp.Keys = append(rsp.Keys, string(value))
	}

	return &rsp, nil
}

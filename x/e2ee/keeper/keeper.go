package keeper

import (
	"context"

	"cosmossdk.io/core/address"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/crypto-org-chain/cronos/v2/x/e2ee/types"
)

type Keeper struct {
	storeKey     storetypes.StoreKey
	addressCodec address.Codec
}

var (
	_ types.MsgServer   = Keeper{}
	_ types.QueryServer = Keeper{}
)

func NewKeeper(storeKey storetypes.StoreKey, addressCodec address.Codec) Keeper {
	return Keeper{
		storeKey:     storeKey,
		addressCodec: addressCodec,
	}
}

func (k Keeper) RegisterEncryptionKey(
	ctx context.Context,
	req *types.MsgRegisterEncryptionKey,
) (*types.MsgRegisterEncryptionKeyResponse, error) {
	bz, err := k.addressCodec.StringToBytes(req.Address)
	if err != nil {
		return nil, err
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.KVStore(k.storeKey).Set(types.KeyPrefix(bz), req.Key)
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
		address, err := k.addressCodec.BytesToString(iter.Key())
		if err != nil {
			return nil, err
		}
		key := iter.Value()
		keys = append(keys, types.EncryptionKeyEntry{
			Address: address,
			Key:     key,
		})
	}
	return &types.GenesisState{Keys: keys}, nil
}

func (k Keeper) Key(ctx context.Context, req *types.KeyRequest) (*types.KeyResponse, error) {
	bz, err := k.addressCodec.StringToBytes(req.Address)
	if err != nil {
		return nil, err
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	value := sdkCtx.KVStore(k.storeKey).Get(types.KeyPrefix(bz))
	return &types.KeyResponse{Key: value}, nil
}

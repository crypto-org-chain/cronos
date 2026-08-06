package keeper

import (
	"context"

	"github.com/crypto-org-chain/cronos/x/e2ee/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"cosmossdk.io/core/address"

	"github.com/cosmos/cosmos-sdk/store/v2/prefix"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

<<<<<<< HEAD
// MaxKeysAddresses caps request body under 1MB (10000* ~100B ≈ ~1MB)
=======
// 10K × ~100B ≈ ~1MB response
>>>>>>> 26d9e8df (add comment)
const MaxKeysAddresses = 10000

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

func (k Keeper) registerEncryptionKey(
	ctx context.Context,
	address string,
	key []byte,
) error {
	bz, err := k.addressCodec.StringToBytes(address)
	if err != nil {
		return err
	}
	sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey).Set(types.KeyPrefix(bz), key)
	return nil
}

func (k Keeper) RegisterEncryptionKey(
	ctx context.Context,
	req *types.MsgRegisterEncryptionKey,
) (*types.MsgRegisterEncryptionKeyResponse, error) {
	if err := k.registerEncryptionKey(ctx, req.Address, []byte(req.Key)); err != nil {
		return nil, err
	}
	return &types.MsgRegisterEncryptionKeyResponse{}, nil
}

func (k Keeper) InitGenesis(
	ctx context.Context,
	state *types.GenesisState,
) error {
	for _, key := range state.Keys {
		if err := k.registerEncryptionKey(ctx, key.Address, []byte(key.Key)); err != nil {
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
			Key:     string(key),
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
	return &types.KeyResponse{Key: string(value)}, nil
}

func (k Keeper) Keys(ctx context.Context, requests *types.KeysRequest) (*types.KeysResponse, error) {
	if len(requests.Addresses) > MaxKeysAddresses {
		return nil, status.Errorf(codes.InvalidArgument,
			"too many addresses in Keys request: %d (max %d)", len(requests.Addresses), MaxKeysAddresses)
	}

	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	var rsp types.KeysResponse
	for _, address := range requests.Addresses {
		bz, err := k.addressCodec.StringToBytes(address)
		if err != nil {
			return nil, err
		}
		value := store.Get(types.KeyPrefix(bz))
		rsp.Keys = append(rsp.Keys, string(value))
	}

	return &rsp, nil
}

package keeper_test

import (
	"fmt"
	"testing"

	"github.com/crypto-org-chain/cronos/x/e2ee/keeper"
	e2eetypes "github.com/crypto-org-chain/cronos/x/e2ee/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"cosmossdk.io/core/address"

	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const testBech32Prefix = "crc"

func setupKeeper(t *testing.T) (keeper.Keeper, sdk.Context, address.Codec) {
	t.Helper()

	key := storetypes.NewKVStoreKey(e2eetypes.StoreKey)
	ctx := testutil.DefaultContext(key, storetypes.NewTransientStoreKey("transient_test"))
	codec := addresscodec.NewBech32Codec(testBech32Prefix)
	return keeper.NewKeeper(key, codec), ctx, codec
}

func addresses(t *testing.T, codec address.Codec, n int) []string {
	t.Helper()

	out := make([]string, n)
	for i := range out {
		bz := make([]byte, 20)
		bz[0] = byte(i)
		bz[1] = byte(i >> 8)
		s, err := codec.BytesToString(bz)
		require.NoError(t, err)
		out[i] = s
	}
	return out
}

func TestKeysRejectsOversizedBatch(t *testing.T) {
	k, ctx, codec := setupKeeper(t)

	rsp, err := k.Keys(ctx, &e2eetypes.KeysRequest{
		Addresses: addresses(t, codec, keeper.MaxKeysAddresses+1),
	})

	require.Nil(t, rsp)
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Contains(t, err.Error(), fmt.Sprintf("max %d", keeper.MaxKeysAddresses))
}

func TestKeysAcceptsBatchAtLimit(t *testing.T) {
	k, ctx, codec := setupKeeper(t)

	rsp, err := k.Keys(ctx, &e2eetypes.KeysRequest{
		Addresses: addresses(t, codec, keeper.MaxKeysAddresses),
	})

	require.NoError(t, err)
	require.Len(t, rsp.Keys, keeper.MaxKeysAddresses)
}

func TestKeysReturnsRegisteredKeys(t *testing.T) {
	k, ctx, codec := setupKeeper(t)
	addrs := addresses(t, codec, 3)

	_, err := k.RegisterEncryptionKey(ctx, &e2eetypes.MsgRegisterEncryptionKey{
		Address: addrs[1],
		Key:     "key-1",
	})
	require.NoError(t, err)

	rsp, err := k.Keys(ctx, &e2eetypes.KeysRequest{Addresses: addrs})
	require.NoError(t, err)
	require.Equal(t, []string{"", "key-1", ""}, rsp.Keys)
}

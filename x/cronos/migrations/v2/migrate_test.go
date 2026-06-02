package v2_test

import (
	"testing"

	"github.com/crypto-org-chain/cronos/x/cronos/exported"
	v2 "github.com/crypto-org-chain/cronos/x/cronos/migrations/v2"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	evmenc "github.com/evmos/ethermint/encoding"
	"github.com/stretchr/testify/require"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type mockSubspace struct {
	ps types.Params
}

func newMockSubspace(ps types.Params) mockSubspace {
	return mockSubspace{ps: ps}
}

func (ms mockSubspace) GetParamSetIfExists(ctx sdk.Context, ps exported.ParamSet) {
	*ps.(*types.Params) = ms.ps
}

func TestMigrate(t *testing.T) {
	storeKey := storetypes.NewKVStoreKey(types.ModuleName)
	ctx := testutil.DefaultContext(storeKey, storetypes.NewTransientStoreKey("test"))
	store := ctx.KVStore(storeKey)
	cdc := evmenc.MakeConfig().Codec
	legacySubspace := newMockSubspace(types.DefaultParams())
	err := v2.Migrate(ctx, store, legacySubspace, cdc)
	require.NoError(t, err)
	var p types.Params
	require.NoError(t, cdc.Unmarshal(store.Get(types.ParamsKey), &p))
	require.Equal(t, legacySubspace.ps, p)
}

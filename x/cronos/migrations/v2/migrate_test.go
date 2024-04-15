package v2_test

import (
	"testing"

	simappparams "cosmossdk.io/simapp/params"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/exported"
	v2 "github.com/crypto-org-chain/cronos/v2/x/cronos/migrations/v2"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	"github.com/stretchr/testify/require"
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
	cdc := simappparams.MakeTestEncodingConfig().Codec
	legacySubspace := newMockSubspace(types.DefaultParams())
	v2.Migrate(ctx, store, legacySubspace, cdc)
	var p types.Params
	require.NoError(t, cdc.Unmarshal(store.Get(types.ParamsKey), &p))
	require.Equal(t, legacySubspace.ps, p)
}

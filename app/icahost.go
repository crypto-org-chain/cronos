package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/types"
)

// ICAHostMockSubspace is a mock implementation to workaround the migration process, because we have nothing to migrate from,
// otherwise it'll panic, see: https://github.com/cosmos/ibc-go/pull/6167
type ICAHostMockSubspace struct{}

var _ icatypes.ParamSubspace = ICAHostMockSubspace{}

func (ICAHostMockSubspace) GetParamSet(ctx sdk.Context, ps paramtypes.ParamSet) {
	*(ps.(*icahosttypes.Params)) = icahosttypes.DefaultParams()
}

package app

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	clientkeeper "github.com/cosmos/ibc-go/v8/modules/core/02-client/keeper"
	e2eetypes "github.com/crypto-org-chain/cronos/v2/x/e2ee/types"
)

func (app *App) RegisterUpgradeHandlers(cdc codec.BinaryCodec, clientKeeper clientkeeper.Keeper) {
	planName := "v1.3"
	app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		m, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		if err != nil {
			return m, err
		}

		sdkCtx := sdk.UnwrapSDKContext(ctx)
		{
			params := app.ICAHostKeeper.GetParams(sdkCtx)
			params.HostEnabled = false
			app.ICAHostKeeper.SetParams(sdkCtx, params)
		}
		return m, nil
	})

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Sprintf("failed to read upgrade info from disk %s", err))
	}
	if !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		if upgradeInfo.Name == planName {
			app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storetypes.StoreUpgrades{
				Added: []string{
					icahosttypes.StoreKey,
					e2eetypes.StoreKey,
				},
			}))
		}
	}
}

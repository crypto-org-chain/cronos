package app

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/module"
)

// RegisterUpgradeHandlers returns if store loader is overridden
func (app *App) RegisterUpgradeHandlers(cdc codec.BinaryCodec, maxVersion int64) bool {
	planName := "v1.5" // TBD
	app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		return app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
	})

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Sprintf("failed to read upgrade info from disk %s", err))
	}

	if !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		if upgradeInfo.Name == planName {
			app.SetStoreLoader(MaxVersionUpgradeStoreLoader(maxVersion, upgradeInfo.Height, &storetypes.StoreUpgrades{
				Deleted: []string{"capability", "feeibc"},
			}))
			return true
		}
	}

	return false
}

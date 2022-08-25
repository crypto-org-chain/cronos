package app

import (
	"fmt"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	icacontrollertypes "github.com/cosmos/ibc-go/v5/modules/apps/27-interchain-accounts/controller/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v5/modules/apps/29-fee/types"
)

func (app *App) RegisterUpgradeHandlers(experimental bool) {
	planName := "v0.9.0"
	app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx sdk.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		return app.mm.RunMigrations(ctx, app.configurator, fromVM)
	})

	gravityPlanName := "v0.8.0-gravity-alpha3"
	if experimental {
		app.UpgradeKeeper.SetUpgradeHandler(gravityPlanName, func(ctx sdk.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			return app.mm.RunMigrations(ctx, app.configurator, fromVM)
		})
	}

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Sprintf("failed to read upgrade info from disk %s", err))
	}

	if !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		if upgradeInfo.Name == planName {
			storeUpgrades := storetypes.StoreUpgrades{
				Added: []string{ibcfeetypes.StoreKey},
			}

			// configure store loader that checks if version == upgradeHeight and applies store upgrades
			app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
		}
		if experimental && upgradeInfo.Name == gravityPlanName {
			storeUpgrades := storetypes.StoreUpgrades{
				Added:   []string{ibcfeetypes.StoreKey},
				Deleted: []string{icacontrollertypes.StoreKey},
			}

			// configure store loader that checks if version == upgradeHeight and applies store upgrades
			app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
		}
	}
}

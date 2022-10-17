package app

import (
	"fmt"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

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

	gravityPlanName := "v0.8.0-gravity-alpha2"
	if experimental {
		app.UpgradeKeeper.SetUpgradeHandler(gravityPlanName, func(ctx sdk.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			updatedVM, err := app.mm.RunMigrations(ctx, app.configurator, fromVM)
			if err != nil {
				return nil, err
			}
			// set new gravity id
			gravParams := app.GravityKeeper.GetParams(ctx)
			gravParams.GravityId = "cronos_gravity_pioneer_v3"
			app.GravityKeeper.SetParams(ctx, gravParams)

			// Estimate time upgrade take place
			// 100% is not necessary here because it will be tuned by relayer later on
			// it is set to georli height at Wed Oct 26 2022 04:37:30 GMT+0900
			app.GravityKeeper.MigrateGravityContract(
				ctx, "0x0000000000000000000000000000000000000000", 7833000)

			// Fix bug on ethermint due to cutting the binary before official release
			evmParamStore := app.GetSubspace(evmtypes.ModuleName)
			evmParamStore.Set(ctx, evmtypes.ParamStoreKeyAllowUnprotectedTxs, false)

			return updatedVM, nil
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

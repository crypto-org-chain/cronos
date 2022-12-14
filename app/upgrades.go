package app

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	icacontrollertypes "github.com/cosmos/ibc-go/v5/modules/apps/27-interchain-accounts/controller/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v5/modules/apps/29-fee/types"
)

func (app *App) RegisterUpgradeHandlers(experimental bool) {
	upgradeHandlerV1 := func(ctx sdk.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		m, err := app.Mm.RunMigrations(ctx, app.Configurator, fromVM)
		if err != nil {
			return m, err
		}
		// clear extra_eips from evm parameters
		// Ref: https://github.com/crypto-org-chain/cronos/issues/755
		params := app.EvmKeeper.GetParams(ctx)
		params.ExtraEIPs = []int64{}

		// fix the incorrect value on testnet parameters
		zero := sdkmath.ZeroInt()
		params.ChainConfig.LondonBlock = &zero

		app.EvmKeeper.SetParams(ctx, params)
		return m, nil
	}
	// `v1.0.0` upgrade plan will clear the `extra_eips` parameters, and upgrade ibc-go to v5.1.
	planName := "v1.0.0"
	app.UpgradeKeeper.SetUpgradeHandler(planName, upgradeHandlerV1)
	// testnet3 should use `v1.0.0-testnet3` instead, it won't re-add the feeibc store, which is added in `v0.9.0` upgrade.
	planNameTestnet3 := "v1.0.0-testnet3"
	app.UpgradeKeeper.SetUpgradeHandler(planNameTestnet3, upgradeHandlerV1)

	gravityPlanName := "v0.8.0-gravity-alpha3"
	if experimental {
		app.UpgradeKeeper.SetUpgradeHandler(gravityPlanName, func(ctx sdk.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			return app.Mm.RunMigrations(ctx, app.Configurator, fromVM)
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

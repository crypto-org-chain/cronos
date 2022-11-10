package app

import (
	"fmt"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	icacontrollertypes "github.com/cosmos/ibc-go/v5/modules/apps/27-interchain-accounts/controller/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v5/modules/apps/29-fee/types"

	"github.com/crypto-org-chain/cronos/cmd/cronosd/config"
)

func (app *App) RegisterUpgradeHandlers(experimental bool) {
	// `v1.0.0` upgrade plan will clear the `extra_eips` parameters, and upgrade ibc-go to v5.1.
	// dry-run and mainnet will upgrade to `v1.0.0` directly, skipping the `v0.9.0`.
	// testnet can also upgrade to `v1.0.0` after `v0.9.0`.
	planName := "v1.0.0"
	app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx sdk.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		m, err := app.mm.RunMigrations(ctx, app.configurator, fromVM)
		if err != nil {
			return m, err
		}
		// clear extra_eips from evm parameters
		// Ref: https://github.com/crypto-org-chain/cronos/issues/755
		params := app.EvmKeeper.GetParams(ctx)
		params.ExtraEIPs = []int64{}
		app.EvmKeeper.SetParams(ctx, params)
		return m, nil
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
		// testnet has added the ibcfee store in `v0.9.0`, skip this time.
		if upgradeInfo.Name == planName && config.CurrentNetwork() != config.NetworkTestnet {
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

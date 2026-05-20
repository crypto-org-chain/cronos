package app

import (
	"context"

	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

// v6UpgradeName is the name of the upgrade plan that migrates Cronos to
// Cosmos SDK v0.54 (store/v2, log/v2, IBC v11, in-tree x/* modules).
const v6UpgradeName = "v6.0.0-sdk54"

// RegisterUpgradeHandlers returns if store loader is overridden
func (app *App) RegisterUpgradeHandlers(cdc codec.BinaryCodec, maxVersion int64) bool {
	planName := "v1.8"
	app.UpgradeKeeper.SetUpgradeHandler(planName,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			return app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		},
	)

	app.UpgradeKeeper.SetUpgradeHandler(v6UpgradeName,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			// Cosmos SDK v0.54 RunMigrations runs:
			// - staking v6 migration (pending-slot index backfill from PR #26023)
			// - rehoming of evidence/feegrant/upgrade modules into in-tree paths
			// - gov vote-results calculator wiring
			return app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		},
	)

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		return false
	}
	if upgradeInfo.Name == v6UpgradeName && !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storetypes.StoreUpgrades{}))
		return true
	}

	return false
}

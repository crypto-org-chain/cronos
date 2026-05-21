package app

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

// RegisterUpgradeHandlers returns if store loader is overridden
func (app *App) RegisterUpgradeHandlers(cdc codec.BinaryCodec, maxVersion int64) bool {
	planName := "v1.8"
	app.UpgradeKeeper.SetUpgradeHandler(planName,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			toVM, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
			if err != nil {
				return toVM, err
			}
			// Populate staking queue pending-slot indexes after migrations so the
			// indexes are built on fully-migrated queue keys (cosmos-sdk PR #26023
			// optimization, exposed as opt-in utility per crypto-org-chain
			// cosmos-sdk PR #1814 instead of an auto-migration).
			if err := app.StakingKeeper.PopulateQueuePendingSlots(ctx); err != nil {
				return toVM, fmt.Errorf("populate queue pending slots: %w", err)
			}
			return toVM, nil
		},
	)

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		// Missing file returns (Plan{}, nil); a real error here means the file
		// exists but is unreadable or malformed. At an upgrade height this
		// would silently skip StoreUpgrades wiring, so fail fast.
		panic(err)
	}
	if upgradeInfo.Name == planName && !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		// No store-key churn from v0.53→v0.54 in this app, but follow the
		// cosmos-sdk recommended pattern (UpgradeStoreLoader pins the load to
		// upgradeHeight and is the canonical loader for upgrade boots).
		// See https://docs.cosmos.network/sdk/latest/upgrade/upgrade
		storeUpgrades := storetypes.StoreUpgrades{
			Added: []string{},
		}
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
		return true
	}

	return false
}

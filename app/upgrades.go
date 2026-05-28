package app

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

// RegisterUpgradeHandlers returns if store loader is overridden.
// No store-key churn from v0.53→v0.54 in this app, so the default
// MaxVersionStoreLoader (set by the caller when this returns false)
// covers both regular and upgrade-height boots.
func (app *App) RegisterUpgradeHandlers(cdc codec.BinaryCodec, maxVersion int64) bool {
	app.UpgradeKeeper.SetUpgradeHandler("v1.8",
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			toVM, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
			if err != nil {
				return toVM, err
			}
			// Populate staking queue pending-slot indexes after migrations so the
			// indexes are built on fully-migrated queue keys (cosmos-sdk PR #26023
			// optimization, exposed as opt-in utility per crypto-org-chain
			// cosmos-sdk PR #1814 instead of an auto-migration). The keeper
			// implementation overwrites the per-time slot via Set, so re-running
			// at the same height is idempotent.
			if err := app.StakingKeeper.PopulateQueuePendingSlots(ctx); err != nil {
				return toVM, fmt.Errorf("populate queue pending slots: %w", err)
			}
			return toVM, nil
		},
	)
	return false
}

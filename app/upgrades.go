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
			// Populate staking queue pending-slot indexes (cosmos-sdk PR #26023
			// optimization, exposed as opt-in utility per crypto-org-chain
			// cosmos-sdk PR #1814 instead of an auto-migration).
			if err := app.StakingKeeper.PopulateQueuePendingSlots(ctx); err != nil {
				return fromVM, fmt.Errorf("populate queue pending slots: %w", err)
			}
			return app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		},
	)
	return false
}

package app

import (
	"context"

	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/module"
)

// RegisterUpgradeHandlers returns if store loader is overridden
func (app *App) RegisterUpgradeHandlers(cdc codec.BinaryCodec, maxVersion int64) bool {
	planName := "v1.8"
	app.UpgradeKeeper.SetUpgradeHandler(planName,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			// Activate the CRC21 precompile blocks at upgrade height. Shared-map
			// reference means subsequent BankKeeper.SendCoins calls see the new
			// entries without keeper reinitialization.
			app.ActivateCRC21PrecompileBlocks()
			return app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		},
	)

	return false
}

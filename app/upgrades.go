package app

import (
	"context"

	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/module"
)

func (app *App) RegisterUpgradeHandlers(cdc codec.BinaryCodec) {
	planName := "v1.5" // TBD
	app.UpgradeKeeper.SetUpgradeHandler(planName, func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		return app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
	})
}

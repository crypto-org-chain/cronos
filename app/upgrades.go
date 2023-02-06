package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
)

func (app *App) RegisterUpgradeHandlers() {
	upgradeHandlerV2 := func(ctx sdk.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		m, err := app.mm.RunMigrations(ctx, app.configurator, fromVM)
		if err != nil {
			return m, err
		}

		gravParams := app.GravityKeeper.GetParams(ctx)
		gravParams.GravityId = "cronos_gravity_testnet"
		// can be activated later on
		gravParams.BridgeActive = false
		app.GravityKeeper.SetParams(ctx, gravParams)
		return m, nil
	}
	// `v1.0.0` upgrade plan will clear the `extra_eips` parameters, and upgrade ibc-go to v5.2.0.
	planName := "v2.0.0-testnet3"
	app.UpgradeKeeper.SetUpgradeHandler(planName, upgradeHandlerV2)
}

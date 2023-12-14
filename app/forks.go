package app

import sdk "github.com/cosmos/cosmos-sdk/types"

// Fork defines a struct containing the requisite fields for a non-software upgrade proposal
// Hard Fork at a given height to implement.
// There is one time code that can be added for the start of the Fork, in `BeginForkLogic`.
// Any other change in the code should be height-gated, if the goal is to have old and new binaries
// to be compatible prior to the upgrade height.
//
// Adapted from osmosis: https://github.com/osmosis-labs/osmosis/blob/057192c2c0949fde5673a5f314bf41816f808fd9/app/upgrades/types.go#L40
type Fork struct {
	// Upgrade version name, for the upgrade handler, e.g. `v7`
	UpgradeName string
	// height the upgrade occurs at
	UpgradeHeight int64
	// chain-id the upgrade occurs at
	UpgradeChainId string

	// Function that runs some custom state transition code at the beginning of a fork.
	BeginForkLogic func(ctx sdk.Context, app *App)
}

// BeginBlockForks is intended to be ran in a chain upgrade.
func BeginBlockForks(ctx sdk.Context, app *App) {
	for _, fork := range Forks {
		if ctx.BlockHeight() == fork.UpgradeHeight && ctx.ChainID() == fork.UpgradeChainId {
			fork.BeginForkLogic(ctx, app)
			return
		}
	}
}

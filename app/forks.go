package app

import sdk "github.com/cosmos/cosmos-sdk/types"

var Forks = []Fork{
	ForkV1Mainnnet,
	ForkV1Dryrun,
}

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

func ForkV1Logic(ctx sdk.Context, app *App) {
	params := app.FeeMarketKeeper.GetParams(ctx)
	params.BaseFeeChangeDenominator = 300
	params.ElasticityMultiplier = 4
	params.BaseFee = sdk.NewInt(10000000000000)
	params.MinGasPrice = sdk.NewDec(10000000000000)
	app.FeeMarketKeeper.SetParams(ctx, params)
}

var (
	ForkV1Mainnnet = Fork{
		UpgradeName:    "v1.0.x-base-fee",
		UpgradeHeight:  11608760,
		UpgradeChainId: "cronosmainnet_25-1",
		BeginForkLogic: ForkV1Logic,
	}
	ForkV1Dryrun = Fork{
		UpgradeName:    "v1.0.x-base-fee",
		UpgradeHeight:  5215165,
		UpgradeChainId: "tempcronosmainnet_28-1",
		BeginForkLogic: ForkV1Logic,
	}
)

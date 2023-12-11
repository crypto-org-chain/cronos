package ante

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmante "github.com/evmos/ethermint/app/ante"
)

// NewAnteHandler add additional logic on top of Ethermint's anteHandler
func NewAnteHandler(options HandlerOptions) (sdk.AnteHandler, error) {
	return func(
		ctx sdk.Context, tx sdk.Tx, sim bool,
	) (newCtx sdk.Context, err error) {
		var anteHandler sdk.AnteHandler

		defer evmante.Recover(ctx.Logger(), &err)

		anteHandler, err = evmante.NewAnteHandler(options.EvmOptions)
		if err != nil {
			panic(err)
		}
		return anteHandler(ctx, tx, sim)
	}, nil
}

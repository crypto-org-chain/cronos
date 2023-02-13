package ante

import (
	evmante "github.com/evmos/ethermint/app/ante"
)

// HandlerOptions extend the ethermint's AnteHandler options by adding extra keeper necessary for
// custom ante handler logics
type HandlerOptions struct {
	EvmOptions   evmante.HandlerOptions
	CronosKeeper CronosKeeper
}

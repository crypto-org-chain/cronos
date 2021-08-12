package app

import (
	"github.com/cosmos/cosmos-sdk/simapp/params"
	evmenc "github.com/tharsis/ethermint/encoding"
)

// MakeEncodingConfig creates the EncodingConfig for cronos chain
func MakeEncodingConfig() params.EncodingConfig {
	return evmenc.MakeConfig(ModuleBasics)
}

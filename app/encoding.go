package app

import (
	"cosmossdk.io/simapp/params"
	evmenc "github.com/evmos/ethermint/encoding"
)

// MakeEncodingConfig creates the EncodingConfig for cronos chain
func MakeEncodingConfig() params.EncodingConfig {
	return evmenc.MakeConfig(ModuleBasics)
}

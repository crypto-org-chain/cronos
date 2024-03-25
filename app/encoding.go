package app

import (
	evmenc "github.com/evmos/ethermint/encoding"
	ethermint "github.com/evmos/ethermint/types"
)

// MakeEncodingConfig creates the EncodingConfig for cronos chain
func MakeEncodingConfig() ethermint.EncodingConfig {
	return evmenc.MakeConfig(ModuleBasics)
}

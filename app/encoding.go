package app

import (
	"github.com/cosmos/cosmos-sdk/simapp/params"
	evmenc "github.com/evmos/ethermint/encoding"
)

// MakeEncodingConfig creates the EncodingConfig for cronos chain
func MakeEncodingConfig() params.EncodingConfig {
	return evmenc.MakeConfig(ModuleBasics)
}

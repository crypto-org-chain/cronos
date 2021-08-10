package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	cmdcfg "github.com/tharsis/ethermint/cmd/config"
)

func SetConfig() {
	config := sdk.GetConfig()
	// use the configurations from ethermint
	cmdcfg.SetBech32Prefixes(config)
	cmdcfg.SetBip44CoinType(config)
	config.Seal()
}

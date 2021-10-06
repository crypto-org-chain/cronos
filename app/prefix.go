package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	cmdcfg "github.com/crypto-org-chain/cronos/cmd/cronosd/config"
)

func SetConfig() {
	config := sdk.GetConfig()
	// use the configurations from ethermint
	cmdcfg.SetBech32Prefixes(config)
	cmdcfg.SetBip44CoinType(config)
	// Make sure address is compatible with ethereum
	config.SetAddressVerifier(VerifyAddressFormat)
	config.Seal()
}

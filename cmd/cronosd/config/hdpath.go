// +build !ledger

package config

import ethermint "github.com/tharsis/ethermint/types"

var (
	BIP44CoinType uint32 = ethermint.Bip44CoinType

	BIP44HDPath = ethermint.BIP44HDPath
)

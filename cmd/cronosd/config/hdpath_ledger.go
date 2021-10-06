// +build ledger

package config

import (
    ethaccounts "github.com/ethereum/go-ethereum/accounts"
)

var (
    BIP44CoinType uint32 = 394

    BIP44HDPath = ethaccounts.DerivationPath{0x80000000 + 44, 0x80000000 + 394, 0x80000000 + 0, 0, 0}.String()
)
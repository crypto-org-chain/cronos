package types

import (
	"math/big"
	"strings"
)

var (
	Ten       = big.NewInt(10)
	TenPowTen = Ten.Exp(Ten, Ten, nil)
)

// IsValidDenomToWrap returns if it's ok to wrap the native denom in erc20
// Currently only supports ibc/{hash} and gravity{contract}
func IsValidDenomToWrap(denom string) bool {
	return strings.HasPrefix(denom, "ibc/") || strings.HasPrefix(denom, "gravity0x")
}

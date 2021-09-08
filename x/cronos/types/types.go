package types

import (
	"math/big"
	"strings"
)

var (
	Ten       = big.NewInt(10)
	TenPowTen = Ten.Exp(Ten, Ten, nil)
)

const (
	ibcDenomPrefix     = "ibc/"
	ibcDenomLen        = len(ibcDenomPrefix) + 64
	gravityDenomPrefix = "gravity0x"
	gravityDenomLen    = len(gravityDenomPrefix) + 40
)

// IsValidIBCDenom returns if denom is a valid ibc denom
func IsValidIBCDenom(denom string) bool {
	return len(denom) == ibcDenomLen && strings.HasPrefix(denom, ibcDenomPrefix)
}

// IsValidGravityDenom returns if denom is a valid gravity denom
func IsValidGravityDenom(denom string) bool {
	return len(denom) == gravityDenomLen && strings.HasPrefix(denom, gravityDenomPrefix)
}

// IsValidDenomToWrap returns if it's ok to wrap the native denom in erc20
// Currently only supports ibc and gravity denom
func IsValidDenomToWrap(denom string) bool {
	return IsValidIBCDenom(denom) || IsValidGravityDenom(denom)
}

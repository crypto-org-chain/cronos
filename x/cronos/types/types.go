package types

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
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
	cronosDenomPrefix  = "cronos0x"
	cronosDenomLen     = len(cronosDenomPrefix) + 40
)

// IsValidIBCDenom returns true if denom is a valid ibc denom
func IsValidIBCDenom(denom string) bool {
	return len(denom) == ibcDenomLen && strings.HasPrefix(denom, ibcDenomPrefix)
}

// IsValidGravityDenom returns true if denom is a valid gravity denom
func IsValidGravityDenom(denom string) bool {
	return len(denom) == gravityDenomLen && strings.HasPrefix(denom, gravityDenomPrefix)
}

// IsValidCronosDenom returns true if denom is a valid cronos denom
func IsValidCronosDenom(denom string) bool {
	return len(denom) == cronosDenomLen && strings.HasPrefix(denom, cronosDenomPrefix)
}

// IsSourceCoin returns true if denom is a coin originated from cronos
func IsSourceCoin(denom string) bool {
	return IsValidCronosDenom(denom)
}

// IsValidCoinDenom returns true if it's ok it is a valid coin denom
func IsValidCoinDenom(denom string) bool {
	return IsValidIBCDenom(denom) || IsValidGravityDenom(denom) || IsValidCronosDenom(denom)
}

// GetContractAddressFromDenom get the contract address from the coin denom
func GetContractAddressFromDenom(denom string) (string, error) {
	contractAddress := ""
	if strings.HasPrefix(denom, gravityDenomPrefix) {
		contractAddress = denom[7:]
	} else if strings.HasPrefix(denom, cronosDenomPrefix) {
		contractAddress = denom[6:]
	}
	if !common.IsHexAddress(contractAddress) {
		return "", fmt.Errorf("invalid contract address (%s)", contractAddress)
	}
	return contractAddress, nil
}

// ValidateTokenMapping performs the stateless checks a denom/contract token mapping
// must satisfy: the denom must be a valid IBC, gravity, or cronos denom; the contract
// must be a hex address; and for a source (cronos) denom, the contract must equal the
// address embedded in the denom itself.
func ValidateTokenMapping(denom, contract string) error {
	if !IsValidCoinDenom(denom) {
		return fmt.Errorf("invalid denom to map to contract: %s", denom)
	}
	if !common.IsHexAddress(contract) {
		return fmt.Errorf("invalid contract address: %s", contract)
	}
	if IsSourceCoin(denom) {
		contractFromDenom, err := GetContractAddressFromDenom(denom)
		if err != nil {
			return err
		}
		if !strings.EqualFold(contractFromDenom, contract) {
			return fmt.Errorf(
				"the contract address for source denom %s is %s, mismatch with requested contract %s",
				denom, common.HexToAddress(contractFromDenom).Hex(), common.HexToAddress(contract).Hex(),
			)
		}
	}
	return nil
}

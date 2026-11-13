package types

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	yaml "gopkg.in/yaml.v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	IbcCroDenomDefaultValue    = "ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865"
	IbcTimeoutDefaultValue     = uint64(86400000000000) // 1 day
	MaxCallbackGasDefaultValue = uint64(50000)
	MaxIbcTimeoutValue         = uint64(30 * 24 * time.Hour) // 30 days
)

// NewParams creates a new parameter configuration for the cronos module
func NewParams(ibcCroDenom string, ibcTimeout uint64, cronosAdmin string, enableAutoDeployment bool, maxCallbackGas uint64, croBridgeContractAddresses []string) Params {
	return Params{
		IbcCroDenom:                ibcCroDenom,
		IbcTimeout:                 ibcTimeout,
		CronosAdmin:                cronosAdmin,
		EnableAutoDeployment:       enableAutoDeployment,
		MaxCallbackGas:             maxCallbackGas,
		CroBridgeContractAddresses: croBridgeContractAddresses,
	}
}

// DefaultParams is the default parameter configuration for the cronos module
func DefaultParams() Params {
	return Params{
		IbcCroDenom:          IbcCroDenomDefaultValue,
		IbcTimeout:           IbcTimeoutDefaultValue,
		CronosAdmin:          "",
		EnableAutoDeployment: false,
		MaxCallbackGas:       MaxCallbackGasDefaultValue,
	}
}

// Validate all cronos module parameters
func (p Params) Validate() error {
	if err := validateIbcTimeout(p.IbcTimeout); err != nil {
		return err
	}
	if err := validateIsIbcDenom(p.IbcCroDenom); err != nil {
		return err
	}
	if len(p.CronosAdmin) > 0 {
		if _, err := sdk.AccAddressFromBech32(p.CronosAdmin); err != nil {
			return err
		}
	}
	if err := validateIsEvmAddresses(p.CroBridgeContractAddresses); err != nil {
		return err
	}
	return nil
}

// String implements the fmt.Stringer interface
func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}

func validateIsIbcDenom(denom string) error {
	if !IsValidIBCDenom(denom) {
		return fmt.Errorf("invalid ibc denom: %s", denom)
	}
	return nil
}

func validateIbcTimeout(timeout uint64) error {
	if timeout > MaxIbcTimeoutValue {
		return fmt.Errorf("invalid ibc timeout %d: must not exceed %d", timeout, MaxIbcTimeoutValue)
	}
	return nil
}

func validateIsEvmAddresses(addrs []string) error {
	for _, addr := range addrs {
		if !common.IsHexAddress(addr) || (common.HexToAddress(addr) == common.Address{}) {
			return fmt.Errorf("invalid evm address: %s: must be a non-zero EVM hex address", addr)
		}
	}
	return nil
}

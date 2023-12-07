package types

import (
	"fmt"

	yaml "gopkg.in/yaml.v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

var (
	// KeyIbcCroDenom is store's key for the IBC Cro denomination
	KeyIbcCroDenom = []byte("IbcCroDenom")
	// KeyIbcTimeout is store's key for the IBC Timeout
	KeyIbcTimeout = []byte("IbcTimeout")
	// KeyCronosAdmin is store's key for the admin address
	KeyCronosAdmin = []byte("CronosAdmin")
	// KeyEnableAutoDeployment is store's key for the EnableAutoDeployment
	KeyEnableAutoDeployment = []byte("EnableAutoDeployment")
	// KeyMaxCallbackGas is store's key for the MaxCallbackGas
	KeyMaxCallbackGas = []byte("MaxCallbackGas")
)

const (
	IbcCroDenomDefaultValue    = "ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865"
	IbcTimeoutDefaultValue     = uint64(86400000000000) // 1 day
	MaxCallbackGasDefaultValue = uint64(50000)
)

// ParamKeyTable returns the parameter key table.
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// NewParams creates a new parameter configuration for the cronos module
func NewParams(ibcCroDenom string, ibcTimeout uint64, cronosAdmin string, enableAutoDeployment bool, maxCallbackGas uint64) Params {
	return Params{
		IbcCroDenom:          ibcCroDenom,
		IbcTimeout:           ibcTimeout,
		CronosAdmin:          cronosAdmin,
		EnableAutoDeployment: enableAutoDeployment,
		MaxCallbackGas:       maxCallbackGas,
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
	if err := validateIsUint64(p.IbcTimeout); err != nil {
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
	if err := validateIsUint64(p.MaxCallbackGas); err != nil {
		return err
	}
	return nil
}

// String implements the fmt.Stringer interface
func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}

// ParamSetPairs implements params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeyIbcCroDenom, &p.IbcCroDenom, validateIsIbcDenom),
		paramtypes.NewParamSetPair(KeyIbcTimeout, &p.IbcTimeout, validateIsUint64),
		paramtypes.NewParamSetPair(KeyCronosAdmin, &p.CronosAdmin, validateIsAddress),
		paramtypes.NewParamSetPair(KeyEnableAutoDeployment, &p.EnableAutoDeployment, validateIsBool),
		paramtypes.NewParamSetPair(KeyMaxCallbackGas, &p.MaxCallbackGas, validateIsUint64),
	}
}

func validateIsIbcDenom(i interface{}) error {
	s, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if !IsValidIBCDenom(s) {
		return fmt.Errorf("invalid ibc denom: %T", i)
	}
	return nil
}

func validateIsUint64(i interface{}) error {
	if _, ok := i.(uint64); !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateIsAddress(i interface{}) error {
	s, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if len(s) > 0 {
		if _, err := sdk.AccAddressFromBech32(s); err != nil {
			return err
		}
	}
	return nil
}

func validateIsBool(i interface{}) error {
	_, ok := i.(bool)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

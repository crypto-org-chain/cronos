package types

import (
	"fmt"

	yaml "gopkg.in/yaml.v2"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

var (
	// KeyIbcCroDenom is store's key for the IBC Cro denomination
	KeyIbcCroDenom = []byte("IbcCroDenom")
	// KeyIbcTimeout is store's key for the IBC Timeout
	KeyIbcTimeout = []byte("KeyIbcTimeout")
)

const IbcCroDenomDefaultValue = "ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865"
const IbcTimeoutDefaultValue = uint64(86400000000000) // 1 day

// ParamKeyTable returns the parameter key table.
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// NewParams creates a new parameter configuration for the cronos module
func NewParams(ibcCroDenom string, ibcTimeout uint64) Params {
	return Params{
		IbcCroDenom: ibcCroDenom,
		IbcTimeout:  ibcTimeout,
	}
}

// DefaultParams is the default parameter configuration for the cronos module
func DefaultParams() Params {
	return Params{
		IbcCroDenom: IbcCroDenomDefaultValue,
		IbcTimeout:  IbcTimeoutDefaultValue,
	}
}

// Validate all cronos module parameters
func (p Params) Validate() error {
	if err := validateIsUint64(p.IbcTimeout); err != nil {
		return err
	}
	return validateIsIbcDenom(p.IbcCroDenom)
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

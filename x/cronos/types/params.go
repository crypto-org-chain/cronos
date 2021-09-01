package types

import (
	"fmt"

	yaml "gopkg.in/yaml.v2"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

var (
	// KeyIbcCroDenom is store's key for the IBC Cro denomination
	KeyIbcCroDenom = []byte("IbcCroDenom")
)

// ParamKeyTable returns the parameter key table.
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// NewParams creates a new parameter configuration for the cronos module
func NewParams(ibcCroDenom string) Params {
	return Params{
		IbcCroDenom: ibcCroDenom,
	}
}

// DefaultParams is the default parameter configuration for the cronos module
func DefaultParams() Params {
	return Params{
		IbcCroDenom: "ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865",
	}
}

// Validate all cronos module parameters
func (p Params) Validate() error {
	return validateIsString(p.IbcCroDenom)
}

// String implements the fmt.Stringer interface
func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}

// ParamSetPairs implements params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeyIbcCroDenom, &p.IbcCroDenom, validateIsString),
	}
}

func validateIsString(i interface{}) error {
	_, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

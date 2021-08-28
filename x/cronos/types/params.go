package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	yaml "gopkg.in/yaml.v2"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
)

var (
	// KeyConvertEnabled is store's key for ConvertEnabled Params
	KeyConvertEnabled = []byte("ConvertEnabled")
	// KeyIbcCroDenom is store's key for the IBC Cro denomination
	KeyIbcCroDenom = []byte("IbcCroDenom")
)

// ParamKeyTable returns the parameter key table.
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// NewParams creates a new parameter configuration for the cronos module
func NewParams(convertEnabledParams ConvertEnabledParams, ibcCroDenom string) Params {
	return Params{
		ConvertEnabled: convertEnabledParams,
		IbcCroDenom:    ibcCroDenom,
	}
}

// DefaultParams is the default parameter configuration for the cronos module
func DefaultParams() Params {
	return Params{
		ConvertEnabled: ConvertEnabledParams{},
		IbcCroDenom:    "",
	}
}

// Validate all bank module parameters
func (p Params) Validate() error {
	if err := validateConvertEnabledParams(p.ConvertEnabled); err != nil {
		return err
	}
	return validateIsString(p.IbcCroDenom)
}

// String implements the fmt.Stringer interface
func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}

// ConvertEnabledDenom returns true if the given denom is enabled for converting
func (p Params) ConvertEnabledDenom(denom string) bool {
	for _, pse := range p.ConvertEnabled {
		if pse.Denom == denom {
			return pse.Enabled
		}
	}
	return false
}

// SetConvertEnabledParam returns an updated set of Parameters with the given denom
// convert enabled flag set.
func (p Params) SetConvertEnabledParam(denom string, convertEnabled bool) Params {
	var convertParams ConvertEnabledParams
	for _, p := range p.ConvertEnabled {
		if p.Denom != denom {
			convertParams = append(convertParams, NewConvertEnabled(p.Denom, p.Enabled))
		}
	}
	convertParams = append(convertParams, NewConvertEnabled(denom, convertEnabled))
	return NewParams(convertParams, p.IbcCroDenom)
}

// ParamSetPairs implements params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeyConvertEnabled, &p.ConvertEnabled, validateConvertEnabledParams),
		paramtypes.NewParamSetPair(KeyIbcCroDenom, &p.IbcCroDenom, validateIsString),
	}
}

// ConvertEnabledParams is a collection of parameters indicating if a coin denom is enabled for converting
type ConvertEnabledParams []*ConvertEnabled

func validateConvertEnabledParams(i interface{}) error {
	params, ok := i.([]*ConvertEnabled)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	// ensure each denom is only registered one time.
	registered := make(map[string]bool)
	for _, p := range params {
		if _, exists := registered[p.Denom]; exists {
			return fmt.Errorf("duplicate send enabled parameter found: '%s'", p.Denom)
		}
		if err := validateConvertEnabled(*p); err != nil {
			return err
		}
		registered[p.Denom] = true
	}
	return nil
}

// NewConvertEnabled creates a new ConvertEnabled object
func NewConvertEnabled(denom string, convertEnabled bool) *ConvertEnabled {
	return &ConvertEnabled{
		Denom:   denom,
		Enabled: convertEnabled,
	}
}

// String implements stringer interface
func (se ConvertEnabled) String() string {
	out, _ := yaml.Marshal(se)
	return string(out)
}

func validateConvertEnabled(i interface{}) error {
	param, ok := i.(ConvertEnabled)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return sdk.ValidateDenom(param.Denom)
}

func validateIsString(i interface{}) error {
	_, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

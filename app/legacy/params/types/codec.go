// Package types keeps x/params' ParameterChangeProposal so pre-v1.9 gov proposals still decode.
// params.pb.go is cosmos-sdk v0.54's x/params/types/proposal/params.pb.go with only the package renamed.
package types

import (
	"errors"

	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

// ErrRemoved rejects new params proposals: x/params was removed in v1.9 and the type is kept only to decode history.
var ErrRemoved = errors.New("x/params has been removed; ParameterChangeProposal is decode-only")

var _ govv1beta1.Content = (*ParameterChangeProposal)(nil)

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*govv1beta1.Content)(nil), &ParameterChangeProposal{})
}

func (pcp *ParameterChangeProposal) GetTitle() string { return pcp.Title }

func (pcp *ParameterChangeProposal) GetDescription() string { return pcp.Description }

func (pcp *ParameterChangeProposal) ProposalRoute() string { return "params" }

func (pcp *ParameterChangeProposal) ProposalType() string { return "ParameterChange" }

func (pcp *ParameterChangeProposal) ValidateBasic() error { return ErrRemoved }

package types

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

const (
	// ProposalTypeTokenMappingChange defines the type for a TokenMappingChangeProposal
	ProposalTypeTokenMappingChange = "TokenMappingChange"
)

// Assert TokenMappingChangeProposal implements govtypes.Content at compile-time
var _ govtypes.Content = &TokenMappingChangeProposal{}

func init() {
	govtypes.RegisterProposalType(ProposalTypeTokenMappingChange)
}

func NewTokenMappingChangeProposal(title, description, denom, symbol string, decimal uint32, contractAddr *common.Address) *TokenMappingChangeProposal {
	contract := ""
	if contractAddr != nil {
		contract = contractAddr.Hex()
	}
	return &TokenMappingChangeProposal{title, description, denom, contract, symbol, decimal}
}

// GetTitle returns the title of a parameter change proposal.
func (tcp *TokenMappingChangeProposal) GetTitle() string { return tcp.Title }

// GetDescription returns the description of a parameter change proposal.
func (tcp *TokenMappingChangeProposal) GetDescription() string { return tcp.Description }

// ProposalRoute returns the routing key of a parameter change proposal.
func (tcp *TokenMappingChangeProposal) ProposalRoute() string { return RouterKey }

// ProposalType returns the type of a parameter change proposal.
func (tcp *TokenMappingChangeProposal) ProposalType() string { return ProposalTypeTokenMappingChange }

// ValidateBasic validates the parameter change proposal
func (tcp *TokenMappingChangeProposal) ValidateBasic() error {
	// TODO
	return nil
}

// String implements the Stringer interface.
func (tcp TokenMappingChangeProposal) String() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf(`Token Mapping Change Proposal:
  Title:       %s
  Description: %s
  Denom:       %s
  Contract:    %s
  Symbol:      %s
  Decimal:     %d
`, tcp.Title, tcp.Description, tcp.Denom, tcp.Contract, tcp.Symbol, tcp.Decimal))

	return b.String()
}

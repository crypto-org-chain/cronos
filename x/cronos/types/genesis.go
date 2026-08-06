package types

// this line is used by starport scaffolding # genesis/types/import
// this line is used by starport scaffolding # ibc/genesistype/import

// DefaultIndex is the default capability global index
const DefaultIndex uint64 = 1

// DefaultGenesis returns the default Capability genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
		// this line is used by starport scaffolding # ibc/genesistype/default
		// this line is used by starport scaffolding # genesis/types/default
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	// this line is used by starport scaffolding # ibc/genesistype/validate

	// this line is used by starport scaffolding # genesis/types/validate

	if err := gs.Params.Validate(); err != nil {
		return err
	}

	for _, m := range gs.ExternalContracts {
		if err := ValidateTokenMapping(m.Denom, m.Contract); err != nil {
			return err
		}
	}
	for _, m := range gs.AutoContracts {
		if err := ValidateTokenMapping(m.Denom, m.Contract); err != nil {
			return err
		}
	}

	return nil
}

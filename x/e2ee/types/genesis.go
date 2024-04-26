package types

// DefaultGenesis returns the default Capability genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	for _, key := range gs.Keys {
		if err := key.Validate(); err != nil {
			return err
		}
	}
	return nil
}

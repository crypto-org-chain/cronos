package types

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:              DefaultParams(),
		LastSentHeight:      0,
		PendingAttestations: []*PendingAttestationRecord{},
		V2ClientID:          "", // Empty by default, set via governance or setup
	}
}

// GenesisState defines the attestation module's genesis state
type GenesisState struct {
	Params              Params                      `json:"params"`
	LastSentHeight      uint64                      `json:"last_sent_height"`
	PendingAttestations []*PendingAttestationRecord `json:"pending_attestations"`
	V2ClientID          string                      `json:"v2_client_id"` // IBC v2 client ID for attestation layer (explicit config)
	// V1ChannelID and V1PortID removed - discovered automatically via IBC callbacks
}

// PendingAttestationRecord stores a pending attestation for genesis export
type PendingAttestationRecord struct {
	Height      uint64                `json:"height"`
	Attestation *BlockAttestationData `json:"attestation"`
}

// Validate performs basic genesis state validation returning an error upon any failure
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}

	// Validate pending attestations
	for _, pa := range gs.PendingAttestations {
		if pa.Height == 0 {
			return ErrInvalidBlockHeight.Wrap("block height cannot be 0")
		}
		if pa.Attestation == nil {
			return ErrInvalidPacketData.Wrap("attestation data cannot be nil")
		}
	}

	return nil
}

// DefaultParams returns default module parameters
func DefaultParams() Params {
	return Params{
		AttestationInterval:    500, // Send attestation every 500 blocks
		AttestationEnabled:     true,
		PacketTimeoutTimestamp: 600000000000, // 10 minutes in nanoseconds
	}
}

// Validate validates the set of params
func (p Params) Validate() error {
	if p.AttestationInterval == 0 {
		return ErrInvalidParams.Wrap("attestation interval must be greater than 0")
	}

	if p.PacketTimeoutTimestamp == 0 {
		return ErrInvalidParams.Wrap("packet timeout timestamp must be greater than 0")
	}

	return nil
}

package types

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:              DefaultParams(),
		PortId:              PortID,
		LastSentHeight:      0,
		PendingAttestations: []*PendingAttestationRecord{},
	}
}

// GenesisState defines the attestation module's genesis state
type GenesisState struct {
	Params              Params                      `json:"params"`
	PortId              string                      `json:"port_id"`
	ChannelId           string                      `json:"channel_id"`
	LastSentHeight      uint64                      `json:"last_sent_height"`
	PendingAttestations []*PendingAttestationRecord `json:"pending_attestations"`
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

	// Validate port ID
	if gs.PortId == "" {
		return ErrInvalidParams.Wrap("port ID cannot be empty")
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
		PortId:                   PortID,
		AttestationBatchSize:     10, // Send attestation every 10 blocks
		MinValidatorsForFinality: 2,
		AttestationEnabled:       true,
		PacketTimeoutTimestamp:   600000000000, // 10 minutes in nanoseconds
	}
}

// Validate validates the set of params
func (p Params) Validate() error {
	if p.PortId == "" {
		return ErrInvalidParams.Wrap("port ID cannot be empty")
	}

	if p.AttestationBatchSize == 0 {
		return ErrInvalidParams.Wrap("attestation batch size must be greater than 0")
	}

	if p.MinValidatorsForFinality == 0 {
		return ErrInvalidParams.Wrap("min validators for finality must be greater than 0")
	}

	if p.PacketTimeoutTimestamp == 0 {
		return ErrInvalidParams.Wrap("packet timeout timestamp must be greater than 0")
	}

	return nil
}

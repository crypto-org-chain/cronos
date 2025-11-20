package types

import (
	"cosmossdk.io/errors"
)

// x/attestation module sentinel errors
var (
	ErrInvalidPacketData = errors.Register(ModuleName, 2, "invalid packet data")
	ErrInvalidAck        = errors.Register(ModuleName, 3, "invalid acknowledgement")
	// ErrChannelNotFound removed - IBC v1 channels not used
	ErrInvalidChannel        = errors.Register(ModuleName, 5, "invalid channel")
	ErrInvalidBlockHeight    = errors.Register(ModuleName, 6, "invalid block height")
	ErrAttestationNotFound   = errors.Register(ModuleName, 7, "attestation not found")
	ErrInvalidSignature      = errors.Register(ModuleName, 8, "invalid signature")
	ErrDuplicateAttestation  = errors.Register(ModuleName, 9, "duplicate attestation")
	ErrNoAttestationsToSend  = errors.Register(ModuleName, 10, "no attestations to send")
	ErrInvalidParams         = errors.Register(ModuleName, 11, "invalid parameters")
	ErrAttestationDisabled   = errors.Register(ModuleName, 12, "attestation is disabled")
	ErrInvalidChainID        = errors.Register(ModuleName, 13, "invalid chain ID")
	ErrFailedToSendPacket    = errors.Register(ModuleName, 14, "failed to send IBC packet")
	ErrInvalidFinalityStatus = errors.Register(ModuleName, 15, "invalid finality status")
)

package types

const (
	// ModuleName defines the module name
	ModuleName = "attestation"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// QuerierRoute defines the module's query routing key
	QuerierRoute = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_attestation"

	// Version defines the IBC v2 attestation module version
	Version = "attestation-1"

	// PortID defines the IBC port ID for attestation
	PortID = "attestation"
)

// Consensus store key prefixes (minimal, BFT replicated)
var (
	// LastSentHeightKey stores the last block height sent for attestation
	LastSentHeightKey = []byte{0x01}

	// ParamsKey stores the module parameters
	ParamsKey = []byte{0x02}

	// V2ClientIDPrefix stores IBC v2 client IDs (prefix + key -> clientID)
	V2ClientIDPrefix = []byte{0x03}

	// HighestFinalityHeightKey stores the highest finalized block height (consensus storage)
	// This is the ONLY finality-related data stored in consensus state for tracking progress
	HighestFinalityHeightKey = []byte{0x04}
)

// Local (non-consensus) store key prefixes
var (
	// PendingAttestationsPrefix stores pending attestations by height (local storage)
	// Each validator tracks their own pending attestations independently
	PendingAttestationsPrefix = []byte{0xF1}

	// FinalizedBlocksPrefix stores finalized block data (local storage)
	// Full finality data is NOT stored in consensus state, only in local database
	FinalizedBlocksPrefix = []byte{0xF2}
)

// GetPendingAttestationKey returns the key for a pending attestation by height
func GetPendingAttestationKey(height uint64) []byte {
	return append(PendingAttestationsPrefix, UintToBytes(height)...)
}

// GetFinalizedBlockKey returns the key for local (non-consensus) finalized block storage
// This key is used in the local database, not in consensus state
func GetFinalizedBlockKey(height uint64) []byte {
	return append(FinalizedBlocksPrefix, UintToBytes(height)...)
}

// UintToBytes converts uint64 to big-endian bytes
func UintToBytes(val uint64) []byte {
	b := make([]byte, 8)
	for i := 0; i < 8; i++ {
		b[7-i] = byte(val >> (i * 8))
	}
	return b
}

// BytesToUint converts big-endian bytes to uint64
func BytesToUint(b []byte) uint64 {
	var val uint64
	for i := 0; i < 8 && i < len(b); i++ {
		val |= uint64(b[7-i]) << (i * 8)
	}
	return val
}

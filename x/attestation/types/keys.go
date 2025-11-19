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

	// Version defines the IBC application version
	Version = "attestation-1"

	// PortID is the default port id that module binds to
	PortID = "attestation"
)

// Store key prefixes
var (
	// AttestationSequenceKey tracks the next attestation ID
	AttestationSequenceKey = []byte{0x01}

	// PendingAttestationsPrefix stores pending attestations by height
	PendingAttestationsPrefix = []byte{0x02}

	// FinalizedBlocksPrefix stores finalized block heights
	FinalizedBlocksPrefix = []byte{0x03}

	// LastSentHeightKey stores the last block height sent for attestation
	LastSentHeightKey = []byte{0x04}

	// IBC ChannelKey stores the IBC channel ID for attestation
	IBCChannelKey = []byte{0x05}

	// ParamsKey stores the module parameters
	ParamsKey = []byte{0x06}
)

// GetPendingAttestationKey returns the key for a pending attestation by height
func GetPendingAttestationKey(height uint64) []byte {
	return append(PendingAttestationsPrefix, UintToBytes(height)...)
}

// GetFinalizedBlockKey returns the key for a finalized block by height
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

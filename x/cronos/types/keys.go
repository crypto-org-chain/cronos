package types

const (
	// ModuleName defines the module name
	ModuleName = "cronos"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey is the message route for slashing
	RouterKey = ModuleName

	// QuerierRoute defines the module's query routing key
	QuerierRoute = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_cronos"

	// this line is used by starport scaffolding # ibc/keys/name
)

// prefix bytes for the cronos persistent store
const (
	prefixDenomToExternalContract = iota + 1
	prefixDenomToAutoContract
)

// KVStore key prefixes
var (
	KeyPrefixDenomToExternalContract = []byte{prefixDenomToExternalContract}
	KeyPrefixDenomToAutoContract     = []byte{prefixDenomToAutoContract}
)

// this line is used by starport scaffolding # ibc/keys/port

// DenomToExternalContractKey defines the store key for denom to contract mapping
func DenomToExternalContractKey(denom string) []byte {
	return append(KeyPrefixDenomToExternalContract, denom...)
}

// DenomToAutoContractKey defines the store key for denom to auto contract mapping
func DenomToAutoContractKey(denom string) []byte {
	return append(KeyPrefixDenomToAutoContract, denom...)
}

package types

const (
	// ModuleName defines the module name
	ModuleName = "icactl"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey is the message route for slashing
	RouterKey = ModuleName

	// QuerierRoute defines the module's query routing key
	QuerierRoute = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_icactl"

	// Version defines the current version the IBC module supports
	Version = "icactl-1"
)

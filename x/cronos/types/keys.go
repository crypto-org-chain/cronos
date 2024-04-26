package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// ModuleName defines the module name
	ModuleName = "cronos"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey is the message route for slashing
	RouterKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_cronos"

	// this line is used by starport scaffolding # ibc/keys/name
)

// prefix bytes for the cronos persistent store
const (
	prefixDenomToExternalContract = iota + 1
	prefixDenomToAutoContract
	prefixContractToDenom
	paramsKey
	prefixAdminToPermissions
	prefixBlockList
)

// KVStore key prefixes
var (
	KeyPrefixDenomToExternalContract = []byte{prefixDenomToExternalContract}
	KeyPrefixDenomToAutoContract     = []byte{prefixDenomToAutoContract}
	KeyPrefixContractToDenom         = []byte{prefixContractToDenom}
	// ParamsKey is the key for params.
	ParamsKey                   = []byte{paramsKey}
	KeyPrefixAdminToPermissions = []byte{prefixAdminToPermissions}
	KeyPrefixBlockList          = []byte{prefixBlockList}
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

// ContractToDenomKey defines the store key for contract to denom reverse index
func ContractToDenomKey(contract []byte) []byte {
	return append(KeyPrefixContractToDenom, contract...)
}

// AdminToPermissionsKey defines the store key for admin to permissions mapping
func AdminToPermissionsKey(address sdk.AccAddress) []byte {
	return append(KeyPrefixAdminToPermissions, address.Bytes()...)
}

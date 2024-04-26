package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// ModuleName defines the module name
	ModuleName = "e2ee"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey is the message route for e2ee
	RouterKey = ModuleName
)

const (
	prefixEncryptionKey = iota + 1
)

var KeyPrefixEncryptionKey = []byte{prefixEncryptionKey}

func KeyPrefix(addr sdk.AccAddress) []byte {
	key := make([]byte, 1+len(addr))
	key[0] = prefixEncryptionKey
	copy(key[1:], addr)
	return key
}

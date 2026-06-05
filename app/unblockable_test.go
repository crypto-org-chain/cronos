package app

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestIsUnblockable(t *testing.T) {
	// a listed address (raw 20-byte form) is unblockable
	listed := common.HexToAddress("0x007F588ca3FFe53F20cb03553Ca38bb13542FF89")
	require.True(t, IsUnblockable(listed.Bytes()))

	// case-insensitive: the same address in a different case is still matched,
	// since matching is on raw bytes
	listedLower := common.HexToAddress("0x007f588ca3ffe53f20cb03553ca38bb13542ff89")
	require.True(t, IsUnblockable(listedLower.Bytes()))

	// an address not in the list is blockable
	other := common.HexToAddress("0x0000000000000000000000000000000000000001")
	require.False(t, IsUnblockable(other.Bytes()))

	// empty / wrong-length input does not match
	require.False(t, IsUnblockable(nil))
	require.False(t, IsUnblockable([]byte{0x01, 0x02}))
}

func TestBuildUnblockableSet(t *testing.T) {
	// every hardcoded entry is indexed and looked up by its raw bytes
	require.Len(t, unblockableSet, len(unblockableHexAddresses))
	for _, h := range unblockableHexAddresses {
		require.True(t, IsUnblockable(common.HexToAddress(h).Bytes()), h)
	}

	// a malformed entry fails fast at construction time
	require.Panics(t, func() {
		buildUnblockableSet([]string{"not-an-address"})
	})
}

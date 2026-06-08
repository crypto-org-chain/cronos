package app

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// unblockableHexAddresses is the hardcoded set of EVM addresses that must never
// be blocked by either the mempool or the consensus blocklist. Entries here are
// filtered out when building both blocklist lookup maps, so they can never take
// effect regardless of node config or on-chain MsgStoreBlockList contents. This
// list is intentionally not configurable.
var unblockableHexAddresses = []string{
	"0x007F588ca3FFe53F20cb03553Ca38bb13542FF89",
	"0x4356e8c6Ddca1964b22ECd35cb74A74BDdeDe2a3",
	"0x3D7F2C478aAfdB65542BCB44bCeeC05849999d2D",
	"0xC543052518F7787936522926242f86BADD39Cb46",
	"0x405fCcd57dA8ffbd0F2C38D57B1DA933b00B7bC6",
	"0x31f58b04f03d791c56de058211f0c767af96b464",
	"0xA6dE01a2d62C6B5f3525d768f34d276652C554c8",
	"0x192E362B2810f604e0618B5033d26F3b85E05AF9",
	"0xA4EC772557A0E72985EA2532B72f363fA5379C11",
	"0xeae603121f38d43e801254d172fd0bee959918b6",
	"0x28b5a0e9C621a5BadaA536219b3a228C8168cf5d",
	"0x81D40F21F12A8F0E3252Bccb954D722d4c464B64",
	"0xfd78EE919681417d192449715b2594ab58f5D002",
	"0x1CcaFdffBC1b7B5C499c97322F961B7d929a41b4",
	"0x01fB02b8209c8A5c271a4fCB700Bfb9C80b5B614",
	"0xec546b6B005471ECf012e5aF77FBeC07e0FD8f78",
	"0xda95b41655EA94d93241d97432DAfb6B27148289",
	"0x3812789185aF19B2002c0DfAcC3C7926eCbA674D",
	"0xc375fe4b88c5858bD5521917D0C3418856Ac1FB1",
	"0x69F762B2f1706e15eF77F7F8C5b07Fda66844d67",
	"0x7ea46aDC49Eb1228350f76327c94b9F06A032bd9",
	"0x4E6B78bF26881E38FfB939945116Dd8d4DD48551",
	"0xBE3866F2Cdddc6A5dE252e50EFD9429BD3495007",
	"0x26132C4bCceFa08bBEa4Ca85E3dBB797Ba8C1f09",
	"0x1a061EDeA58DA99c2d09FdD1f9e6BA9DaB1413ff",
	"0xa64915eaf58b245b2d2bbe7a7dc8c69956ac8670",
	// test
	"0xCC5d9bF5C3662D8A86A45ed23B300bc06ab36644",
	"0xDaB2C01b1eBdf1D33eCF6Aff3a29b977a1EFba41",
}

// unblockableSet maps the raw 20-byte account-address form of each unblockable
// address to an empty struct, for O(1) lookup.
var unblockableSet = buildUnblockableSet(unblockableHexAddresses)

// buildUnblockableSet validates and indexes the hardcoded addresses. It panics
// on any malformed entry, since that is a programming error in a hardcoded list
// and should fail fast at startup.
func buildUnblockableSet(hexAddrs []string) map[string]struct{} {
	m := make(map[string]struct{}, len(hexAddrs))
	for _, h := range hexAddrs {
		if !common.IsHexAddress(h) {
			panic(fmt.Sprintf("invalid unblockable address: %s", h))
		}
		m[string(common.HexToAddress(h).Bytes())] = struct{}{}
	}
	return m
}

// IsUnblockable reports whether the given account-address bytes are in the
// hardcoded unblockable list.
func IsUnblockable(addr []byte) bool {
	_, ok := unblockableSet[string(addr)]
	return ok
}

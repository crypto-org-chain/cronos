# Unblockable List — Design

Date: 2026-06-03

## Problem

Cronos has two independent blocklists:

1. **Mempool level** — `BlockAddressesDecorator` (`app/block_address.go`), whose
   `blockedMap` is built in `app/app.go` `setAnteHandler` from the node's static
   config blacklist. Rejects blocked signers / EVM destinations / EIP-7702
   authorisations during `CheckTx`.
2. **Consensus level** — `ProposalHandler` (`app/proposal.go`), whose `blocklist`
   map is built in `SetBlockList` from the age-encrypted, on-chain blocklist blob
   (`MsgStoreBlockList`), refreshed each `EndBlock`. Enforced during
   `PrepareProposal` / `ProcessProposal`.

We need an **unblockable list**: a fixed set of addresses that must *never* be
subject to either blocklist. If one of these addresses appears in the config
blacklist or in a submitted on-chain blocklist, it must be filtered out and have
no blocking effect.

## Requirements

- The unblockable list is **hardcoded**, not configurable (no params, no config,
  no genesis field).
- Applies to **both** blocklists.
- An unblockable address can never be added to either lookup map, regardless of
  config blacklist contents or `MsgStoreBlockList` payloads.
- Covered by an integration test.

## Approach

Enforce "never blockable" by **filtering at map-construction time**. Both
blocklists build an in-memory lookup map of normalized addresses; we skip any
address that is in the hardcoded unblockable set when building either map. This:

- gives true "can never be added" semantics (the address is simply absent from
  the map that every check consults),
- adds zero per-transaction overhead (filtering happens only when the map is
  rebuilt: once at ante-handler setup, and on each blocklist refresh),
- keeps a single source of truth shared by both call sites.

Matching is done on the **raw 20-byte account-address bytes**, not on bech32
strings. Both call sites already hold the raw bytes at the point of insertion,
and byte matching avoids any dependence on the bech32 prefix or SDK config
initialization order. A cronos account address and an EVM address are both the
same 20 bytes.

## New file: `app/unblockable.go`

```go
package app

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
```

## Call site 1 — mempool (`app/app.go`, `setAnteHandler`)

In the loop that builds `blockedMap` from the config `blacklist`:

```go
for _, str := range blacklist {
	addr, err := sdk.AccAddressFromBech32(str)
	if err != nil {
		return fmt.Errorf("invalid bech32 address: %s, err: %w", str, err)
	}
	if IsUnblockable(addr) {
		continue
	}
	blockedMap[addr.String()] = struct{}{}
}
```

## Call site 2 — consensus (`app/proposal.go`, `SetBlockList`)

In the loop that builds `m` from the decrypted blocklist addresses:

```go
for _, s := range blocklist.Addresses {
	addr, err := h.addressCodec.StringToBytes(s)
	if err != nil {
		return fmt.Errorf("invalid bech32 address: %s, err: %w", s, err)
	}
	if IsUnblockable(addr) {
		continue
	}
	encoded, err := h.addressCodec.BytesToString(addr)
	if err != nil {
		return fmt.Errorf("invalid bech32 address: %s, err: %w", s, err)
	}
	m[encoded] = struct{}{}
}
```

## Testing

### Integration test (Python pystarport, `integration_tests/test_e2ee.py`)

`test_block_list_unblockable(cronos)`:

1. `gen_validator_identity(cronos)` to enable the encrypted blocklist path.
2. Choose one hardcoded unblockable address (an EVM hex constant in the test,
   converted to its bech32 form) as a transaction **destination**, and a normal
   controllable address (e.g. `signer1`) as a **control** blocked destination.
3. Submit a blocklist containing **both** addresses via `encrypt_to_validators`.
4. Send a tx from `signer2` **to the control address**: assert it is *not*
   included in a block (blocklist is active) — establishes the mechanism works.
5. Send a tx from `signer2` **to the unblockable address**: assert it *is*
   included (nonce advances) — proves the unblockable address was filtered out
   and that filtering is selective, not merely an empty-blocklist fast path.
6. Clear the blocklist.

Using the destination path means the test needs no private key for the
unblockable address.

### Go unit test (optional, `app/unblockable_test.go`)

Direct test of `IsUnblockable`: returns true for raw bytes of a listed address,
false for an unlisted address; and that `buildUnblockableSet` panics on a
malformed entry. Cheap and fast; complements the integration test.

## Out of scope

- No configurability, params, genesis, or governance control of the list.
- No changes to the per-tx check logic (`AnteHandle`, `ValidateTransaction`) —
  only the map-construction loops change.
- EIP-7702 / signer paths are inherently covered: a filtered address is absent
  from the map, so every check that consults the map passes for it.

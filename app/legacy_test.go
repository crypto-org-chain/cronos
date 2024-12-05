package app

import (
	"encoding/base64"
	"testing"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	evmenc "github.com/evmos/ethermint/encoding"
	"github.com/stretchr/testify/require"
)

func TestDecodeLegacyTx(t *testing.T) {
	// msg url: /icaauth.v1.MsgRegisterAccount
	tx := "Cl8KXQoeL2ljYWF1dGgudjEuTXNnUmVnaXN0ZXJBY2NvdW50EjsKK3RjcmMxcndsNW54bHJ4ZXd5eGVqZDA4OTh1cWFqNDc5NnY0Mm1hNTB5cDQSDGNvbm5lY3Rpb24tMBKAAQpXCk8KKC9ldGhlcm1pbnQuY3J5cHRvLnYxLmV0aHNlY3AyNTZrMS5QdWJLZXkSIwohA1Q2Pphmzen/aIkuFd/3k+8YQATsARLhmWV9RxF+FGS/EgQKAggBEiUKHwoIYmFzZXRjcm8SEzQ4NDgwMDAwMDAwMDAwMDAwMDAQgLUYGkHOZPyLl81RsTbNHTv4o4XjWtYwO1fm4NzYyuju4boHpmALIytbPm+saXwhxtUG6hPT+sAsu9Bk224A9xd/8isZAQ=="

	bz, err := base64.StdEncoding.DecodeString(tx)
	require.NoError(t, err)

	encodingConfig := evmenc.MakeConfig()

	_, err = encodingConfig.TxConfig.TxDecoder()(bz)
	require.Error(t, err, "")
	require.Contains(t, err.Error(), "unable to resolve type URL /icaauth.v1.MsgRegisterAccount")

	RegisterLegacyCodec(encodingConfig.Amino)
	RegisterLegacyInterfaces(encodingConfig.InterfaceRegistry)

	_, err = encodingConfig.TxConfig.TxDecoder()(bz)
	require.NoError(t, err)
}

func TestDecodeAuthzLegacyTx(t *testing.T) {
	// msg url: /cosmos.authz.v1beta1.MsgGrant
	tx := "Cr0BCroBCh4vY29zbW9zLmF1dGh6LnYxYmV0YTEuTXNnR3JhbnQSlwEKKmNyYzF4N3g5cGtmeGYzM2w4N2Z0c3BrNWFldHdua3IwbHZsdjMzNDZjZBIqY3JjMTZ6MGhlcno5OTg5NDZ3cjY1OWxyODRjOGM1NTZkYTU1ZGMzNGhoGj0KOwomL2Nvc21vcy5iYW5rLnYxYmV0YTEuU2VuZEF1dGhvcml6YXRpb24SEQoPCghiYXNldGNybxIDMjAwEl8KVwpPCigvZXRoZXJtaW50LmNyeXB0by52MS5ldGhzZWNwMjU2azEuUHViS2V5EiMKIQNg/r8Tea2PYFq2XE07fpnN97ASePg32cuO4HmUkEGpFBIECgIIARIEEMCaDBpBekTx2LtLIFDODLVr1OqMUR9UYjZBv0KAr8eCacs7jTw/szr3jCvtvHGNraifkLfBEDatH3Rp/wqaNvxXgGjH4gA="

	bz, err := base64.StdEncoding.DecodeString(tx)
	require.NoError(t, err)

	encodingConfig := evmenc.MakeConfig()

	_, err = encodingConfig.TxConfig.TxDecoder()(bz)
	require.Error(t, err, "")
	require.Contains(t, err.Error(), "unable to resolve type URL /cosmos.authz.v1beta1.MsgGrant")

	RegisterLegacyCodec(encodingConfig.Amino)
	RegisterLegacyInterfaces(encodingConfig.InterfaceRegistry)
	banktypes.RegisterLegacyAminoCodec(encodingConfig.Amino)
	banktypes.RegisterInterfaces(encodingConfig.InterfaceRegistry)

	_, err = encodingConfig.TxConfig.TxDecoder()(bz)
	require.NoError(t, err)
}

package app

import (
	"encoding/base64"
	"testing"

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

package types_test

import (
	"bytes"
	"fmt"
	"log"
	"testing"

	"filippo.io/age"
	cmdcfg "github.com/crypto-org-chain/cronos/cmd/cronosd/config"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestValidateMsgUpdateTokenMapping(t *testing.T) {
	cmdcfg.SetBech32Prefixes(sdk.GetConfig())

	testCases := []struct {
		name     string
		msg      *types.MsgUpdateTokenMapping
		expValid bool
	}{
		{
			"valid gravity denom",
			types.NewMsgUpdateTokenMapping("crc12luku6uxehhak02py4rcz65zu0swh7wjsrw0pp", "gravity0x6E7eef2b30585B2A4D45Ba9312015d5354FDB067", "0x57f96e6B86CdeFdB3d412547816a82E3E0EbF9D2", "", 0),
			true,
		},
		{
			"valid ibc denom",
			types.NewMsgUpdateTokenMapping("crc12luku6uxehhak02py4rcz65zu0swh7wjsrw0pp", "ibc/0000000000000000000000000000000000000000000000000000000000000000", "0x57f96e6B86CdeFdB3d412547816a82E3E0EbF9D2", "", 0),
			true,
		},
		{
			"invalid sender",
			types.NewMsgUpdateTokenMapping("crc12luku6uxehhak02py4r", "gravity0x6E7eef2b30585B2A4D45Ba9312015d5354FDB067", "0x57f96e6B86CdeFdB3d412547816a82E3E0EbF9D2", "", 0),
			false,
		},
		{
			"invalid denom",
			types.NewMsgUpdateTokenMapping("crc12luku6uxehhak02py4rcz65zu0swh7wjsrw0pp", "aaa", "0x57f96e6B86CdeFdB3d412547816a82E3E0EbF9D2", "", 0),
			false,
		},
		{
			"invalid contract address",
			types.NewMsgUpdateTokenMapping("crc12luku6uxehhak02py4rcz65zu0swh7wjsrw0pp", "gravity0x6E7eef2b30585B2A4D45Ba9312015d5354FDB067", "0x57f96e6B86CdeFdB3d4125", "", 0),
			false,
		},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Case %s", tc.name), func(t1 *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expValid {
				require.NoError(t1, err)
			} else {
				require.Error(t1, err)
			}
		})
	}
}

func TestValidateMsgStoreBlockList(t *testing.T) {
	cmdcfg.SetBech32Prefixes(sdk.GetConfig())

	publicKey := "age1cy0su9fwf3gf9mw868g5yut09p6nytfmmnktexz2ya5uqg9vl9sss4euqm"
	recipient, err := age.ParseX25519Recipient(publicKey)
	if err != nil {
		log.Fatalf("Failed to parse public key %q: %v", publicKey, err)
	}

	from := "crc12luku6uxehhak02py4rcz65zu0swh7wjsrw0pp"
	blob := []byte("valid blob data")
	testCases := []struct {
		name        string
		msg         *types.MsgStoreBlockList
		noEncrypt   bool
		expectError bool
		errorMsg    string
	}{
		{
			"valid message",
			types.NewMsgStoreBlockList(from, blob),
			false,
			false,
			"",
		},
		{
			"invalid sender address",
			types.NewMsgStoreBlockList("invalid", blob),
			false,
			true,
			"invalid sender address",
		},
		{
			"decryption error",
			types.NewMsgStoreBlockList(from, blob),
			true,
			true,
			"failed to read header",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.noEncrypt {
				out := new(bytes.Buffer)
				w, err := age.Encrypt(out, recipient)
				require.NoError(t, err)
				_, err = w.Write(tc.msg.Blob)
				require.NoError(t, err)
				err = w.Close()
				require.NoError(t, err)
				tc.msg.Blob = out.Bytes()
			}

			err = tc.msg.ValidateBasic()
			if tc.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

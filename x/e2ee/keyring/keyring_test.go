package keyring_test

import (
	"bytes"
	"io"
	"testing"

	"filippo.io/age"
	sdkkeyring "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/crypto-org-chain/cronos/v2/x/e2ee/keyring"
	"github.com/stretchr/testify/require"
)

func TestKeyring(t *testing.T) {
	kr, err := keyring.New("cronosd", sdkkeyring.BackendTest, t.TempDir(), nil)
	require.NoError(t, err)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	var ciphertext []byte
	{
		dst := bytes.NewBuffer(nil)
		writer, err := age.Encrypt(dst, identity.Recipient())
		require.NoError(t, err)
		writer.Write([]byte("test"))
		writer.Close()
		ciphertext = dst.Bytes()
	}

	require.NoError(t, kr.Set("test", []byte(identity.String())))

	secret, err := kr.Get("test")
	require.NoError(t, err)

	identity, err = age.ParseX25519Identity(string(secret))
	require.NoError(t, err)

	{
		reader, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
		require.NoError(t, err)
		bz, err := io.ReadAll(reader)
		require.NoError(t, err)

		require.Equal(t, []byte("test"), bz)
	}
}

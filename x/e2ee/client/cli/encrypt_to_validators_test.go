package cli

import (
	"bytes"
	"testing"

	"filippo.io/age"
	"github.com/stretchr/testify/require"
)

func TestValidatorRecipientsSkipsUnusableKeys(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	var warn bytes.Buffer
	recipients := validatorRecipients(
		&warn,
		[]string{"missing", "invalid", "valid"},
		[]string{"", "not-an-age-key", identity.Recipient().String()},
	)

	require.Len(t, recipients, 1)
	require.Equal(t, identity.Recipient().String(), recipients[0].(*age.X25519Recipient).String())
	require.Contains(t, warn.String(), "missing encryption key for validator missing")
	require.Contains(t, warn.String(), "invalid encryption key for validator invalid")

	var plain, cipher bytes.Buffer
	plain.WriteString("hello")
	require.NoError(t, encrypt(recipients, &plain, &cipher))
}

func TestEncryptWithoutRecipientsReturnsError(t *testing.T) {
	recipients := validatorRecipients(&bytes.Buffer{}, []string{"missing"}, []string{""})

	var plain, cipher bytes.Buffer
	plain.WriteString("hello")
	require.Error(t, encrypt(recipients, &plain, &cipher))
}

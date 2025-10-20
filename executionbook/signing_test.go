package executionbook

import (
	"encoding/hex"
	"testing"

	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"
)

// testLogger returns a no-op logger for testing
func testLogger() log.Logger {
	return log.NewNopLogger()
}

func TestLoadPrivKeyFromBytes(t *testing.T) {
	// Create a test Ed25519 key (32 bytes)
	ed25519KeyBytes := make([]byte, 32)
	for i := range ed25519KeyBytes {
		ed25519KeyBytes[i] = byte(i)
	}

	// Create a test secp256k1 key (32 bytes)
	secp256k1KeyBytes := make([]byte, 32)
	for i := range secp256k1KeyBytes {
		secp256k1KeyBytes[i] = byte(i + 100)
	}

	t.Run("Load Ed25519 key", func(t *testing.T) {
		privKey, err := LoadPrivKeyFromBytes(ed25519KeyBytes, "ed25519")
		require.NoError(t, err)
		require.NotNil(t, privKey)
		require.Equal(t, "ed25519", privKey.Type())

		// Verify we can get the public key
		pubKey := privKey.PubKey()
		require.NotNil(t, pubKey)
	})

	t.Run("Load Ed25519 key (default)", func(t *testing.T) {
		privKey, err := LoadPrivKeyFromBytes(ed25519KeyBytes, "")
		require.NoError(t, err)
		require.NotNil(t, privKey)
		require.Equal(t, "ed25519", privKey.Type())
	})

	t.Run("Load secp256k1 key", func(t *testing.T) {
		privKey, err := LoadPrivKeyFromBytes(secp256k1KeyBytes, "secp256k1")
		require.NoError(t, err)
		require.NotNil(t, privKey)
		require.Equal(t, "secp256k1", privKey.Type())

		// Verify we can get the public key
		pubKey := privKey.PubKey()
		require.NotNil(t, pubKey)
	})

	t.Run("Invalid key length", func(t *testing.T) {
		invalidKey := make([]byte, 16) // Wrong length
		_, err := LoadPrivKeyFromBytes(invalidKey, "ed25519")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid")
	})

	t.Run("Unsupported key type", func(t *testing.T) {
		_, err := LoadPrivKeyFromBytes(ed25519KeyBytes, "rsa")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported key type")
	})
}

func TestLoadPrivKeyFromHex(t *testing.T) {
	// Create a test key and encode it as hex
	keyBytes := make([]byte, 32)
	for i := range keyBytes {
		keyBytes[i] = byte(i * 2)
	}
	hexKey := hex.EncodeToString(keyBytes)

	t.Run("Load Ed25519 key from hex", func(t *testing.T) {
		privKey, err := LoadPrivKeyFromHex(hexKey, "ed25519")
		require.NoError(t, err)
		require.NotNil(t, privKey)
		require.Equal(t, "ed25519", privKey.Type())
	})

	t.Run("Invalid hex string", func(t *testing.T) {
		_, err := LoadPrivKeyFromHex("not-valid-hex", "ed25519")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid hex key")
	})
}

func TestSigningAndVerification(t *testing.T) {
	// Generate Ed25519 test key
	ed25519PrivKey := ed25519.GenPrivKey()
	ed25519PubKey := ed25519PrivKey.PubKey()

	// Generate secp256k1 test key
	secp256k1PrivKey := secp256k1.GenPrivKey()
	secp256k1PubKey := secp256k1PrivKey.PubKey()

	t.Run("Sign and verify with Ed25519", func(t *testing.T) {
		// Create a service with Ed25519 key
		service := &PriorityTxService{
			validatorPrivKey: ed25519PrivKey,
			validatorAddress: "cronosvaloper1abc123",
			logger:           testLogger(),
		}

		txHash := "0x1234567890abcdef"
		priorityLevel := uint32(1)

		// Sign the preconfirmation
		signature := service.signPreconfirmation(txHash, priorityLevel)
		require.NotNil(t, signature)
		require.NotEmpty(t, signature)

		// Verify the signature
		isValid := service.VerifyPreconfirmationSignature(txHash, priorityLevel, signature, ed25519PubKey)
		require.True(t, isValid, "Signature should be valid")
	})

	t.Run("Sign and verify with secp256k1", func(t *testing.T) {
		// Create a service with secp256k1 key
		service := &PriorityTxService{
			validatorPrivKey: secp256k1PrivKey,
			validatorAddress: "cronosvaloper1xyz789",
			logger:           testLogger(),
		}

		txHash := "0xfedcba0987654321"
		priorityLevel := uint32(1)

		// Sign the preconfirmation
		signature := service.signPreconfirmation(txHash, priorityLevel)
		require.NotNil(t, signature)
		require.NotEmpty(t, signature)

		// Verify the signature
		isValid := service.VerifyPreconfirmationSignature(txHash, priorityLevel, signature, secp256k1PubKey)
		require.True(t, isValid, "Signature should be valid")
	})

	t.Run("Signature verification fails with wrong public key", func(t *testing.T) {
		service := &PriorityTxService{
			validatorPrivKey: ed25519PrivKey,
			validatorAddress: "cronosvaloper1abc123",
			logger:           testLogger(),
		}

		txHash := "0x1234567890abcdef"
		priorityLevel := uint32(1)

		// Sign with Ed25519 key
		signature := service.signPreconfirmation(txHash, priorityLevel)
		require.NotNil(t, signature)

		// Try to verify with wrong public key (secp256k1 instead of Ed25519)
		isValid := service.VerifyPreconfirmationSignature(txHash, priorityLevel, signature, secp256k1PubKey)
		require.False(t, isValid, "Signature should be invalid with wrong public key")
	})

	t.Run("Signature verification fails with modified data", func(t *testing.T) {
		service := &PriorityTxService{
			validatorPrivKey: ed25519PrivKey,
			validatorAddress: "cronosvaloper1abc123",
			logger:           testLogger(),
		}

		txHash := "0x1234567890abcdef"
		priorityLevel := uint32(1)

		// Sign with original data
		signature := service.signPreconfirmation(txHash, priorityLevel)
		require.NotNil(t, signature)

		// Try to verify with different transaction hash
		differentTxHash := "0xdifferent_hash"
		isValid := service.VerifyPreconfirmationSignature(differentTxHash, priorityLevel, signature, ed25519PubKey)
		require.False(t, isValid, "Signature should be invalid with modified tx hash")

		// Try to verify with different priority level
		differentPriorityLevel := uint32(2)
		isValid = service.VerifyPreconfirmationSignature(txHash, differentPriorityLevel, signature, ed25519PubKey)
		require.False(t, isValid, "Signature should be invalid with modified priority level")
	})

	t.Run("Unsigned preconfirmation when no key configured", func(t *testing.T) {
		service := &PriorityTxService{
			validatorPrivKey: nil, // No key
			validatorAddress: "cronosvaloper1abc123",
		}

		txHash := "0x1234567890abcdef"
		priorityLevel := uint32(1)

		// Sign should return nil
		signature := service.signPreconfirmation(txHash, priorityLevel)
		require.Nil(t, signature)

		// Service should report signing is disabled
		require.False(t, service.IsSigningEnabled())
		require.Nil(t, service.GetPublicKey())
	})

	t.Run("Service helper methods", func(t *testing.T) {
		service := &PriorityTxService{
			validatorPrivKey: ed25519PrivKey,
			validatorAddress: "cronosvaloper1abc123",
		}

		// Check signing is enabled
		require.True(t, service.IsSigningEnabled())

		// Get public key
		pubKey := service.GetPublicKey()
		require.NotNil(t, pubKey)
		require.Equal(t, ed25519PubKey.Bytes(), pubKey.Bytes())
	})
}

func TestCreatePreconfirmationMessage(t *testing.T) {
	service := &PriorityTxService{
		validatorAddress: "cronosvaloper1test",
	}

	t.Run("Message is deterministic", func(t *testing.T) {
		txHash := "0xabcd1234"
		priorityLevel := uint32(1)

		// Create message twice
		msg1 := service.createPreconfirmationMessage(txHash, priorityLevel)
		msg2 := service.createPreconfirmationMessage(txHash, priorityLevel)

		// Should be identical
		require.Equal(t, msg1, msg2)
		require.Len(t, msg1, 32) // SHA-256 hash is 32 bytes
	})

	t.Run("Different inputs produce different messages", func(t *testing.T) {
		txHash1 := "0xabcd1234"
		txHash2 := "0xdifferent"
		priorityLevel := uint32(1)

		msg1 := service.createPreconfirmationMessage(txHash1, priorityLevel)
		msg2 := service.createPreconfirmationMessage(txHash2, priorityLevel)

		// Should be different
		require.NotEqual(t, msg1, msg2)
	})

	t.Run("Different priority levels produce different messages", func(t *testing.T) {
		txHash := "0xabcd1234"
		priorityLevel1 := uint32(1)
		priorityLevel2 := uint32(2)

		msg1 := service.createPreconfirmationMessage(txHash, priorityLevel1)
		msg2 := service.createPreconfirmationMessage(txHash, priorityLevel2)

		// Should be different
		require.NotEqual(t, msg1, msg2)
	})
}

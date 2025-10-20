package app

import (
	"encoding/hex"
	"testing"

	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	ethsecp256k1 "github.com/evmos/ethermint/crypto/ethsecp256k1"
	"github.com/stretchr/testify/require"
)

func TestParseSequencerPubKey(t *testing.T) {
	t.Run("Valid Ed25519 key", func(t *testing.T) {
		// Generate a valid ed25519 key
		privKey := ed25519.GenPrivKey()
		pubKey := privKey.PubKey()
		pubKeyHex := hex.EncodeToString(pubKey.Bytes())

		parsedKey, err := parseSequencerPubKey("ed25519", pubKeyHex)
		require.NoError(t, err)
		require.NotNil(t, parsedKey)
		require.Equal(t, pubKey.Bytes(), parsedKey.Bytes())
	})

	t.Run("Valid ECDSA key", func(t *testing.T) {
		// Generate a valid ECDSA key (Ethereum-compatible)
		privKey, err := ethsecp256k1.GenerateKey()
		require.NoError(t, err)
		pubKey := privKey.PubKey()
		pubKeyHex := hex.EncodeToString(pubKey.Bytes())

		parsedKey, err := parseSequencerPubKey("ecdsa", pubKeyHex)
		require.NoError(t, err)
		require.NotNil(t, parsedKey)
		require.Equal(t, pubKey.Bytes(), parsedKey.Bytes())
	})

	t.Run("Valid ECDSA key with eth_secp256k1 type", func(t *testing.T) {
		// Test alternate key type name
		privKey, err := ethsecp256k1.GenerateKey()
		require.NoError(t, err)
		pubKey := privKey.PubKey()
		pubKeyHex := hex.EncodeToString(pubKey.Bytes())

		parsedKey, err := parseSequencerPubKey("eth_secp256k1", pubKeyHex)
		require.NoError(t, err)
		require.NotNil(t, parsedKey)
		require.Equal(t, pubKey.Bytes(), parsedKey.Bytes())
	})

	t.Run("Invalid hex string", func(t *testing.T) {
		_, err := parseSequencerPubKey("ed25519", "not_hex")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode hex")
	})

	t.Run("Invalid Ed25519 key length", func(t *testing.T) {
		// 16 bytes instead of 32
		shortKey := "0123456789abcdef0123456789abcdef"
		_, err := parseSequencerPubKey("ed25519", shortKey)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid ed25519 public key length")
	})

	t.Run("Invalid ECDSA key length", func(t *testing.T) {
		// 16 bytes instead of 33 or 65
		shortKey := "0123456789abcdef0123456789abcdef"
		_, err := parseSequencerPubKey("ecdsa", shortKey)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid ecdsa public key length")
	})

	t.Run("Unsupported key type", func(t *testing.T) {
		privKey := ed25519.GenPrivKey()
		pubKeyHex := hex.EncodeToString(privKey.PubKey().Bytes())

		_, err := parseSequencerPubKey("rsa", pubKeyHex)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported key type")
	})
}

// mockAppOptions is a simple mock for testing
type mockAppOptions struct {
	opts map[string]interface{}
}

func newMockAppOptions() *mockAppOptions {
	return &mockAppOptions{
		opts: make(map[string]interface{}),
	}
}

func (m *mockAppOptions) Get(key string) interface{} {
	return m.opts[key]
}

func (m *mockAppOptions) Set(key string, value interface{}) {
	m.opts[key] = value
}

func TestLoadSequencerPubKeys(t *testing.T) {
	logger := log.NewNopLogger()

	t.Run("Empty configuration", func(t *testing.T) {
		appOpts := newMockAppOptions()

		keys, err := loadSequencerPubKeys(appOpts, logger)
		require.NoError(t, err)
		require.Empty(t, keys)
	})

	t.Run("Single valid sequencer", func(t *testing.T) {
		appOpts := newMockAppOptions()

		privKey := ed25519.GenPrivKey()
		pubKeyHex := hex.EncodeToString(privKey.PubKey().Bytes())

		appOpts.Set("sequencer.keys", []interface{}{
			map[string]interface{}{
				"id":     "seq1",
				"type":   "ed25519",
				"pubkey": pubKeyHex,
			},
		})

		keys, err := loadSequencerPubKeys(appOpts, logger)
		require.NoError(t, err)
		require.Len(t, keys, 1)
		require.Contains(t, keys, "seq1")
		require.Equal(t, privKey.PubKey().Bytes(), keys["seq1"].Bytes())
	})

	t.Run("Multiple valid sequencers", func(t *testing.T) {
		appOpts := newMockAppOptions()

		privKey1 := ed25519.GenPrivKey()
		privKey2, err := ethsecp256k1.GenerateKey()
		require.NoError(t, err)
		pubKeyHex1 := hex.EncodeToString(privKey1.PubKey().Bytes())
		pubKeyHex2 := hex.EncodeToString(privKey2.PubKey().Bytes())

		appOpts.Set("sequencer.keys", []interface{}{
			map[string]interface{}{
				"id":     "seq1",
				"type":   "ed25519",
				"pubkey": pubKeyHex1,
			},
			map[string]interface{}{
				"id":     "seq2",
				"type":   "ecdsa",
				"pubkey": pubKeyHex2,
			},
		})

		keys, err := loadSequencerPubKeys(appOpts, logger)
		require.NoError(t, err)
		require.Len(t, keys, 2)
		require.Contains(t, keys, "seq1")
		require.Contains(t, keys, "seq2")
	})

	t.Run("Invalid configuration type", func(t *testing.T) {
		appOpts := newMockAppOptions()
		appOpts.Set("sequencer.keys", "not_an_array")

		_, err := loadSequencerPubKeys(appOpts, logger)
		require.Error(t, err)
		require.Contains(t, err.Error(), "must be an array")
	})

	t.Run("Missing required fields", func(t *testing.T) {
		appOpts := newMockAppOptions()

		// Missing id
		appOpts.Set("sequencer.keys", []interface{}{
			map[string]interface{}{
				"type":   "ed25519",
				"pubkey": "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			},
		})

		keys, err := loadSequencerPubKeys(appOpts, logger)
		require.NoError(t, err) // Logs error but continues
		require.Empty(t, keys)  // Key not added due to missing id
	})

	t.Run("Invalid public key", func(t *testing.T) {
		appOpts := newMockAppOptions()

		appOpts.Set("sequencer.keys", []interface{}{
			map[string]interface{}{
				"id":     "seq1",
				"type":   "ed25519",
				"pubkey": "invalid_hex",
			},
		})

		keys, err := loadSequencerPubKeys(appOpts, logger)
		require.NoError(t, err) // Logs error but continues
		require.Empty(t, keys)  // Key not added due to invalid pubkey
	})

	t.Run("Mixed valid and invalid sequencers", func(t *testing.T) {
		appOpts := newMockAppOptions()

		privKey := ed25519.GenPrivKey()
		pubKeyHex := hex.EncodeToString(privKey.PubKey().Bytes())

		appOpts.Set("sequencer.keys", []interface{}{
			map[string]interface{}{
				"id":     "seq1",
				"type":   "ed25519",
				"pubkey": pubKeyHex,
			},
			map[string]interface{}{
				"id":     "seq2",
				"type":   "ed25519",
				"pubkey": "invalid",
			},
		})

		keys, err := loadSequencerPubKeys(appOpts, logger)
		require.NoError(t, err)
		require.Len(t, keys, 1) // Only valid key loaded
		require.Contains(t, keys, "seq1")
		require.NotContains(t, keys, "seq2")
	})
}

package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultSequencerConfig(t *testing.T) {
	config := DefaultSequencerConfig()

	// Sequencer should be disabled by default
	require.False(t, config.Enable)
	// Keys should be empty by default
	require.Empty(t, config.Keys)
}

func TestSequencerConfigStruct(t *testing.T) {
	testCases := []struct {
		name     string
		enable   bool
		expected bool
	}{
		{
			name:     "enabled",
			enable:   true,
			expected: true,
		},
		{
			name:     "disabled",
			enable:   false,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := SequencerConfig{
				Enable: tc.enable,
			}
			require.Equal(t, tc.expected, config.Enable)
		})
	}
}

func TestSequencerTemplateFormat(t *testing.T) {
	// Test that the template is properly formatted
	require.Contains(t, DefaultSequencerTemplate, "[sequencer]")
	require.Contains(t, DefaultSequencerTemplate, "enable")
	require.Contains(t, DefaultSequencerTemplate, "{{ .Sequencer.Enable }}")
}

func TestSequencerKeyConfig(t *testing.T) {
	key := SequencerKeyConfig{
		ID:     "test-sequencer",
		Type:   "ed25519",
		PubKey: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
	}

	require.Equal(t, "test-sequencer", key.ID)
	require.Equal(t, "ed25519", key.Type)
	require.Equal(t, "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", key.PubKey)
}

func TestDefaultVersionDBConfig(t *testing.T) {
	config := DefaultVersionDBConfig()

	// VersionDB should be disabled by default
	require.False(t, config.Enable)
}

func TestVersionDBConfigStruct(t *testing.T) {
	testCases := []struct {
		name     string
		enable   bool
		expected bool
	}{
		{
			name:     "enabled",
			enable:   true,
			expected: true,
		},
		{
			name:     "disabled",
			enable:   false,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := VersionDBConfig{
				Enable: tc.enable,
			}
			require.Equal(t, tc.expected, config.Enable)
		})
	}
}

func TestVersionDBTemplateFormat(t *testing.T) {
	// Test that the template is properly formatted
	require.Contains(t, DefaultVersionDBTemplate, "[versiondb]")
	require.Contains(t, DefaultVersionDBTemplate, "enable")
	require.Contains(t, DefaultVersionDBTemplate, "{{ .VersionDB.Enable }}")
}

func TestConfigConsistency(t *testing.T) {
	// Verify that default configs are consistent
	sequencerConfig := DefaultSequencerConfig()
	versionDBConfig := DefaultVersionDBConfig()

	// Sequencer should be disabled by default
	require.False(t, sequencerConfig.Enable)

	// VersionDB should be disabled by default
	require.False(t, versionDBConfig.Enable)
}

func TestSequencerConfigWithKeys(t *testing.T) {
	// Test configuration with multiple keys
	config := SequencerConfig{
		Enable: true,
		Keys: []SequencerKeyConfig{
			{
				ID:     "seq1",
				Type:   "ed25519",
				PubKey: "a1b2c3d4e5f6789012345678901234567890123456789012345678901234abcd",
			},
			{
				ID:     "seq2",
				Type:   "ecdsa",
				PubKey: "02a1b2c3d4e5f6789012345678901234567890123456789012345678901234ab",
			},
		},
	}

	require.True(t, config.Enable)
	require.Len(t, config.Keys, 2)
	require.Equal(t, "seq1", config.Keys[0].ID)
	require.Equal(t, "ed25519", config.Keys[0].Type)
	require.Equal(t, "seq2", config.Keys[1].ID)
	require.Equal(t, "ecdsa", config.Keys[1].Type)
}

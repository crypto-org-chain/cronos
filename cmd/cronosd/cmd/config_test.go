package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultPreconferConfig(t *testing.T) {
	config := DefaultPreconferConfig()

	// Preconfer should be enabled by default for backward compatibility
	require.True(t, config.Enable)
}

func TestPreconferConfigStruct(t *testing.T) {
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
			config := PreconferConfig{
				Enable: tc.enable,
			}
			require.Equal(t, tc.expected, config.Enable)
		})
	}
}

func TestPreconferTemplateFormat(t *testing.T) {
	// Test that the template is properly formatted
	require.Contains(t, DefaultPreconferTemplate, "[preconfer]")
	require.Contains(t, DefaultPreconferTemplate, "enable")
	require.Contains(t, DefaultPreconferTemplate, "{{ .Preconfer.Enable }}")
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
	preconferConfig := DefaultPreconferConfig()
	versionDBConfig := DefaultVersionDBConfig()

	// Preconfer should be enabled by default (backward compatibility)
	require.True(t, preconferConfig.Enable)

	// VersionDB should be disabled by default
	require.False(t, versionDBConfig.Enable)
}

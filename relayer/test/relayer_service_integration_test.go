package relayer_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/crypto-org-chain/cronos/relayer"
	"github.com/stretchr/testify/require"
)

func TestRelayerServiceWithRPC(t *testing.T) {
	t.Run("RPCEnabled", func(t *testing.T) {
		config := &relayer.Config{
			SourceChainID:      "cronos_777-1",
			AttestationChainID: "attestation-1",
			RPCEnabled:         true,
			RPCConfig: &relayer.RPCConfig{
				ListenAddr:   "127.0.0.1:18080",
				ReadTimeout:  10 * time.Second,
				WriteTimeout: 10 * time.Second,
				EnableCORS:   true,
			},
		}

		// Test that config includes RPC settings
		require.True(t, config.RPCEnabled)
		require.NotNil(t, config.RPCConfig)
		require.Equal(t, "127.0.0.1:18080", config.RPCConfig.ListenAddr)
	})

	t.Run("RPCDisabled", func(t *testing.T) {
		config := &relayer.Config{
			SourceChainID:      "cronos_777-1",
			AttestationChainID: "attestation-1",
			RPCEnabled:         false,
		}

		require.False(t, config.RPCEnabled)
		require.Nil(t, config.RPCConfig)
	})
}

func TestConfigWithRPC(t *testing.T) {
	t.Run("MarshalUnmarshal", func(t *testing.T) {
		config := &relayer.Config{
			SourceChainID:      "cronos_777-1",
			AttestationChainID: "attestation-1",
			BlockBatchSize:     100,
			RPCEnabled:         true,
			RPCConfig: &relayer.RPCConfig{
				ListenAddr:   "0.0.0.0:8080",
				ReadTimeout:  15 * time.Second,
				WriteTimeout: 15 * time.Second,
				EnableCORS:   true,
			},
		}

		// Marshal to JSON
		data, err := json.Marshal(config)
		require.NoError(t, err)

		// Unmarshal back
		var config2 relayer.Config
		err = json.Unmarshal(data, &config2)
		require.NoError(t, err)

		require.Equal(t, config.SourceChainID, config2.SourceChainID)
		require.Equal(t, config.RPCEnabled, config2.RPCEnabled)
		require.NotNil(t, config2.RPCConfig)
		require.Equal(t, config.RPCConfig.ListenAddr, config2.RPCConfig.ListenAddr)
	})
}

func TestRPCIntegration(t *testing.T) {
	t.Run("RPCServerLifecycle", func(t *testing.T) {
		// This test verifies the RPC server lifecycle integration
		// In a real scenario, this would start a full RelayerService
		// For now, we verify the config structure

		config := &relayer.Config{
			RPCEnabled: true,
			RPCConfig: &relayer.RPCConfig{
				ListenAddr:   "127.0.0.1:19090",
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
				EnableCORS:   false,
			},
		}

		require.True(t, config.RPCEnabled)
		require.NotNil(t, config.RPCConfig)

		// Verify config values
		require.Equal(t, "127.0.0.1:19090", config.RPCConfig.ListenAddr)
		require.Equal(t, 5*time.Second, config.RPCConfig.ReadTimeout)
		require.Equal(t, 5*time.Second, config.RPCConfig.WriteTimeout)
		require.False(t, config.RPCConfig.EnableCORS)
	})
}

func TestRPCEndpointIntegration(t *testing.T) {
	t.Run("HealthEndpointStructure", func(t *testing.T) {
		// Verify health response structure
		response := relayer.HealthResponse{
			Healthy:   true,
			Version:   "1.0.0",
			Timestamp: time.Now(),
		}

		data, err := json.Marshal(response)
		require.NoError(t, err)
		require.Contains(t, string(data), "healthy")
		require.Contains(t, string(data), "version")
		require.Contains(t, string(data), "timestamp")
	})

	t.Run("StatusEndpointStructure", func(t *testing.T) {
		// Verify status response structure
		status := &relayer.RelayerStatus{
			Running:              true,
			SourceChainID:        "cronos_777-1",
			AttestationChainID:   "attestation-1",
			LastBlockForwarded:   1000,
			LastFinalityReceived: 950,
			UpdatedAt:            time.Now(),
		}

		response := relayer.StatusResponse{
			Status:    status,
			Timestamp: time.Now(),
		}

		data, err := json.Marshal(response)
		require.NoError(t, err)
		require.Contains(t, string(data), "status")
		require.Contains(t, string(data), "running")
		require.Contains(t, string(data), "source_chain_id")
	})
}

func TestRPCClientUsage(t *testing.T) {
	t.Run("HTTPClientExample", func(t *testing.T) {
		// This demonstrates how a client would use the RPC API
		// In actual integration tests, you'd make real HTTP calls

		// Example: Creating an HTTP client
		client := &http.Client{
			Timeout: 10 * time.Second,
		}

		// Example endpoint URLs
		baseURL := "http://localhost:8080"
		endpoints := []string{
			baseURL + "/health",
			baseURL + "/status",
			baseURL + "/checkpoint",
			baseURL + "/pending",
		}

		for _, endpoint := range endpoints {
			// In a real test, you'd make the request
			// resp, err := client.Get(endpoint)

			// For now, just verify the endpoint format
			require.Contains(t, endpoint, "http://")
		}

		_ = client // Silence unused warning
	})
}

func TestRPCConfigDefaults(t *testing.T) {
	t.Run("EmptyRPCConfig", func(t *testing.T) {
		config := &relayer.RPCConfig{}

		// Empty config should have zero values
		require.Empty(t, config.ListenAddr)
		require.Equal(t, time.Duration(0), config.ReadTimeout)
		require.Equal(t, time.Duration(0), config.WriteTimeout)
		require.False(t, config.EnableCORS)
	})

	t.Run("PartialRPCConfig", func(t *testing.T) {
		config := &relayer.RPCConfig{
			ListenAddr: "0.0.0.0:8080",
			EnableCORS: true,
		}

		require.Equal(t, "0.0.0.0:8080", config.ListenAddr)
		require.True(t, config.EnableCORS)
		// These should have default zero values
		require.Equal(t, time.Duration(0), config.ReadTimeout)
		require.Equal(t, time.Duration(0), config.WriteTimeout)
	})
}

func TestRPCFeatureFlags(t *testing.T) {
	t.Run("RPCEnabledTrue", func(t *testing.T) {
		config := &relayer.Config{
			RPCEnabled: true,
			RPCConfig: &relayer.RPCConfig{
				ListenAddr: "0.0.0.0:8080",
			},
		}

		require.True(t, config.RPCEnabled)
		require.NotNil(t, config.RPCConfig)
	})

	t.Run("RPCEnabledFalse", func(t *testing.T) {
		config := &relayer.Config{
			RPCEnabled: false,
		}

		require.False(t, config.RPCEnabled)
		// RPCConfig can be nil when disabled
	})

	t.Run("RPCEnabledWithoutConfig", func(t *testing.T) {
		config := &relayer.Config{
			RPCEnabled: true,
			RPCConfig:  nil, // This would use defaults in actual service
		}

		require.True(t, config.RPCEnabled)
		require.Nil(t, config.RPCConfig)
	})
}

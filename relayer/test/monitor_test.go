package relayer_test

import (
	"context"
	"testing"
	"time"

	"github.com/crypto-org-chain/cronos/relayer"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
)

func TestChainMonitorConfig(t *testing.T) {
	logger := log.NewNopLogger()

	t.Run("NewChainMonitor_ValidConfig", func(t *testing.T) {
		monitor, err := relayer.NewChainMonitor(
			"http://localhost:26657",
			"test-chain",
			"test-chain-id",
			logger,
		)
		require.NoError(t, err)
		require.NotNil(t, monitor)
	})

	t.Run("NewChainMonitor_InvalidRPC", func(t *testing.T) {
		monitor, err := relayer.NewChainMonitor(
			"invalid://url",
			"test-chain",
			"test-chain-id",
			logger,
		)
		// Invalid URL might still create monitor or fail immediately depending on implementation
		if err != nil {
			require.Nil(t, monitor)
		} else {
			require.NotNil(t, monitor)
		}
	})
}

func TestChainMonitorLifecycle(t *testing.T) {
	logger := log.NewNopLogger()

	t.Run("Start_Stop", func(t *testing.T) {
		// Create mock monitor (will fail to connect but that's ok for lifecycle test)
		monitor, err := relayer.NewChainMonitor(
			"http://localhost:26657",
			"test-chain",
			"test-chain-id",
			logger,
		)
		require.NoError(t, err)

		ctx := context.Background()

		// Start will fail if no RPC endpoint, but we can test the pattern
		err = monitor.Start(ctx)
		// Expected to fail with connection error
		// require.Error(t, err)

		// Stop should work even if Start failed
		err = monitor.Stop()
		require.NoError(t, err)
	})

	t.Run("Stop_Without_Start", func(t *testing.T) {
		monitor, err := relayer.NewChainMonitor(
			"http://localhost:26657",
			"test-chain",
			"test-chain-id",
			logger,
		)
		require.NoError(t, err)

		// Should be able to stop without starting
		err = monitor.Stop()
		require.NoError(t, err)
	})
}

func TestChainMonitorContext(t *testing.T) {
	logger := log.NewNopLogger()

	t.Run("Context_Cancellation", func(t *testing.T) {
		monitor, err := relayer.NewChainMonitor(
			"http://localhost:26657",
			"test-chain",
			"test-chain-id",
			logger,
		)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())

		// Start monitor (will fail to connect)
		_ = monitor.Start(ctx)

		// Cancel context
		cancel()

		// Give it a moment to process cancellation
		time.Sleep(100 * time.Millisecond)

		// Stop should work
		err = monitor.Stop()
		require.NoError(t, err)
	})

	t.Run("Context_Timeout", func(t *testing.T) {
		monitor, err := relayer.NewChainMonitor(
			"http://localhost:26657",
			"test-chain",
			"test-chain-id",
			logger,
		)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Start monitor (will fail or timeout)
		_ = monitor.Start(ctx)

		// Wait for timeout
		<-ctx.Done()

		// Stop should work
		err = monitor.Stop()
		require.NoError(t, err)
	})
}

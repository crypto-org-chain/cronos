package executionbook

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

func TestVerifyMempool(t *testing.T) {
	logger := log.NewNopLogger()

	t.Run("Verify mock mempool wrapped by preconfer", func(t *testing.T) {
		baseMpool := newMockMempool()
		txDecoder := func(txBytes []byte) (sdk.Tx, error) {
			return &mockTx{memo: string(txBytes)}, nil
		}

		preconferMpool := NewExecutionBook(ExecutionBookConfig{
			BaseMempool:   baseMpool,
			TxDecoder:     txDecoder,
			PriorityBoost: 1000000,
			Logger:        logger,
		})

		verification := VerifyMempool(preconferMpool, logger)

		require.True(t, verification.IsPreconferMempool)
		require.Equal(t, "*executionbook.mockMempool", verification.BaseMempoolType)
		require.False(t, verification.IsPriorityNonceMempool)
		require.Equal(t, int64(1000000), verification.PriorityBoost)
		require.True(t, verification.SupportsInsertWithGasWanted)
	})

	t.Run("Verify PriorityNonceMempool wrapped by preconfer", func(t *testing.T) {
		baseMpool := mempool.NewPriorityMempool(mempool.PriorityNonceMempoolConfig[int64]{
			TxPriority:      mempool.NewDefaultTxPriority(),
			SignerExtractor: mempool.NewDefaultSignerExtractionAdapter(),
			MaxTx:           100,
		})

		txDecoder := func(txBytes []byte) (sdk.Tx, error) {
			return &mockTx{memo: string(txBytes)}, nil
		}

		preconferMpool := NewExecutionBook(ExecutionBookConfig{
			BaseMempool:   baseMpool,
			TxDecoder:     txDecoder,
			PriorityBoost: 1000000,
			Logger:        logger,
		})

		verification := VerifyMempool(preconferMpool, logger)

		require.True(t, verification.IsPreconferMempool)
		require.Equal(t, PriorityNonceMempoolType, verification.BaseMempoolType)
		require.True(t, verification.IsPriorityNonceMempool)
		require.Equal(t, int64(1000000), verification.PriorityBoost)
		require.True(t, verification.SupportsInsertWithGasWanted)
	})

	t.Run("Verify direct PriorityNonceMempool (not wrapped)", func(t *testing.T) {
		baseMpool := mempool.NewPriorityMempool(mempool.PriorityNonceMempoolConfig[int64]{
			TxPriority:      mempool.NewDefaultTxPriority(),
			SignerExtractor: mempool.NewDefaultSignerExtractionAdapter(),
			MaxTx:           100,
		})

		verification := VerifyMempool(baseMpool, logger)

		require.False(t, verification.IsPreconferMempool)
		require.Equal(t, PriorityNonceMempoolType, verification.BaseMempoolType)
		require.True(t, verification.IsPriorityNonceMempool)
		require.Equal(t, int64(0), verification.PriorityBoost)
		require.True(t, verification.SupportsInsertWithGasWanted)
	})
}

func TestValidatePreconferMempool(t *testing.T) {
	logger := log.NewNopLogger()

	t.Run("Valid preconfer mempool with PriorityNonceMempool", func(t *testing.T) {
		baseMpool := mempool.NewPriorityMempool(mempool.PriorityNonceMempoolConfig[int64]{
			TxPriority:      mempool.NewDefaultTxPriority(),
			SignerExtractor: mempool.NewDefaultSignerExtractionAdapter(),
			MaxTx:           100,
		})

		txDecoder := func(txBytes []byte) (sdk.Tx, error) {
			return &mockTx{memo: string(txBytes)}, nil
		}

		preconferMpool := NewExecutionBook(ExecutionBookConfig{
			BaseMempool:   baseMpool,
			TxDecoder:     txDecoder,
			PriorityBoost: 1000000,
			Logger:        logger,
		})

		err := ValidatePreconferMempool(preconferMpool)
		require.NoError(t, err)
	})

	t.Run("Invalid - not a preconfer mempool", func(t *testing.T) {
		baseMpool := mempool.NewPriorityMempool(mempool.PriorityNonceMempoolConfig[int64]{
			TxPriority:      mempool.NewDefaultTxPriority(),
			SignerExtractor: mempool.NewDefaultSignerExtractionAdapter(),
			MaxTx:           100,
		})

		err := ValidatePreconferMempool(baseMpool)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not a preconfer.ExecutionBook")
	})

	t.Run("Invalid - base mempool is not PriorityNonceMempool", func(t *testing.T) {
		baseMpool := newMockMempool()
		txDecoder := func(txBytes []byte) (sdk.Tx, error) {
			return &mockTx{memo: string(txBytes)}, nil
		}

		preconferMpool := NewExecutionBook(ExecutionBookConfig{
			BaseMempool:   baseMpool,
			TxDecoder:     txDecoder,
			PriorityBoost: 1000000,
			Logger:        logger,
		})

		err := ValidatePreconferMempool(preconferMpool)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not wrapping a PriorityNonceMempool")
	})

	t.Run("Invalid - zero priority boost", func(t *testing.T) {
		baseMpool := mempool.NewPriorityMempool(mempool.PriorityNonceMempoolConfig[int64]{
			TxPriority:      mempool.NewDefaultTxPriority(),
			SignerExtractor: mempool.NewDefaultSignerExtractionAdapter(),
			MaxTx:           100,
		})

		txDecoder := func(txBytes []byte) (sdk.Tx, error) {
			return &mockTx{memo: string(txBytes)}, nil
		}

		// Manually create with zero boost (bypassing the NewExecutionBook default)
		preconferMpool := &ExecutionBook{
			Mempool:       baseMpool,
			txDecoder:     txDecoder,
			logger:        logger,
			priorityBoost: 0,
		}

		err := ValidatePreconferMempool(preconferMpool)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid priority boost")
	})
}

func TestLogMempoolConfiguration(t *testing.T) {
	// This test just ensures the function doesn't panic
	// and can be called with various mempool types
	logger := log.NewNopLogger()

	t.Run("Log preconfer mempool", func(t *testing.T) {
		baseMpool := mempool.NewPriorityMempool(mempool.PriorityNonceMempoolConfig[int64]{
			TxPriority:      mempool.NewDefaultTxPriority(),
			SignerExtractor: mempool.NewDefaultSignerExtractionAdapter(),
			MaxTx:           100,
		})

		txDecoder := func(txBytes []byte) (sdk.Tx, error) {
			return &mockTx{memo: string(txBytes)}, nil
		}

		preconferMpool := NewExecutionBook(ExecutionBookConfig{
			BaseMempool:   baseMpool,
			TxDecoder:     txDecoder,
			PriorityBoost: 1000000,
			Logger:        logger,
		})

		// Should not panic
		require.NotPanics(t, func() {
			LogMempoolConfiguration(preconferMpool, logger)
		})
	})

	t.Run("Log direct PriorityNonceMempool", func(t *testing.T) {
		baseMpool := mempool.NewPriorityMempool(mempool.PriorityNonceMempoolConfig[int64]{
			TxPriority:      mempool.NewDefaultTxPriority(),
			SignerExtractor: mempool.NewDefaultSignerExtractionAdapter(),
			MaxTx:           100,
		})

		// Should not panic
		require.NotPanics(t, func() {
			LogMempoolConfiguration(baseMpool, logger)
		})
	})
}

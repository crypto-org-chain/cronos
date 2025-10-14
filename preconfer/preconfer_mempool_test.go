package preconfer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

// mockMempool is a simple mock mempool for testing
type mockMempool struct {
	txs                   []sdk.Tx
	lastInsertedPriority  int64
	lastInsertedGasWanted uint64
}

func newMockMempool() *mockMempool {
	return &mockMempool{
		txs: make([]sdk.Tx, 0),
	}
}

func (mm *mockMempool) Insert(ctx context.Context, tx sdk.Tx) error {
	// Capture the priority from context for testing
	if sdkCtx, ok := ctx.(sdk.Context); ok {
		mm.lastInsertedPriority = sdkCtx.Priority()
	}
	mm.txs = append(mm.txs, tx)
	return nil
}

func (mm *mockMempool) InsertWithGasWanted(ctx context.Context, tx sdk.Tx, gasWanted uint64) error {
	// Capture the priority and gas from context for testing
	if sdkCtx, ok := ctx.(sdk.Context); ok {
		mm.lastInsertedPriority = sdkCtx.Priority()
	}
	mm.lastInsertedGasWanted = gasWanted
	mm.txs = append(mm.txs, tx)
	return nil
}

func (mm *mockMempool) Select(ctx context.Context, txs [][]byte) mempool.Iterator {
	return nil
}

func (mm *mockMempool) CountTx() int {
	return len(mm.txs)
}

func (mm *mockMempool) Remove(tx sdk.Tx) error {
	for i, t := range mm.txs {
		if t == tx {
			mm.txs = append(mm.txs[:i], mm.txs[i+1:]...)
			return nil
		}
	}
	return nil
}

func TestMempool_Insert(t *testing.T) {
	// Create SDK context with base priority
	basePriority := int64(100)
	ctx := sdk.Context{}.WithPriority(basePriority)

	baseMpool := newMockMempool()

	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: string(txBytes)}, nil
	}

	priorityBoost := int64(1000000)
	mpool := NewMempool(MempoolConfig{
		BaseMempool:   baseMpool,
		TxDecoder:     txDecoder,
		PriorityBoost: priorityBoost,
		Logger:        log.NewNopLogger(),
	})

	t.Run("Insert priority transaction with boosted priority", func(t *testing.T) {
		priorityTx := &mockTx{memo: "PRIORITY:1"}
		err := mpool.Insert(ctx, priorityTx)
		require.NoError(t, err)
		require.Equal(t, 1, mpool.CountTx())

		// Verify priority was boosted
		expectedPriority := basePriority + priorityBoost
		require.Equal(t, expectedPriority, baseMpool.lastInsertedPriority,
			"Priority transaction should have boosted priority")
	})

	t.Run("Insert normal transaction with original priority", func(t *testing.T) {
		normalTx := &mockTx{memo: "normal"}
		err := mpool.Insert(ctx, normalTx)
		require.NoError(t, err)
		require.Equal(t, 2, mpool.CountTx())

		// Verify priority was NOT boosted
		require.Equal(t, basePriority, baseMpool.lastInsertedPriority,
			"Normal transaction should keep original priority")
	})
}

func TestMempool_Remove(t *testing.T) {
	ctx := context.Background()
	baseMpool := newMockMempool()

	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: string(txBytes)}, nil
	}

	mpool := NewMempool(MempoolConfig{
		BaseMempool:   baseMpool,
		TxDecoder:     txDecoder,
		PriorityBoost: 1000000,
		Logger:        log.NewNopLogger(),
	})

	t.Run("Remove transaction", func(t *testing.T) {
		tx := &mockTx{memo: "test"}
		err := mpool.Insert(ctx, tx)
		require.NoError(t, err)
		require.Equal(t, 1, mpool.CountTx())

		err = mpool.Remove(tx)
		require.NoError(t, err)
		require.Equal(t, 0, mpool.CountTx())
	})
}

func TestMempool_CountTx(t *testing.T) {
	ctx := context.Background()
	baseMpool := newMockMempool()

	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: string(txBytes)}, nil
	}

	mpool := NewMempool(MempoolConfig{
		BaseMempool:   baseMpool,
		TxDecoder:     txDecoder,
		PriorityBoost: 1000000,
		Logger:        log.NewNopLogger(),
	})

	t.Run("Count starts at zero", func(t *testing.T) {
		require.Equal(t, 0, mpool.CountTx())
	})

	t.Run("Count increases with inserts", func(t *testing.T) {
		for i := 1; i <= 5; i++ {
			tx := &mockTx{memo: "test"}
			err := mpool.Insert(ctx, tx)
			require.NoError(t, err)
			require.Equal(t, i, mpool.CountTx())
		}
	})
}

func TestMempool_GetSetPriorityBoost(t *testing.T) {
	baseMpool := newMockMempool()

	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: string(txBytes)}, nil
	}

	initialBoost := int64(1000000)
	mpool := NewMempool(MempoolConfig{
		BaseMempool:   baseMpool,
		TxDecoder:     txDecoder,
		PriorityBoost: initialBoost,
		Logger:        log.NewNopLogger(),
	})

	t.Run("Get initial priority boost", func(t *testing.T) {
		boost := mpool.GetPriorityBoost()
		require.Equal(t, initialBoost, boost)
	})

	t.Run("Set new priority boost", func(t *testing.T) {
		newBoost := int64(2000000)
		mpool.SetPriorityBoost(newBoost)
		require.Equal(t, newBoost, mpool.GetPriorityBoost())
	})

	t.Run("Reject negative priority boost", func(t *testing.T) {
		currentBoost := mpool.GetPriorityBoost()
		mpool.SetPriorityBoost(-100)
		require.Equal(t, currentBoost, mpool.GetPriorityBoost(), "Should reject negative boost")
	})
}

func TestMempool_DefaultConfig(t *testing.T) {
	baseMpool := newMockMempool()

	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: string(txBytes)}, nil
	}

	t.Run("Default priority boost when zero", func(t *testing.T) {
		mpool := NewMempool(MempoolConfig{
			BaseMempool:   baseMpool,
			TxDecoder:     txDecoder,
			PriorityBoost: 0, // Should use default
			Logger:        nil,
		})
		require.Equal(t, DefaultPriorityBoost, mpool.GetPriorityBoost())
	})

	t.Run("Nil logger uses nop logger", func(t *testing.T) {
		mpool := NewMempool(MempoolConfig{
			BaseMempool:   baseMpool,
			TxDecoder:     txDecoder,
			PriorityBoost: 1000000,
			Logger:        nil,
		})
		require.NotNil(t, mpool)
	})
}

func TestMempool_GetStats(t *testing.T) {
	ctx := context.Background()
	baseMpool := newMockMempool()

	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: string(txBytes)}, nil
	}

	mpool := NewMempool(MempoolConfig{
		BaseMempool:   baseMpool,
		TxDecoder:     txDecoder,
		PriorityBoost: 1000000,
		Logger:        log.NewNopLogger(),
	})

	t.Run("Get stats empty mempool", func(t *testing.T) {
		stats := mpool.GetStats()
		require.Contains(t, stats, "Mempool")
		require.Contains(t, stats, "count=0")
		require.Contains(t, stats, "boost=1000000")
	})

	t.Run("Get stats with transactions", func(t *testing.T) {
		tx := &mockTx{memo: "test"}
		err := mpool.Insert(ctx, tx)
		require.NoError(t, err)

		stats := mpool.GetStats()
		require.Contains(t, stats, "count=1")
	})
}

func TestMempool_BaseMempoolType(t *testing.T) {
	t.Run("Check base mempool type with mock", func(t *testing.T) {
		baseMpool := newMockMempool()
		txDecoder := func(txBytes []byte) (sdk.Tx, error) {
			return &mockTx{memo: string(txBytes)}, nil
		}

		mpool := NewMempool(MempoolConfig{
			BaseMempool:   baseMpool,
			TxDecoder:     txDecoder,
			PriorityBoost: 1000000,
			Logger:        log.NewNopLogger(),
		})

		// Verify we can get the base mempool
		base := mpool.GetBaseMempool()
		require.NotNil(t, base)
		require.Equal(t, baseMpool, base)

		// Check type
		typeName := mpool.GetBaseMempoolType()
		require.Equal(t, "*preconfer.mockMempool", typeName)

		// This is a mock, not a PriorityNonceMempool
		require.False(t, mpool.IsPriorityNonceMempool())
	})

	t.Run("Check with actual PriorityNonceMempool", func(t *testing.T) {
		// Create an actual PriorityNonceMempool
		baseMpool := mempool.NewPriorityMempool(mempool.PriorityNonceMempoolConfig[int64]{
			TxPriority:      mempool.NewDefaultTxPriority(),
			SignerExtractor: mempool.NewDefaultSignerExtractionAdapter(),
			MaxTx:           100,
		})

		txDecoder := func(txBytes []byte) (sdk.Tx, error) {
			return &mockTx{memo: string(txBytes)}, nil
		}

		mpool := NewMempool(MempoolConfig{
			BaseMempool:   baseMpool,
			TxDecoder:     txDecoder,
			PriorityBoost: 1000000,
			Logger:        log.NewNopLogger(),
		})

		// Verify type
		typeName := mpool.GetBaseMempoolType()
		require.Equal(t, PriorityNonceMempoolType, typeName)

		// Should be recognized as PriorityNonceMempool
		require.True(t, mpool.IsPriorityNonceMempool(),
			"Should correctly identify PriorityNonceMempool as base")
	})
}

func TestMempool_InsertWithGasWanted(t *testing.T) {
	// Create SDK context with base priority
	basePriority := int64(100)
	ctx := sdk.Context{}.WithPriority(basePriority)

	baseMpool := newMockMempool()

	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: string(txBytes)}, nil
	}

	priorityBoost := int64(1000000)
	mpool := NewMempool(MempoolConfig{
		BaseMempool:   baseMpool,
		TxDecoder:     txDecoder,
		PriorityBoost: priorityBoost,
		Logger:        log.NewNopLogger(),
	})

	t.Run("Insert priority transaction with gas wanted and boosted priority", func(t *testing.T) {
		priorityTx := &mockTx{memo: "PRIORITY:1"}
		gasWanted := uint64(100000)
		err := mpool.InsertWithGasWanted(ctx, priorityTx, gasWanted)
		require.NoError(t, err)
		require.Equal(t, 1, mpool.CountTx())

		// Verify priority was boosted
		expectedPriority := basePriority + priorityBoost
		require.Equal(t, expectedPriority, baseMpool.lastInsertedPriority,
			"Priority transaction should have boosted priority")
		require.Equal(t, gasWanted, baseMpool.lastInsertedGasWanted,
			"Gas wanted should be passed through")
	})

	t.Run("Insert normal transaction with gas wanted and original priority", func(t *testing.T) {
		normalTx := &mockTx{memo: "normal"}
		gasWanted := uint64(50000)
		err := mpool.InsertWithGasWanted(ctx, normalTx, gasWanted)
		require.NoError(t, err)
		require.Equal(t, 2, mpool.CountTx())

		// Verify priority was NOT boosted
		require.Equal(t, basePriority, baseMpool.lastInsertedPriority,
			"Normal transaction should keep original priority")
		require.Equal(t, gasWanted, baseMpool.lastInsertedGasWanted,
			"Gas wanted should be passed through")
	})
}

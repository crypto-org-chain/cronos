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
	txs []sdk.Tx
}

func newMockMempool() *mockMempool {
	return &mockMempool{
		txs: make([]sdk.Tx, 0),
	}
}

func (mm *mockMempool) Insert(ctx context.Context, tx sdk.Tx) error {
	mm.txs = append(mm.txs, tx)
	return nil
}

func (mm *mockMempool) InsertWithGasWanted(ctx context.Context, tx sdk.Tx, gasWanted uint64) error {
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

	t.Run("Insert priority transaction", func(t *testing.T) {
		priorityTx := &mockTx{memo: "PRIORITY:1"}
		err := mpool.Insert(ctx, priorityTx)
		require.NoError(t, err)
		require.Equal(t, 1, mpool.CountTx())
	})

	t.Run("Insert normal transaction", func(t *testing.T) {
		normalTx := &mockTx{memo: "normal"}
		err := mpool.Insert(ctx, normalTx)
		require.NoError(t, err)
		require.Equal(t, 2, mpool.CountTx())
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

func TestPriorityTxWrapper(t *testing.T) {
	t.Run("PriorityTxWrapper returns boosted priority", func(t *testing.T) {
		baseTx := &mockTx{memo: "test"}
		boostedPriority := int64(1000100)

		wrapper := &PriorityTxWrapper{
			Tx:              baseTx,
			boostedPriority: boostedPriority,
		}

		require.Equal(t, boostedPriority, wrapper.GetPriority())
	})
}

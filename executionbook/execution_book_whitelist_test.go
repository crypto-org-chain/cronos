package executionbook

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

// mockSignerExtractor is a simple mock for testing
type mockSignerExtractor struct {
	signers []mempool.SignerData
	err     error
}

func (m *mockSignerExtractor) GetSigners(tx sdk.Tx) ([]mempool.SignerData, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.signers, nil
}

// Helper function to create AccAddress from Ethereum address string (0x...)
func ethAddrToAccAddress(ethAddr string) sdk.AccAddress {
	return sdk.AccAddress(common.HexToAddress(ethAddr).Bytes())
}

func TestMempool_Whitelist_DefaultBehavior(t *testing.T) {
	// Test that with empty whitelist, all addresses are allowed
	basePriority := int64(100)
	ctx := sdk.Context{}.WithPriority(basePriority)

	baseMpool := newMockMempool()
	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: string(txBytes)}, nil
	}

	priorityBoost := int64(1000000)
	testAddr := "0x1234567890123456789012345678901234567890"

	// Create mempool with no whitelist
	mpool := NewExecutionBook(ExecutionBookConfig{
		BaseMempool:   baseMpool,
		TxDecoder:     txDecoder,
		PriorityBoost: priorityBoost,
		Logger:        log.NewNopLogger(),
		SignerExtractor: &mockSignerExtractor{
			signers: []mempool.SignerData{
				{Signer: ethAddrToAccAddress(testAddr)},
			},
		},
	})

	t.Run("Empty whitelist allows all addresses", func(t *testing.T) {
		require.Equal(t, 0, mpool.WhitelistCount())

		priorityTx := &mockTx{memo: "PRIORITY:1"}
		err := mpool.Insert(ctx, priorityTx)
		require.NoError(t, err)

		// Priority should be boosted
		require.Equal(t, basePriority+priorityBoost, baseMpool.lastInsertedPriority)
	})
}

func TestMempool_Whitelist_AddRemove(t *testing.T) {
	baseMpool := newMockMempool()
	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: string(txBytes)}, nil
	}

	mpool := NewExecutionBook(ExecutionBookConfig{
		BaseMempool:   baseMpool,
		TxDecoder:     txDecoder,
		PriorityBoost: 1000000,
		Logger:        log.NewNopLogger(),
	})

	ethAddr1 := "0x1234567890123456789012345678901234567890"
	ethAddr2 := "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"
	ethAddr3 := "0x9876543210987654321098765432109876543210"

	t.Run("Add address to whitelist", func(t *testing.T) {
		mpool.AddToWhitelist(ethAddr1)
		require.Equal(t, 1, mpool.WhitelistCount())
		require.True(t, mpool.IsWhitelisted(ethAddr1))
	})

	t.Run("Add multiple addresses", func(t *testing.T) {
		mpool.AddToWhitelist(ethAddr2)
		mpool.AddToWhitelist(ethAddr3)
		require.Equal(t, 3, mpool.WhitelistCount())
		require.True(t, mpool.IsWhitelisted(ethAddr2))
		require.True(t, mpool.IsWhitelisted(ethAddr3))
	})

	t.Run("Remove address from whitelist", func(t *testing.T) {
		mpool.RemoveFromWhitelist(ethAddr2)
		require.Equal(t, 2, mpool.WhitelistCount())
		require.False(t, mpool.IsWhitelisted(ethAddr2))
		require.True(t, mpool.IsWhitelisted(ethAddr1))
	})

	t.Run("Get whitelist returns all addresses", func(t *testing.T) {
		whitelist := mpool.GetWhitelist()
		require.Len(t, whitelist, 2)
		require.Contains(t, whitelist, ethAddr1)
		require.Contains(t, whitelist, ethAddr3)
	})

	t.Run("Clear whitelist", func(t *testing.T) {
		mpool.ClearWhitelist()
		require.Equal(t, 0, mpool.WhitelistCount())
		require.False(t, mpool.IsWhitelisted(ethAddr1))
	})
}

func TestMempool_Whitelist_SetWhitelist(t *testing.T) {
	baseMpool := newMockMempool()
	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: string(txBytes)}, nil
	}

	mpool := NewExecutionBook(ExecutionBookConfig{
		BaseMempool:   baseMpool,
		TxDecoder:     txDecoder,
		PriorityBoost: 1000000,
		Logger:        log.NewNopLogger(),
	})

	ethAddrOld := "0x1111111111111111111111111111111111111111"
	ethAddrNew1 := "0x2222222222222222222222222222222222222222"
	ethAddrNew2 := "0x3333333333333333333333333333333333333333"

	t.Run("Set whitelist replaces existing", func(t *testing.T) {
		mpool.AddToWhitelist(ethAddrOld)
		require.True(t, mpool.IsWhitelisted(ethAddrOld))

		mpool.SetWhitelist([]string{ethAddrNew1, ethAddrNew2})
		require.Equal(t, 2, mpool.WhitelistCount())
		require.False(t, mpool.IsWhitelisted(ethAddrOld))
		require.True(t, mpool.IsWhitelisted(ethAddrNew1))
		require.True(t, mpool.IsWhitelisted(ethAddrNew2))
	})

	t.Run("Set empty whitelist", func(t *testing.T) {
		mpool.SetWhitelist([]string{})
		require.Equal(t, 0, mpool.WhitelistCount())
	})
}

func TestMempool_Whitelist_PriorityBoosting(t *testing.T) {
	basePriority := int64(100)
	ctx := sdk.Context{}.WithPriority(basePriority)
	priorityBoost := int64(1000000)

	whitelistedAddr := "0x1111111111111111111111111111111111111111"
	notWhitelistedAddr := "0x2222222222222222222222222222222222222222"

	t.Run("Whitelisted address gets priority boost", func(t *testing.T) {
		baseMpool := newMockMempool()
		txDecoder := func(txBytes []byte) (sdk.Tx, error) {
			return &mockTx{memo: string(txBytes)}, nil
		}

		mpool := NewExecutionBook(ExecutionBookConfig{
			BaseMempool:        baseMpool,
			TxDecoder:          txDecoder,
			PriorityBoost:      priorityBoost,
			Logger:             log.NewNopLogger(),
			WhitelistAddresses: []string{whitelistedAddr},
			SignerExtractor: &mockSignerExtractor{
				signers: []mempool.SignerData{
					{Signer: ethAddrToAccAddress(whitelistedAddr)},
				},
			},
		})

		priorityTx := &mockTx{memo: "PRIORITY:1"}
		err := mpool.Insert(ctx, priorityTx)
		require.NoError(t, err)

		// Priority should be boosted
		expectedPriority := basePriority + priorityBoost
		require.Equal(t, expectedPriority, baseMpool.lastInsertedPriority)
	})

	t.Run("Non-whitelisted address does NOT get priority boost", func(t *testing.T) {
		baseMpool := newMockMempool()
		txDecoder := func(txBytes []byte) (sdk.Tx, error) {
			return &mockTx{memo: string(txBytes)}, nil
		}

		mpool := NewExecutionBook(ExecutionBookConfig{
			BaseMempool:        baseMpool,
			TxDecoder:          txDecoder,
			PriorityBoost:      priorityBoost,
			Logger:             log.NewNopLogger(),
			WhitelistAddresses: []string{whitelistedAddr},
			SignerExtractor: &mockSignerExtractor{
				signers: []mempool.SignerData{
					{Signer: ethAddrToAccAddress(notWhitelistedAddr)},
				},
			},
		})

		priorityTx := &mockTx{memo: "PRIORITY:1"}
		err := mpool.Insert(ctx, priorityTx)
		require.NoError(t, err)

		// Priority should NOT be boosted
		require.Equal(t, basePriority, baseMpool.lastInsertedPriority)
	})

	t.Run("Normal transaction is not affected by whitelist", func(t *testing.T) {
		baseMpool := newMockMempool()
		txDecoder := func(txBytes []byte) (sdk.Tx, error) {
			return &mockTx{memo: string(txBytes)}, nil
		}

		mpool := NewExecutionBook(ExecutionBookConfig{
			BaseMempool:        baseMpool,
			TxDecoder:          txDecoder,
			PriorityBoost:      priorityBoost,
			Logger:             log.NewNopLogger(),
			WhitelistAddresses: []string{whitelistedAddr},
			SignerExtractor: &mockSignerExtractor{
				signers: []mempool.SignerData{
					{Signer: ethAddrToAccAddress(notWhitelistedAddr)},
				},
			},
		})

		normalTx := &mockTx{memo: "normal transaction"}
		err := mpool.Insert(ctx, normalTx)
		require.NoError(t, err)

		// Normal transaction keeps original priority
		require.Equal(t, basePriority, baseMpool.lastInsertedPriority)
	})
}

func TestMempool_Whitelist_DynamicUpdate(t *testing.T) {
	basePriority := int64(100)
	ctx := sdk.Context{}.WithPriority(basePriority)
	priorityBoost := int64(1000000)

	testAddr := "0x3333333333333333333333333333333333333333"
	otherAddr := "0x4444444444444444444444444444444444444444"

	t.Run("Adding address dynamically enables priority boost", func(t *testing.T) {
		baseMpool := newMockMempool()
		txDecoder := func(txBytes []byte) (sdk.Tx, error) {
			return &mockTx{memo: string(txBytes)}, nil
		}

		mpool := NewExecutionBook(ExecutionBookConfig{
			BaseMempool:   baseMpool,
			TxDecoder:     txDecoder,
			PriorityBoost: priorityBoost,
			Logger:        log.NewNopLogger(),
			SignerExtractor: &mockSignerExtractor{
				signers: []mempool.SignerData{
					{Signer: ethAddrToAccAddress(testAddr)},
				},
			},
		})

		// Initially empty whitelist - should allow boost
		priorityTx := &mockTx{memo: "PRIORITY:1"}
		err := mpool.Insert(ctx, priorityTx)
		require.NoError(t, err)
		require.Equal(t, basePriority+priorityBoost, baseMpool.lastInsertedPriority)

		// Add a different address to whitelist
		mpool.AddToWhitelist(otherAddr)

		// Now testAddr should be rejected
		baseMpool.lastInsertedPriority = 0 // Reset
		err = mpool.Insert(ctx, priorityTx)
		require.NoError(t, err)
		require.Equal(t, basePriority, baseMpool.lastInsertedPriority)

		// Add testAddr to whitelist
		mpool.AddToWhitelist(testAddr)

		// Now should be boosted again
		baseMpool.lastInsertedPriority = 0 // Reset
		err = mpool.Insert(ctx, priorityTx)
		require.NoError(t, err)
		require.Equal(t, basePriority+priorityBoost, baseMpool.lastInsertedPriority)
	})

	t.Run("Clearing whitelist re-enables boost for all", func(t *testing.T) {
		baseMpool := newMockMempool()
		txDecoder := func(txBytes []byte) (sdk.Tx, error) {
			return &mockTx{memo: string(txBytes)}, nil
		}

		mpool := NewExecutionBook(ExecutionBookConfig{
			BaseMempool:        baseMpool,
			TxDecoder:          txDecoder,
			PriorityBoost:      priorityBoost,
			Logger:             log.NewNopLogger(),
			WhitelistAddresses: []string{otherAddr},
			SignerExtractor: &mockSignerExtractor{
				signers: []mempool.SignerData{
					{Signer: ethAddrToAccAddress(testAddr)},
				},
			},
		})

		// testAddr not whitelisted - should not boost
		priorityTx := &mockTx{memo: "PRIORITY:1"}
		err := mpool.Insert(ctx, priorityTx)
		require.NoError(t, err)
		require.Equal(t, basePriority, baseMpool.lastInsertedPriority)

		// Clear whitelist
		mpool.ClearWhitelist()

		// Now should boost
		baseMpool.lastInsertedPriority = 0 // Reset
		err = mpool.Insert(ctx, priorityTx)
		require.NoError(t, err)
		require.Equal(t, basePriority+priorityBoost, baseMpool.lastInsertedPriority)
	})
}

func TestMempool_Whitelist_InitialConfiguration(t *testing.T) {
	baseMpool := newMockMempool()
	txDecoder := func(txBytes []byte) (sdk.Tx, error) {
		return &mockTx{memo: string(txBytes)}, nil
	}

	t.Run("Initialize with whitelist addresses", func(t *testing.T) {
		addresses := []string{
			"0x5555555555555555555555555555555555555555",
			"0x6666666666666666666666666666666666666666",
			"0x7777777777777777777777777777777777777777",
		}

		mpool := NewExecutionBook(ExecutionBookConfig{
			BaseMempool:        baseMpool,
			TxDecoder:          txDecoder,
			PriorityBoost:      1000000,
			Logger:             log.NewNopLogger(),
			WhitelistAddresses: addresses,
		})

		require.Equal(t, 3, mpool.WhitelistCount())
		for _, addr := range addresses {
			require.True(t, mpool.IsWhitelisted(addr))
		}
	})
}

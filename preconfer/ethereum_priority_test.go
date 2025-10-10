package preconfer

import (
	"testing"

	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// mockEthereumTx creates a mock Ethereum transaction for testing
type mockEthereumTx struct {
	sdk.Tx
	msgs []sdk.Msg
}

func (m *mockEthereumTx) GetMsgs() []sdk.Msg {
	return m.msgs
}

func TestIsMarkedPriorityTx_EthereumTx(t *testing.T) {
	t.Run("Ethereum tx with PRIORITY: memo", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "PRIORITY:5",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		result := IsMarkedPriorityTx(tx)
		require.True(t, result, "Ethereum tx with PRIORITY:5 should be marked as priority")
	})

	t.Run("Ethereum tx with HIGH_PRIORITY memo", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "HIGH_PRIORITY",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		result := IsMarkedPriorityTx(tx)
		require.True(t, result, "Ethereum tx with HIGH_PRIORITY should be marked as priority")
	})

	t.Run("Ethereum tx with URGENT memo", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "URGENT",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		result := IsMarkedPriorityTx(tx)
		require.True(t, result, "Ethereum tx with URGENT should be marked as priority")
	})

	t.Run("Ethereum tx with [PRIORITY] marker", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "Important transaction [PRIORITY]",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		result := IsMarkedPriorityTx(tx)
		require.True(t, result, "Ethereum tx with [PRIORITY] marker should be marked as priority")
	})

	t.Run("Ethereum tx without priority memo", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "normal transaction",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		result := IsMarkedPriorityTx(tx)
		require.False(t, result, "Ethereum tx without priority marker should not be marked as priority")
	})

	t.Run("Ethereum tx with empty memo", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		result := IsMarkedPriorityTx(tx)
		require.False(t, result, "Ethereum tx with empty memo should not be marked as priority")
	})

	t.Run("Ethereum tx with lowercase priority", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "priority:3",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		result := IsMarkedPriorityTx(tx)
		require.True(t, result, "Ethereum tx with lowercase priority should be recognized")
	})
}

func TestGetPriorityLevel_EthereumTx(t *testing.T) {
	t.Run("Extract priority level 1", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "PRIORITY:1",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		level := GetPriorityLevel(tx)
		require.Equal(t, 1, level)
	})

	t.Run("Extract priority level 5", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "PRIORITY:5",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		level := GetPriorityLevel(tx)
		require.Equal(t, 5, level)
	})

	t.Run("Extract priority level 10", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "PRIORITY:10",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		level := GetPriorityLevel(tx)
		require.Equal(t, 10, level)
	})

	t.Run("Invalid priority level (too high)", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "PRIORITY:20",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		level := GetPriorityLevel(tx)
		require.Equal(t, 1, level, "Invalid level should default to 1")
	})

	t.Run("Invalid priority level (zero)", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "PRIORITY:0",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		level := GetPriorityLevel(tx)
		require.Equal(t, 1, level, "Invalid level should default to 1")
	})

	t.Run("Priority without level", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "PRIORITY:",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		level := GetPriorityLevel(tx)
		require.Equal(t, 1, level, "Priority without level should default to 1")
	})

	t.Run("No priority marker", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "normal transaction",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		level := GetPriorityLevel(tx)
		require.Equal(t, 0, level, "No priority marker should return 0")
	})
}

func TestGetEthereumTxMemo(t *testing.T) {
	t.Run("Get memo from Ethereum tx", func(t *testing.T) {
		expectedMemo := "PRIORITY:5"
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: expectedMemo,
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		memo := GetEthereumTxMemo(tx)
		require.Equal(t, expectedMemo, memo)
	})

	t.Run("Get empty memo", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		memo := GetEthereumTxMemo(tx)
		require.Equal(t, "", memo)
	})

	t.Run("No Ethereum tx in transaction", func(t *testing.T) {
		tx := &mockEthereumTx{msgs: []sdk.Msg{}}

		memo := GetEthereumTxMemo(tx)
		require.Equal(t, "", memo)
	})

	t.Run("Nil transaction", func(t *testing.T) {
		memo := GetEthereumTxMemo(nil)
		require.Equal(t, "", memo)
	})
}

func TestIsEthereumTx(t *testing.T) {
	t.Run("Transaction with Ethereum message", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		result := IsEthereumTx(tx)
		require.True(t, result)
	})

	t.Run("Transaction without Ethereum message", func(t *testing.T) {
		tx := &mockEthereumTx{msgs: []sdk.Msg{}}

		result := IsEthereumTx(tx)
		require.False(t, result)
	})

	t.Run("Nil transaction", func(t *testing.T) {
		result := IsEthereumTx(nil)
		require.False(t, result)
	})
}

func TestGetTransactionType(t *testing.T) {
	t.Run("Ethereum transaction", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		txType := GetTransactionType(tx)
		require.Equal(t, "ethereum", txType)
	})

	t.Run("Empty transaction", func(t *testing.T) {
		tx := &mockEthereumTx{msgs: []sdk.Msg{}}

		txType := GetTransactionType(tx)
		require.Equal(t, "empty", txType)
	})

	t.Run("Nil transaction", func(t *testing.T) {
		txType := GetTransactionType(nil)
		require.Equal(t, "unknown", txType)
	})
}

func TestGetEthereumTxInfo(t *testing.T) {
	t.Run("Ethereum tx with priority memo", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "PRIORITY:5",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		info := GetEthereumTxInfo(tx)
		require.True(t, info.HasEthereumTx)
		require.Equal(t, 1, info.EthereumTxCount)
		require.True(t, info.HasPriorityMemo)
		require.Equal(t, 5, info.PriorityLevel)
		require.Equal(t, "PRIORITY:5", info.Memo)
	})

	t.Run("Ethereum tx without priority memo", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "normal",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		info := GetEthereumTxInfo(tx)
		require.True(t, info.HasEthereumTx)
		require.Equal(t, 1, info.EthereumTxCount)
		require.False(t, info.HasPriorityMemo)
		require.Equal(t, 0, info.PriorityLevel)
		require.Equal(t, "normal", info.Memo)
	})

	t.Run("No Ethereum tx", func(t *testing.T) {
		tx := &mockEthereumTx{msgs: []sdk.Msg{}}

		info := GetEthereumTxInfo(tx)
		require.False(t, info.HasEthereumTx)
		require.Equal(t, 0, info.EthereumTxCount)
		require.False(t, info.HasPriorityMemo)
	})
}

func TestValidateEthereumTxMemo(t *testing.T) {
	t.Run("Valid short memo", func(t *testing.T) {
		err := ValidateEthereumTxMemo("PRIORITY:5")
		require.NoError(t, err)
	})

	t.Run("Valid long memo", func(t *testing.T) {
		memo := make([]byte, 256)
		for i := range memo {
			memo[i] = 'a'
		}
		err := ValidateEthereumTxMemo(string(memo))
		require.NoError(t, err)
	})

	t.Run("Memo too long", func(t *testing.T) {
		memo := make([]byte, 257)
		for i := range memo {
			memo[i] = 'a'
		}
		err := ValidateEthereumTxMemo(string(memo))
		require.Error(t, err)
		require.Contains(t, err.Error(), "memo too long")
	})

	t.Run("Invalid priority level", func(t *testing.T) {
		err := ValidateEthereumTxMemo("PRIORITY:20")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid priority level")
	})

	t.Run("Empty memo", func(t *testing.T) {
		err := ValidateEthereumTxMemo("")
		require.NoError(t, err)
	})
}

func TestCalculateBoostedPriority_EthereumTx(t *testing.T) {
	t.Run("Ethereum tx with priority gets boost", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "PRIORITY:5",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		basePriority := int64(100)
		maxBoost := int64(1000000)

		boostedPriority := CalculateBoostedPriority(tx, basePriority, maxBoost)
		expectedBoost := basePriority + (maxBoost * 5 / 10)

		require.Equal(t, expectedBoost, boostedPriority)
	})

	t.Run("Ethereum tx without priority no boost", func(t *testing.T) {
		ethMsg := &evmtypes.MsgEthereumTx{
			Memo: "normal",
		}
		tx := &mockEthereumTx{msgs: []sdk.Msg{ethMsg}}

		basePriority := int64(100)
		maxBoost := int64(1000000)

		boostedPriority := CalculateBoostedPriority(tx, basePriority, maxBoost)

		require.Equal(t, basePriority, boostedPriority)
	})
}

package preconfer

import (
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestIsMarkedPriorityTx(t *testing.T) {
	tests := []struct {
		name     string
		tx       sdk.Tx
		expected bool
	}{
		{
			name:     "nil transaction",
			tx:       nil,
			expected: false,
		},
		{
			name:     "transaction with PRIORITY: marker",
			tx:       &mockTx{memo: "PRIORITY:1"},
			expected: true,
		},
		{
			name:     "transaction without priority marker",
			tx:       &mockTx{memo: "regular transaction"},
			expected: false,
		},
		{
			name:     "transaction with lowercase priority (converted to uppercase)",
			tx:       &mockTx{memo: "priority:1"},
			expected: true,
		},
		{
			name:     "empty memo",
			tx:       &mockTx{memo: ""},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMarkedPriorityTx(tt.tx)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetPriorityLevel(t *testing.T) {
	tests := []struct {
		name     string
		tx       sdk.Tx
		expected int
	}{
		{
			name:     "nil transaction",
			tx:       nil,
			expected: 0,
		},
		{
			name:     "priority level 1",
			tx:       &mockTx{memo: "PRIORITY:1"},
			expected: 1,
		},
		{
			name:     "priority level 10 (invalid, returns 1)",
			tx:       &mockTx{memo: "PRIORITY:10"},
			expected: 1,
		},
		{
			name:     "priority without level",
			tx:       &mockTx{memo: "PRIORITY:"},
			expected: 1, // Default level
		},
		{
			name:     "priority with invalid level",
			tx:       &mockTx{memo: "PRIORITY:abc"},
			expected: 1, // Default level
		},
		{
			name:     "priority level out of range (not 1)",
			tx:       &mockTx{memo: "PRIORITY:2"},
			expected: 1, // Default level
		},
		{
			name:     "priority level out of range (too low)",
			tx:       &mockTx{memo: "PRIORITY:0"},
			expected: 1, // Default level
		},
		{
			name:     "no priority marker",
			tx:       &mockTx{memo: "normal transaction"},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPriorityLevel(tt.tx)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateBoostedPriority(t *testing.T) {
	const maxBoost int64 = 1000000

	tests := []struct {
		name         string
		tx           sdk.Tx
		basePriority int64
		expected     int64
	}{
		{
			name:         "normal transaction (no boost)",
			tx:           &mockTx{memo: "normal"},
			basePriority: 100,
			expected:     100,
		},
		{
			name:         "priority level 1",
			tx:           &mockTx{memo: "PRIORITY:1"},
			basePriority: 100,
			expected:     100 + maxBoost,
		},
		{
			name:         "priority level 10 (invalid, treated as level 1)",
			tx:           &mockTx{memo: "PRIORITY:10"},
			basePriority: 100,
			expected:     100 + maxBoost,
		},
		{
			name:         "priority without level (default level 1)",
			tx:           &mockTx{memo: "PRIORITY:"},
			basePriority: 100,
			expected:     100 + maxBoost,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateBoostedPriority(tt.tx, tt.basePriority, maxBoost)
			require.Equal(t, tt.expected, result)
		})
	}
}

// Note: Priority handling is now done at the TxSelector level
// These tests verify the helper functions used by PriorityTxSelector

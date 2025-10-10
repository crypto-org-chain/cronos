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
			name:     "transaction with HIGH_PRIORITY marker",
			tx:       &mockTx{memo: "HIGH_PRIORITY"},
			expected: true,
		},
		{
			name:     "transaction with URGENT marker",
			tx:       &mockTx{memo: "URGENT"},
			expected: true,
		},
		{
			name:     "transaction with [PRIORITY] marker",
			tx:       &mockTx{memo: "some text [PRIORITY] more text"},
			expected: true,
		},
		{
			name:     "transaction with [HIGH_PRIORITY] marker",
			tx:       &mockTx{memo: "some text [HIGH_PRIORITY] more text"},
			expected: true,
		},
		{
			name:     "transaction without priority marker",
			tx:       &mockTx{memo: "regular transaction"},
			expected: false,
		},
		{
			name:     "transaction with lowercase priority (should work)",
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
			name:     "priority level 5",
			tx:       &mockTx{memo: "PRIORITY:5"},
			expected: 5,
		},
		{
			name:     "priority level 10",
			tx:       &mockTx{memo: "PRIORITY:10"},
			expected: 10,
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
			name:     "priority level out of range (too high)",
			tx:       &mockTx{memo: "PRIORITY:20"},
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
			expected:     100 + (maxBoost * 1 / 10),
		},
		{
			name:         "priority level 5",
			tx:           &mockTx{memo: "PRIORITY:5"},
			basePriority: 100,
			expected:     100 + (maxBoost * 5 / 10),
		},
		{
			name:         "priority level 10 (maximum)",
			tx:           &mockTx{memo: "PRIORITY:10"},
			basePriority: 100,
			expected:     100 + maxBoost,
		},
		{
			name:         "priority without level (default level 1)",
			tx:           &mockTx{memo: "PRIORITY:"},
			basePriority: 100,
			expected:     100 + (maxBoost * 1 / 10),
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

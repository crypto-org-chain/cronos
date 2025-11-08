//go:build !rocksdb
// +build !rocksdb

package dbmigrate

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIncrementBytes tests the byte slice increment helper
func TestIncrementBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "simple_increment",
			input:    []byte{0x01, 0x02, 0x03},
			expected: []byte{0x01, 0x02, 0x04},
		},
		{
			name:     "carry_over",
			input:    []byte{0x01, 0x02, 0xFF},
			expected: []byte{0x01, 0x03, 0x00},
		},
		{
			name:     "all_ff",
			input:    []byte{0xFF, 0xFF, 0xFF},
			expected: []byte{0x01, 0x00, 0x00, 0x00},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := incrementBytes(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestFormatKeyPrefix tests the key prefix formatting helper
func TestFormatKeyPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		maxLen   int
		contains string
	}{
		{
			name:     "ascii_text",
			input:    []byte("test-key-123"),
			maxLen:   20,
			contains: "test-key-123",
		},
		{
			name:     "binary_data",
			input:    []byte{0x01, 0x02, 0xFF, 0xFE},
			maxLen:   20,
			contains: "0x",
		},
		{
			name:     "truncated",
			input:    []byte("this is a very long key that should be truncated"),
			maxLen:   10,
			contains: "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatKeyPrefix(tt.input, tt.maxLen)
			require.Contains(t, result, tt.contains)
		})
	}
}

// TestFormatValue tests the value formatting helper
func TestFormatValue(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		maxLen   int
		contains string
	}{
		{
			name:     "ascii_text",
			input:    []byte("test value"),
			maxLen:   20,
			contains: "test value",
		},
		{
			name:     "binary_data",
			input:    []byte{0x01, 0x02, 0xFF, 0xFE},
			maxLen:   20,
			contains: "0x",
		},
		{
			name:     "empty_value",
			input:    []byte{},
			maxLen:   20,
			contains: "<empty>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValue(tt.input, tt.maxLen)
			require.Contains(t, result, tt.contains)
		})
	}
}

package config

import (
	"math"
	"testing"
)

func TestDeriveTxCacheSize(t *testing.T) {
	testCases := []struct {
		name          string
		txsPerBlock   int
		mempoolMaxTxs int
		expected      int
	}{
		{
			name:          "bounded pool: max-txs floor plus one in-flight block",
			txsPerBlock:   1000,
			mempoolMaxTxs: 5000,
			expected:      6000,
		},
		{
			name:          "bounded pool, txsPerBlock > max-txs",
			txsPerBlock:   2900,
			mempoolMaxTxs: 5000,
			expected:      7900,
		},
		{
			name:          "unlimited txsPerBlock, bounded max-txs",
			txsPerBlock:   0,
			mempoolMaxTxs: 10000,
			expected:      10000,
		},
		{
			name:          "both unlimited",
			txsPerBlock:   0,
			mempoolMaxTxs: 0,
			expected:      -1,
		},
		{
			name:          "mempool disabled (negative max-txs), txsPerBlock set",
			txsPerBlock:   2900,
			mempoolMaxTxs: -1,
			expected:      5800,
		},
		{
			name:          "mempool disabled (negative max-txs), txsPerBlock unlimited",
			txsPerBlock:   0,
			mempoolMaxTxs: -1,
			expected:      -1,
		},
		{
			name:          "overflow guard: mempoolMaxTxs+txsPerBlock wraps",
			txsPerBlock:   1000,
			mempoolMaxTxs: math.MaxInt,
			expected:      MaxDerivedTxCacheSize,
		},
		{
			name:          "absurd max-txs capped",
			txsPerBlock:   2900,
			mempoolMaxTxs: 50_000_000,
			expected:      MaxDerivedTxCacheSize,
		},
		{
			name:          "at the cap, no clamp surprise",
			txsPerBlock:   0,
			mempoolMaxTxs: MaxDerivedTxCacheSize,
			expected:      MaxDerivedTxCacheSize,
		},
		{
			name:          "sum exactly at cap, no clamp",
			txsPerBlock:   1,
			mempoolMaxTxs: MaxDerivedTxCacheSize - 1,
			expected:      MaxDerivedTxCacheSize,
		},
		{
			name:          "default config: unlimited max-txs, default txsPerBlock",
			txsPerBlock:   DefaultMempoolTxsPerBlock,
			mempoolMaxTxs: 0,
			expected:      DefaultTxCacheSize,
		},
		{
			name:          "overflow guard: 2x txsPerBlock wraps",
			txsPerBlock:   math.MaxInt,
			mempoolMaxTxs: 0,
			expected:      MaxDerivedTxCacheSize,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveTxCacheSize(tc.txsPerBlock, tc.mempoolMaxTxs)
			if got != tc.expected {
				t.Errorf("DeriveTxCacheSize(%d, %d) = %d, want %d", tc.txsPerBlock, tc.mempoolMaxTxs, got, tc.expected)
			}
		})
	}
}

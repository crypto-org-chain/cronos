package app

import "testing"

func TestDeriveTxCacheSize(t *testing.T) {
	cases := []struct {
		name          string
		mempoolMaxTxs int
		want          int
	}{
		{"bounded", 5000, 5000},
		{"unbounded", 0, -1},
		{"disabled", -1, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveTxCacheSize(tc.mempoolMaxTxs); got != tc.want {
				t.Fatalf("deriveTxCacheSize(%d) = %d, want %d", tc.mempoolMaxTxs, got, tc.want)
			}
		})
	}
}

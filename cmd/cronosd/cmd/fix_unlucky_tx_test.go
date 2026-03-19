package cmd

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
)

func TestTxExceedsBlockGasLimit(t *testing.T) {
	tests := []struct {
		name   string
		result *abci.ExecTxResult
		want   bool
	}{
		{
			name: "exceeds block gas limit",
			result: &abci.ExecTxResult{
				Code: 11,
				Log:  "out of gas in location: block gas meter; gasWanted: 100000",
			},
			want: true,
		},
		{
			name: "different error code",
			result: &abci.ExecTxResult{
				Code: 5,
				Log:  "out of gas in location: block gas meter; gasWanted: 100000",
			},
			want: false,
		},
		{
			name: "different error message",
			result: &abci.ExecTxResult{
				Code: 11,
				Log:  "some other error",
			},
			want: false,
		},
		{
			name: "success tx",
			result: &abci.ExecTxResult{
				Code: 0,
				Log:  "",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TxExceedsBlockGasLimit(tt.result)
			if got != tt.want {
				t.Errorf("TxExceedsBlockGasLimit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsBlockHeightAllowed(t *testing.T) {
	const defaultMinHeight = 2693800

	tests := []struct {
		name           string
		blockNumber    int64
		minBlockHeight int
		want           bool
	}{
		{
			name:           "block 6541 bypasses min height guard",
			blockNumber:    6541,
			minBlockHeight: defaultMinHeight,
			want:           true,
		},
		{
			name:           "block above min height is allowed",
			blockNumber:    2693900,
			minBlockHeight: defaultMinHeight,
			want:           true,
		},
		{
			name:           "block at exact min height is allowed",
			blockNumber:    defaultMinHeight,
			minBlockHeight: defaultMinHeight,
			want:           true,
		},
		{
			name:           "block below min height is rejected",
			blockNumber:    100,
			minBlockHeight: defaultMinHeight,
			want:           false,
		},
		{
			name:           "block 6540 is not an exception",
			blockNumber:    6540,
			minBlockHeight: defaultMinHeight,
			want:           false,
		},
		{
			name:           "block 6542 is not an exception",
			blockNumber:    6542,
			minBlockHeight: defaultMinHeight,
			want:           false,
		},
		{
			name:           "block 1 is rejected",
			blockNumber:    1,
			minBlockHeight: defaultMinHeight,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBlockHeightAllowed(tt.blockNumber, tt.minBlockHeight)
			if got != tt.want {
				t.Errorf("isBlockHeightAllowed(%d, %d) = %v, want %v",
					tt.blockNumber, tt.minBlockHeight, got, tt.want)
			}
		})
	}
}

func TestKnownPreUpgradeUnluckyBlocks(t *testing.T) {
	if _, ok := knownPreUpgradeUnluckyBlocks[6541]; !ok {
		t.Error("block 6541 should be in knownPreUpgradeUnluckyBlocks")
	}
}

func TestTxIndexStartsAtZeroForFirstUnluckyTx(t *testing.T) {
	// Simulates the txIndex logic in FixUnluckyTxCmd: when the unlucky tx is
	// the first (and only) tx in a block, ethTxIndex should be 0, not 1.
	tests := []struct {
		name             string
		precedingTxIndex int64 // -1 means no preceding tx
		msgCount         int
		wantEthTxIndices []int64
	}{
		{
			name:             "unlucky tx is first in block (block 6541 scenario)",
			precedingTxIndex: -1,
			msgCount:         1,
			wantEthTxIndices: []int64{0},
		},
		{
			name:             "one preceding successful tx with index 0",
			precedingTxIndex: 0,
			msgCount:         1,
			wantEthTxIndices: []int64{1},
		},
		{
			name:             "preceding tx with index 4, unlucky tx has 2 messages",
			precedingTxIndex: 4,
			msgCount:         2,
			wantEthTxIndices: []int64{5, 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txIndex := tt.precedingTxIndex
			txIndex++
			for msgIndex := 0; msgIndex < tt.msgCount; msgIndex++ {
				ethTxIndex := txIndex + int64(msgIndex)
				if ethTxIndex != tt.wantEthTxIndices[msgIndex] {
					t.Errorf("msgIndex=%d: ethTxIndex = %d, want %d",
						msgIndex, ethTxIndex, tt.wantEthTxIndices[msgIndex])
				}
			}
		})
	}
}

func TestAlreadyPatchedDetection(t *testing.T) {
	// When the last event is already ethereum_tx, the tx should be skipped.
	txResult := &abci.ExecTxResult{
		Code: 11,
		Log:  "out of gas in location: block gas meter; gasWanted: 100000",
		Events: []abci.Event{
			{Type: evmtypes.TypeMsgEthereumTx},
		},
	}

	if !TxExceedsBlockGasLimit(txResult) {
		t.Fatal("expected tx to exceed block gas limit")
	}

	lastEvt := txResult.Events[len(txResult.Events)-1]
	if lastEvt.Type != evmtypes.TypeMsgEthereumTx {
		t.Error("expected already-patched detection to find ethereum_tx event")
	}
}

package preconfirmation

import (
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
)

// Priority transaction constants
const (
	// Default priority boost for marked transactions
	DefaultPriorityBoost int64 = 1_000_000

	// Alternative priority markers
	PriorityTxMarker1 = "PRIORITY:"
	PriorityTxMarker2 = "HIGH_PRIORITY"
	PriorityTxMarker3 = "URGENT"

	// PriorityTxPrefix is the prefix in tx memo to mark a transaction as high priority
	PriorityTxPrefix = "PRIORITY:"
)

// Note: Priority boost for marked transactions is handled at the TxSelector level
// See PriorityTxSelector which reorders transactions to put marked priority txs first

// IsMarkedPriorityTx checks if a transaction is marked as priority
// It checks the memo field for priority markers
// This supports both standard Cosmos transactions and Ethereum transactions
func IsMarkedPriorityTx(tx sdk.Tx) bool {
	if tx == nil {
		return false
	}

	// Method 1: Check if tx implements TxWithMemo interface (standard Cosmos tx)
	if txWithMemo, ok := tx.(sdk.TxWithMemo); ok {
		memo := strings.ToUpper(strings.TrimSpace(txWithMemo.GetMemo()))
		if hasPriorityMarker(memo) {
			return true
		}
	}

	// Method 2: Check Ethereum tx messages for memo field
	// This handles MsgEthereumTx which now has memo field in forked ethermint
	if hasEthereumPriorityMemo(tx) {
		return true
	}

	return false
}

// hasPriorityMarker checks if a memo string contains priority markers
func hasPriorityMarker(memo string) bool {
	return strings.HasPrefix(memo, PriorityTxMarker1) ||
		strings.HasPrefix(memo, PriorityTxMarker2) ||
		strings.HasPrefix(memo, PriorityTxMarker3) ||
		strings.Contains(memo, "[PRIORITY]") ||
		strings.Contains(memo, "[HIGH_PRIORITY]")
}

// hasEthereumPriorityMemo checks if any Ethereum tx in the transaction has priority memo
func hasEthereumPriorityMemo(tx sdk.Tx) bool {
	msgs := tx.GetMsgs()
	for _, msg := range msgs {
		if ethMsg, ok := msg.(*evmtypes.MsgEthereumTx); ok {
			// Access the memo field that was added to MsgEthereumTx in forked ethermint
			if ethMsg.Memo != "" {
				memo := strings.ToUpper(strings.TrimSpace(ethMsg.Memo))
				if hasPriorityMarker(memo) {
					return true
				}
			}
		}
	}
	return false
}

// GetPriorityLevel extracts the priority level from memo if specified
// Format: "PRIORITY:LEVEL" where LEVEL is a number (1-10)
// Returns 0 if not specified or invalid
// This supports both standard Cosmos transactions and Ethereum transactions
func GetPriorityLevel(tx sdk.Tx) int {
	if tx == nil {
		return 0
	}

	// Method 1: Try to get from TxWithMemo interface (standard Cosmos tx)
	if txWithMemo, ok := tx.(sdk.TxWithMemo); ok {
		memo := txWithMemo.GetMemo()
		if level := extractLevelFromMemo(memo); level > 0 {
			return level
		}
	}

	// Method 2: Try to get from Ethereum tx memo field
	level := getEthereumTxPriorityLevel(tx)
	if level > 0 {
		return level
	}

	return 0
}

// extractLevelFromMemo extracts priority level from memo string
func extractLevelFromMemo(memo string) int {
	if !strings.HasPrefix(memo, PriorityTxPrefix) {
		return 0
	}

	// Extract level after the prefix
	levelStr := strings.TrimPrefix(memo, PriorityTxPrefix)
	levelStr = strings.TrimSpace(levelStr)

	// Parse level (1-10)
	var level int
	_, err := fmt.Sscanf(levelStr, "%d", &level)
	if err != nil || level < 1 || level > 10 {
		return 1 // Default level if not specified or invalid
	}

	return level
}

// getEthereumTxPriorityLevel extracts priority level from Ethereum tx memo
func getEthereumTxPriorityLevel(tx sdk.Tx) int {
	msgs := tx.GetMsgs()
	for _, msg := range msgs {
		if ethMsg, ok := msg.(*evmtypes.MsgEthereumTx); ok {
			if ethMsg.Memo != "" {
				level := extractLevelFromMemo(ethMsg.Memo)
				if level > 0 {
					return level
				}
			}
		}
	}
	return 0
}

// CalculateBoostedPriority calculates the final priority for a transaction
// considering both base priority and any priority markers
func CalculateBoostedPriority(tx sdk.Tx, basePriority int64, maxBoost int64) int64 {
	if !IsMarkedPriorityTx(tx) {
		return basePriority
	}

	level := GetPriorityLevel(tx)
	if level == 0 {
		level = 1
	}

	// Calculate boost based on level (1-10)
	boost := (maxBoost * int64(level)) / 10

	return basePriority + boost
}

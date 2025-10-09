package preconfirmation

import (
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
)

// GetEthereumTxMemo extracts memo from the first Ethereum transaction in a tx
// Returns empty string if no Ethereum tx is found or memo is empty
func GetEthereumTxMemo(tx sdk.Tx) string {
	if tx == nil {
		return ""
	}

	msgs := tx.GetMsgs()
	for _, msg := range msgs {
		if ethMsg, ok := msg.(*evmtypes.MsgEthereumTx); ok {
			return ethMsg.Memo
		}
	}
	return ""
}

// SetEthereumTxMemo sets memo for all Ethereum transactions in a tx
// This is useful for testing or transaction construction
func SetEthereumTxMemo(tx sdk.Tx, memo string) error {
	if tx == nil {
		return nil
	}

	msgs := tx.GetMsgs()
	for _, msg := range msgs {
		if ethMsg, ok := msg.(*evmtypes.MsgEthereumTx); ok {
			ethMsg.Memo = memo
		}
	}
	return nil
}

// IsEthereumTx checks if a transaction contains any Ethereum messages
func IsEthereumTx(tx sdk.Tx) bool {
	if tx == nil {
		return false
	}

	msgs := tx.GetMsgs()
	for _, msg := range msgs {
		if _, ok := msg.(*evmtypes.MsgEthereumTx); ok {
			return true
		}
	}
	return false
}

// GetTransactionType returns a string describing the transaction type
// Useful for logging and monitoring
func GetTransactionType(tx sdk.Tx) string {
	if tx == nil {
		return "unknown"
	}

	msgs := tx.GetMsgs()
	if len(msgs) == 0 {
		return "empty"
	}

	// Check if it's an Ethereum tx
	hasEthTx := false
	hasOtherTx := false

	for _, msg := range msgs {
		if _, ok := msg.(*evmtypes.MsgEthereumTx); ok {
			hasEthTx = true
		} else {
			hasOtherTx = true
		}
	}

	if hasEthTx && !hasOtherTx {
		return "ethereum"
	} else if !hasEthTx && hasOtherTx {
		return "cosmos"
	} else if hasEthTx && hasOtherTx {
		return "mixed"
	}

	return "unknown"
}

// GetEthereumTxInfo returns detailed information about Ethereum transactions
// Useful for debugging and monitoring
type EthereumTxInfo struct {
	HasEthereumTx   bool
	EthereumTxCount int
	HasPriorityMemo bool
	PriorityLevel   int
	Memo            string
}

// GetEthereumTxInfo extracts Ethereum transaction information from a tx
func GetEthereumTxInfo(tx sdk.Tx) *EthereumTxInfo {
	info := &EthereumTxInfo{}

	if tx == nil {
		return info
	}

	msgs := tx.GetMsgs()
	for _, msg := range msgs {
		if ethMsg, ok := msg.(*evmtypes.MsgEthereumTx); ok {
			info.HasEthereumTx = true
			info.EthereumTxCount++

			if ethMsg.Memo != "" && info.Memo == "" {
				info.Memo = ethMsg.Memo
				memo := strings.ToUpper(strings.TrimSpace(ethMsg.Memo))
				if hasPriorityMarker(memo) {
					info.HasPriorityMemo = true
					info.PriorityLevel = extractLevelFromMemo(ethMsg.Memo)
				}
			}
		}
	}

	return info
}

// ValidateEthereumTxMemo validates that the memo in Ethereum tx is properly formatted
// Returns nil if valid, error otherwise
func ValidateEthereumTxMemo(memo string) error {
	// Check memo length
	if len(memo) > 256 {
		return fmt.Errorf("memo too long: %d bytes (max 256)", len(memo))
	}

	// If memo has priority marker, validate the level explicitly
	if strings.HasPrefix(memo, PriorityTxPrefix) {
		levelStr := strings.TrimPrefix(memo, PriorityTxPrefix)
		levelStr = strings.TrimSpace(levelStr)

		if levelStr != "" {
			var level int
			_, err := fmt.Sscanf(levelStr, "%d", &level)
			if err != nil {
				return fmt.Errorf("invalid priority level format in memo: %s", memo)
			}
			if level < 1 || level > 10 {
				return fmt.Errorf("invalid priority level in memo: %s (must be 1-10)", memo)
			}
		}
	}

	return nil
}

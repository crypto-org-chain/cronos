package preconfer

import (
	"context"
	"strings"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ baseapp.TxSelector = &PriorityTxSelector{}

// PriorityTxSelector extends a baseapp.TxSelector with priority transaction support
// It prioritizes transactions marked with special prefix in memo field
type PriorityTxSelector struct {
	baseapp.TxSelector
	TxDecoder  sdk.TxDecoder
	ValidateTx func(sdk.Tx, []byte) error
}

// NewPriorityTxSelector creates a new priority transaction selector
func NewPriorityTxSelector(parent baseapp.TxSelector, txDecoder sdk.TxDecoder, validateTx func(sdk.Tx, []byte) error) *PriorityTxSelector {
	return &PriorityTxSelector{
		TxSelector: parent,
		TxDecoder:  txDecoder,
		ValidateTx: validateTx,
	}
}

// IsPriorityTx checks if a transaction is marked as priority
func IsPriorityTx(tx sdk.Tx) bool {
	if tx == nil {
		return false
	}

	txWithMemo, ok := tx.(sdk.TxWithMemo)
	if !ok {
		return false
	}

	memo := txWithMemo.GetMemo()
	return strings.HasPrefix(memo, PriorityTxPrefix)
}

// IsPriorityTxBytes checks if a transaction bytes is marked as priority
func (pts *PriorityTxSelector) IsPriorityTxBytes(txBz []byte) bool {
	tx, err := pts.TxDecoder(txBz)
	if err != nil {
		return false
	}
	return IsPriorityTx(tx)
}

// SelectTxForProposal extends the parent implementation to handle priority txs
// Priority transactions are always selected first if they pass validation
func (pts *PriorityTxSelector) SelectTxForProposal(ctx context.Context, maxTxBytes, maxBlockGas uint64, memTx sdk.Tx, txBz []byte, gasWanted uint64) bool {
	// First validate the transaction
	if err := pts.ValidateTx(memTx, txBz); err != nil {
		return false
	}

	// Check if it's a priority transaction
	var isPriority bool
	if memTx != nil {
		isPriority = IsPriorityTx(memTx)
	} else {
		tx, err := pts.TxDecoder(txBz)
		if err == nil {
			isPriority = IsPriorityTx(tx)
		}
	}

	// Priority transactions are selected with more lenient gas limits
	// (but still respecting the max block gas)
	if isPriority {
		// Check basic constraints for priority tx
		if uint64(len(txBz)) > maxTxBytes {
			return false
		}
		// For priority tx, we don't pass memTx to avoid strict gas checking
		return pts.TxSelector.SelectTxForProposal(ctx, maxTxBytes, maxBlockGas, nil, txBz, gasWanted)
	}

	// Non-priority transactions use standard selection logic
	return pts.TxSelector.SelectTxForProposal(ctx, maxTxBytes, maxBlockGas, nil, txBz, gasWanted)
}

// SelectTxForProposalFast extends the parent implementation to prioritize marked txs
// Priority transactions are moved to the front of the list after filtering
func (pts *PriorityTxSelector) SelectTxForProposalFast(ctx context.Context, txs [][]byte) [][]byte {
	// First, filter valid transactions
	var validTxs [][]byte
	for _, txBz := range txs {
		tx, err := pts.TxDecoder(txBz)
		if err != nil {
			continue
		}
		if pts.ValidateTx != nil {
			if err := pts.ValidateTx(tx, txBz); err != nil {
				continue
			}
		}
		validTxs = append(validTxs, txBz)
	}

	if len(validTxs) == 0 {
		return validTxs
	}

	// Separate priority and normal transactions
	var priorityTxs [][]byte
	var normalTxs [][]byte

	for _, txBz := range validTxs {
		if pts.IsPriorityTxBytes(txBz) {
			priorityTxs = append(priorityTxs, txBz)
		} else {
			normalTxs = append(normalTxs, txBz)
		}
	}

	// Combine with priority txs first
	result := make([][]byte, 0, len(validTxs))
	result = append(result, priorityTxs...)
	result = append(result, normalTxs...)

	return result
}

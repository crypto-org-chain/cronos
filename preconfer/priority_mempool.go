package preconfer

import (
	"context"
	"fmt"

	"cosmossdk.io/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

var _ mempool.Mempool = &EnhancedPriorityMempool{}

// EnhancedPriorityMempool wraps the standard PriorityMempool and enhances it
// to give marked transactions higher priority
type EnhancedPriorityMempool struct {
	mempool.Mempool
	txDecoder sdk.TxDecoder
	logger    log.Logger

	// Priority boost for marked transactions
	// This value is added to the transaction's base priority
	priorityBoost int64
}

// EnhancedPriorityMempoolConfig configuration for the enhanced mempool
type EnhancedPriorityMempoolConfig struct {
	// BaseMempool is the underlying mempool implementation
	BaseMempool mempool.Mempool

	// TxDecoder for decoding transaction bytes
	TxDecoder sdk.TxDecoder

	// PriorityBoost is the priority increase for marked transactions
	// Default is 1000000 if not specified
	PriorityBoost int64

	// Logger for mempool operations
	Logger log.Logger
}

// NewEnhancedPriorityMempool creates a new enhanced priority mempool
func NewEnhancedPriorityMempool(cfg EnhancedPriorityMempoolConfig) *EnhancedPriorityMempool {
	if cfg.PriorityBoost == 0 {
		cfg.PriorityBoost = 1000000 // Default high priority boost
	}

	if cfg.Logger == nil {
		cfg.Logger = log.NewNopLogger()
	}

	return &EnhancedPriorityMempool{
		Mempool:       cfg.BaseMempool,
		txDecoder:     cfg.TxDecoder,
		logger:        cfg.Logger,
		priorityBoost: cfg.PriorityBoost,
	}
}

// PriorityTxWrapper wraps a transaction to modify its priority
type PriorityTxWrapper struct {
	sdk.Tx
	boostedPriority int64
}

// GetPriority returns the boosted priority for marked transactions
func (ptw *PriorityTxWrapper) GetPriority() int64 {
	return ptw.boostedPriority
}

// Insert adds a transaction to the mempool
// Note: Priority boosting is handled by the TxPriority implementation
// This method simply passes through to the underlying mempool
func (epm *EnhancedPriorityMempool) Insert(ctx context.Context, tx sdk.Tx) error {
	// Check if this is a priority transaction for logging
	isPriority := IsMarkedPriorityTx(tx)

	if isPriority {
		if txWithMemo, ok := tx.(sdk.TxWithMemo); ok {
			epm.logger.Debug("inserting priority transaction", "memo", txWithMemo.GetMemo())
		}
	}

	// Insert using the base mempool
	// Priority is determined by the TxPriority implementation
	return epm.Mempool.Insert(ctx, tx)
}

// Select returns an iterator of transactions in priority order
// Priority transactions will naturally come first due to their boosted priority
func (epm *EnhancedPriorityMempool) Select(ctx context.Context, txs [][]byte) mempool.Iterator {
	return epm.Mempool.Select(ctx, txs)
}

// CountTx returns the number of transactions in the mempool
func (epm *EnhancedPriorityMempool) CountTx() int {
	return epm.Mempool.CountTx()
}

// Remove removes a transaction from the mempool
func (epm *EnhancedPriorityMempool) Remove(tx sdk.Tx) error {
	// Try to remove the original tx or wrapped tx
	err := epm.Mempool.Remove(tx)
	if err != nil {
		// If removal failed, it might be wrapped, try unwrapping
		if wrapper, ok := tx.(*PriorityTxWrapper); ok {
			return epm.Mempool.Remove(wrapper.Tx)
		}
	}
	return err
}

// GetPriorityBoost returns the configured priority boost value
func (epm *EnhancedPriorityMempool) GetPriorityBoost() int64 {
	return epm.priorityBoost
}

// SetPriorityBoost allows dynamic adjustment of the priority boost
func (epm *EnhancedPriorityMempool) SetPriorityBoost(boost int64) {
	if boost < 0 {
		epm.logger.Warn("attempted to set negative priority boost", "boost", boost)
		return
	}
	epm.priorityBoost = boost
	epm.logger.Info("priority boost updated", "new_boost", boost)
}

// GetStats returns mempool statistics
func (epm *EnhancedPriorityMempool) GetStats() string {
	return fmt.Sprintf("EnhancedPriorityMempool{count=%d, boost=%d}",
		epm.CountTx(), epm.priorityBoost)
}

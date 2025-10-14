package preconfer

import (
	"context"
	"fmt"

	"cosmossdk.io/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

var _ mempool.Mempool = &Mempool{}

// Mempool wraps the standard mempool and enhances it
// to give marked transactions higher priority
type Mempool struct {
	mempool.Mempool
	txDecoder sdk.TxDecoder
	logger    log.Logger

	// Priority boost for marked transactions
	// This value is added to the transaction's base priority
	priorityBoost int64
}

// MempoolConfig configuration for the preconfer mempool
type MempoolConfig struct {
	// BaseMempool is the underlying mempool implementation
	BaseMempool mempool.Mempool

	// TxDecoder for decoding transaction bytes
	TxDecoder sdk.TxDecoder

	// PriorityBoost is the priority increase for marked transactions
	// Default is 1_000_000_000 if not specified
	PriorityBoost int64

	// Logger for mempool operations
	Logger log.Logger
}

// NewMempool creates a new preconfer mempool
func NewMempool(cfg MempoolConfig) *Mempool {
	if cfg.PriorityBoost == 0 {
		cfg.PriorityBoost = DefaultPriorityBoost
	}

	if cfg.Logger == nil {
		cfg.Logger = log.NewNopLogger()
	}

	m := &Mempool{
		Mempool:       cfg.BaseMempool,
		txDecoder:     cfg.TxDecoder,
		logger:        cfg.Logger,
		priorityBoost: cfg.PriorityBoost,
	}

	// Log the base mempool type for verification
	cfg.Logger.Info("preconfer.Mempool initialized",
		"base_mempool_type", fmt.Sprintf("%T", cfg.BaseMempool),
		"priority_boost", cfg.PriorityBoost)

	return m
}

// Insert adds a transaction to the mempool with priority boosting.
// If the transaction is marked as a priority transaction, its priority
// is boosted by adding priorityBoost to the context's priority value.
func (epm *Mempool) Insert(ctx context.Context, tx sdk.Tx) error {
	// Extract gas limit from transaction if available
	var gasWanted uint64
	if gasTx, ok := tx.(interface{ GetGas() uint64 }); ok {
		gasWanted = gasTx.GetGas()
	}

	return epm.InsertWithGasWanted(ctx, tx, gasWanted)
}

// InsertWithGasWanted adds a transaction to the mempool with explicit gas wanted.
// If the transaction is marked as a priority transaction, its priority is boosted
// by modifying the context's priority before delegating to the base mempool.
//
// The PriorityNonceMempool retrieves priority using TxPriority.GetTxPriority(ctx, tx),
// which reads from ctx.Priority(). By modifying the context's priority here, we ensure
// the boosted priority is properly indexed in the underlying mempool's skip list.
func (epm *Mempool) InsertWithGasWanted(ctx context.Context, tx sdk.Tx, gasWanted uint64) error {
	// Check if this is a priority transaction
	isPriority := IsMarkedPriorityTx(tx)

	if isPriority {
		// Log priority transaction insertion
		if txWithMemo, ok := tx.(sdk.TxWithMemo); ok {
			epm.logger.Debug("inserting priority transaction",
				"memo", txWithMemo.GetMemo(),
				"gas", gasWanted,
				"boost", epm.priorityBoost)
		}

		// Boost the priority by modifying the context
		// The base mempool will read this boosted priority value
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		currentPriority := sdkCtx.Priority()
		boostedPriority := currentPriority + epm.priorityBoost

		// Create a new context with the boosted priority
		sdkCtx = sdkCtx.WithPriority(boostedPriority)
		ctx = sdkCtx
	}

	// Delegate to base mempool's InsertWithGasWanted
	// InsertWithGasWanted is part of the mempool.Mempool interface
	return epm.Mempool.InsertWithGasWanted(ctx, tx, gasWanted)
}

// Select returns an iterator of transactions in priority order
// Priority transactions will naturally come first due to their boosted priority
func (epm *Mempool) Select(ctx context.Context, txs [][]byte) mempool.Iterator {
	return epm.Mempool.Select(ctx, txs)
}

// CountTx returns the number of transactions in the mempool
func (epm *Mempool) CountTx() int {
	return epm.Mempool.CountTx()
}

// Remove removes a transaction from the mempool
func (epm *Mempool) Remove(tx sdk.Tx) error {
	return epm.Mempool.Remove(tx)
}

// GetPriorityBoost returns the configured priority boost value
func (epm *Mempool) GetPriorityBoost() int64 {
	return epm.priorityBoost
}

// SetPriorityBoost allows dynamic adjustment of the priority boost
func (epm *Mempool) SetPriorityBoost(boost int64) {
	if boost < 0 {
		epm.logger.Warn("attempted to set negative priority boost", "boost", boost)
		return
	}
	epm.priorityBoost = boost
	epm.logger.Info("priority boost updated", "new_boost", boost)
}

// GetStats returns mempool statistics
func (epm *Mempool) GetStats() string {
	return fmt.Sprintf("Mempool{count=%d, boost=%d}",
		epm.CountTx(), epm.priorityBoost)
}

// GetBaseMempool returns the underlying base mempool
// This is useful for type checking and verification
func (epm *Mempool) GetBaseMempool() mempool.Mempool {
	return epm.Mempool
}

// IsPriorityNonceMempool checks if the base mempool is a PriorityNonceMempool
func (epm *Mempool) IsPriorityNonceMempool() bool {
	typeName := fmt.Sprintf("%T", epm.Mempool)
	// Check if it's a PriorityNonceMempool (the type will be *mempool.PriorityNonceMempool[int64])
	return typeName == PriorityNonceMempoolType
}

// GetBaseMempoolType returns the type name of the base mempool
func (epm *Mempool) GetBaseMempoolType() string {
	return fmt.Sprintf("%T", epm.Mempool)
}

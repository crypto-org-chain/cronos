package executionbook

import (
	"bytes"
	"context"
	"fmt"

	"cosmossdk.io/log"
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

// ProposalHandler handles block proposals using sequencer transactions
type ProposalHandler struct {
	book                  *ExecutionBook
	txDecoder             TxDecoder
	txEncoder             TxEncoder
	mempool               mempool.Mempool
	quickBlockGasFraction float64 // Fraction of maxBlockGas for QuickBlockGasFraction txs (e.g., 0.2 for 1/5)
	logger                log.Logger
}

// TxDecoder is a function that decodes transaction bytes
type TxDecoder func(txBytes []byte) (interface{}, error)

// TxEncoder is a function that encodes transaction to bytes
type TxEncoder func(tx sdk.Tx) ([]byte, error)

// ProposalHandlerConfig configuration for the proposal handler
type ProposalHandlerConfig struct {
	Book                  *ExecutionBook
	TxDecoder             TxDecoder
	TxEncoder             TxEncoder       // Optional: for encoding transactions from mempool
	Mempool               mempool.Mempool // Optional: mempool for fetching regular transactions
	QuickBlockGasFraction float64         // Optional: fraction of maxBlockGas for QuickBlockGasFraction txs (default 0.2 = 1/5)
	Logger                log.Logger
}

// NewProposalHandler creates a new proposal handler
func NewProposalHandler(cfg ProposalHandlerConfig) *ProposalHandler {
	if cfg.Book == nil {
		panic("execution book cannot be nil")
	}
	if cfg.TxDecoder == nil {
		panic("tx decoder cannot be nil")
	}
	if cfg.Logger == nil {
		cfg.Logger = log.NewNopLogger()
	}

	// Default quickBlockGasFraction gas limit to 1/5 (0.2)
	quickBlockGasFraction := cfg.QuickBlockGasFraction
	if quickBlockGasFraction <= 0 || quickBlockGasFraction >= 1.0 {
		quickBlockGasFraction = 0.2 // Default to 1/5
	}

	return &ProposalHandler{
		book:                  cfg.Book,
		txDecoder:             cfg.TxDecoder,
		txEncoder:             cfg.TxEncoder,
		mempool:               cfg.Mempool,
		quickBlockGasFraction: quickBlockGasFraction,
		logger:                cfg.Logger,
	}
}

// PrepareProposalHandler prepares a block proposal from sequencer transactions
// Sequencer transactions get up to quickBlockGasFraction of maxBlockGas
func (h *ProposalHandler) PrepareProposalHandler() func(*abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
	return func(req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		ctx := sdk.Context{}.WithContext(context.Background())
		var maxBlockGas int64
		if b := ctx.ConsensusParams().Block; b != nil {
			maxBlockGas = int64(b.MaxGas)
		}

		h.logger.Info("Preparing block proposal from execution book",
			"height", req.Height,
			"max_tx_bytes", req.MaxTxBytes,
			"quick_block_gas_fraction", h.quickBlockGasFraction)

		// Update block height (sequence numbers are global and don't reset)
		h.book.UpdateBlockHeight(uint64(req.Height))

		// Calculate gas limits based on configuration
		var quickBlockGasLimit uint64
		if maxBlockGas < 0 {
			// Unlimited gas
			quickBlockGasLimit = ^uint64(0)
		} else {
			quickBlockGasLimit = uint64(float64(maxBlockGas) * h.quickBlockGasFraction)
		}

		h.logger.Debug("QuickBlockGasLimit:", quickBlockGasLimit)

		var txsToInclude [][]byte
		var totalBytes int64
		var totalGas uint64
		sequencerTxCount := 0

		sequencerTxs := h.book.GetOrderedTransactions()
		h.logger.Debug("Retrieved sequencer transactions", "count", len(sequencerTxs))

		for _, seqTx := range sequencerTxs {
			// Try to fetch transaction bytes from cache or mempool
			txBytes := h.getTransactionBytes(seqTx.TxHash)
			if txBytes == nil {
				h.logger.Warn("Sequencer transaction bytes not found, skipping",
					"tx_hash", fmt.Sprintf("%x", seqTx.TxHash),
					"sequence", seqTx.SequenceNumber)
				continue
			}

			// Decode transaction to get gas
			tx, err := h.txDecoder(txBytes)
			if err != nil {
				h.logger.Warn("Failed to decode sequencer transaction",
					"tx_hash", fmt.Sprintf("%x", seqTx.TxHash),
					"error", err)
				continue
			}

			var txGas uint64
			if gasTx, ok := tx.(mempool.GasTx); ok {
				txGas = gasTx.GetGas()
			}

			// Check if adding this tx would exceed limits
			if totalBytes+int64(len(txBytes)) > req.MaxTxBytes {
				h.logger.Debug("Sequencer tx would exceed max bytes", "current_bytes", totalBytes)
				break
			}

			if totalGas+txGas > quickBlockGasLimit {
				h.logger.Debug("Sequencer tx would exceed quick block gas limit",
					"current_gas", totalGas,
					"tx_gas", txGas,
					"sequencer_limit", quickBlockGasLimit)
				break
			}

			// Add transaction
			txsToInclude = append(txsToInclude, txBytes)
			totalBytes += int64(len(txBytes))
			totalGas += txGas
			sequencerTxCount++

			h.logger.Debug("Added sequencer transaction",
				"tx_hash", fmt.Sprintf("%x", seqTx.TxHash),
				"sequence", seqTx.SequenceNumber,
				"gas", txGas,
				"total_gas", totalGas)
		}

		h.logger.Info("Block proposal prepared",
			"height", req.Height,
			"total_tx_count", len(txsToInclude),
			"sequencer_tx_count", sequencerTxCount,
			"total_bytes", totalBytes,
			"total_gas", totalGas)

		return &abci.ResponsePrepareProposal{
			Txs: txsToInclude,
		}, nil
	}
}

// ProcessProposalHandler validates a proposed block
// Ensures that all transactions in the proposal are valid sequencer transactions
func (h *ProposalHandler) ProcessProposalHandler() func(*abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
	return func(req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		h.logger.Debug("Processing block proposal",
			"height", req.Height,
			"tx_count", len(req.Txs))

		// Validate that all transactions are from the execution book
		for i, txBytes := range req.Txs {
			// Calculate transaction hash
			txHash := CalculateTxHash(txBytes)

			// Check if this transaction exists in the execution book
			seqTx, exists := h.book.GetTransaction(txHash)
			if !exists {
				h.logger.Error("Transaction not found in execution book",
					"tx_index", i,
					"tx_hash", txHash)
				return &abci.ResponseProcessProposal{
					Status: abci.ResponseProcessProposal_REJECT,
				}, nil
			}

			// Verify transaction hasn't been included already
			if seqTx.Included {
				h.logger.Error("Transaction already included",
					"tx_index", i,
					"tx_hash", txHash)
				return &abci.ResponseProcessProposal{
					Status: abci.ResponseProcessProposal_REJECT,
				}, nil
			}

			h.logger.Debug("Validated sequencer transaction",
				"tx_index", i,
				"tx_hash", txHash,
				"sequence", seqTx.SequenceNumber)
		}

		h.logger.Info("Block proposal accepted",
			"height", req.Height,
			"tx_count", len(req.Txs))

		return &abci.ResponseProcessProposal{
			Status: abci.ResponseProcessProposal_ACCEPT,
		}, nil
	}
}

// OnBlockCommit should be called after a block is committed
// Marks all transactions in the block as included and cleans them up
func (h *ProposalHandler) OnBlockCommit(blockHeight uint64, txHashes [][]byte) error {
	h.logger.Info("Block committed, marking transactions as included",
		"block_height", blockHeight,
		"tx_count", len(txHashes))

	// Mark transactions as included
	h.book.MarkIncluded(txHashes, blockHeight)

	// Cleanup included transactions
	cleaned := h.book.CleanupIncludedTransactions()

	h.logger.Debug("Cleaned up included transactions",
		"cleaned_count", cleaned,
		"block_height", blockHeight)

	// Get stats for logging
	stats := h.book.GetStats()
	h.logger.Info("Execution book stats after commit",
		"pending_txs", stats.PendingTransactions,
		"next_sequence", stats.NextSequence,
		"block_height", blockHeight)

	return nil
}

// GetSequencerTransactionBytes is a placeholder for fetching transaction bytes
// In a real implementation, this would query a transaction pool or mempool
func (h *ProposalHandler) GetSequencerTransactionBytes(txHash []byte) ([]byte, error) {
	// TODO: Implement transaction bytes retrieval
	// This might involve:
	// 1. Querying a local transaction cache
	// 2. Requesting from the sequencer/relayer
	// 3. Looking up in a pending transaction pool

	return nil, fmt.Errorf("transaction bytes retrieval not implemented for hash: %x", txHash)
}

// ValidateSequencerTransaction validates a sequencer transaction
// This can be called before accepting a transaction into the execution book
func (h *ProposalHandler) ValidateSequencerTransaction(txBytes []byte, seqTx *SequencerTransaction) error {
	// Verify hash matches
	calculatedHash := CalculateTxHash(txBytes)
	if !bytesEqual(calculatedHash, seqTx.TxHash) {
		return fmt.Errorf("transaction hash mismatch: expected %x, got %x",
			seqTx.TxHash, calculatedHash)
	}

	// Decode and validate transaction
	_, err := h.txDecoder(txBytes)
	if err != nil {
		return fmt.Errorf("failed to decode transaction: %w", err)
	}

	// Additional validation could include:
	// - Gas limits
	// - Nonce checks
	// - Signature verification
	// - Balance checks

	return nil
}

// bytesEqual compares two byte slices for equality
func bytesEqual(a, b []byte) bool {
	return bytes.Equal(a, b)
}

// getTransactionBytes fetches transaction bytes from mempool
func (h *ProposalHandler) getTransactionBytes(txHash []byte) []byte {
	// If mempool is available, try to find the transaction there
	if h.mempool != nil {
		ctx := sdk.Context{}.WithContext(context.Background())
		iterator := h.mempool.Select(ctx, nil)
		for iterator != nil {
			memTx := iterator.Tx()

			// Get the actual SDK transaction from memTx
			sdkTx := memTx.Tx

			// Try to encode the transaction
			txBytes, err := h.encodeTx(sdkTx)
			if err == nil {
				// Check if this is the transaction we're looking for
				if bytes.Equal(CalculateTxHash(txBytes), txHash) {
					return txBytes
				}
			}

			iterator = iterator.Next()
		}
	}

	return nil
}

// encodeTx encodes an SDK transaction to bytes
func (h *ProposalHandler) encodeTx(tx sdk.Tx) ([]byte, error) {
	if h.txEncoder == nil {
		return nil, fmt.Errorf("tx encoder not configured")
	}
	return h.txEncoder(tx)
}

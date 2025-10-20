package executionbook

import (
	"bytes"
	"fmt"

	"cosmossdk.io/log"
	abci "github.com/cometbft/cometbft/abci/types"
)

// ProposalHandler handles block proposals using sequencer transactions
type ProposalHandler struct {
	book      *ExecutionBook
	txDecoder TxDecoder
	logger    log.Logger
}

// TxDecoder is a function that decodes transaction bytes
type TxDecoder func(txBytes []byte) (interface{}, error)

// ProposalHandlerConfig configuration for the proposal handler
type ProposalHandlerConfig struct {
	Book      *ExecutionBook
	TxDecoder TxDecoder
	Logger    log.Logger
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

	return &ProposalHandler{
		book:      cfg.Book,
		txDecoder: cfg.TxDecoder,
		logger:    cfg.Logger,
	}
}

// PrepareProposalHandler prepares a block proposal using ONLY sequencer transactions
// This overrides the default block building to use transactions from the execution book
func (h *ProposalHandler) PrepareProposalHandler() func(*abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
	return func(req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		h.logger.Info("Preparing block proposal from execution book",
			"height", req.Height,
			"max_tx_bytes", req.MaxTxBytes)

		// Update block height (sequence numbers are global and don't reset)
		h.book.UpdateBlockHeight(uint64(req.Height))

		// Get ordered transactions from execution book
		sequencerTxs := h.book.GetOrderedTransactions()

		h.logger.Debug("Retrieved sequencer transactions",
			"count", len(sequencerTxs),
			"height", req.Height)

		// Build transaction list for the block
		// We need to fetch the actual transaction bytes for each sequencer transaction
		var txsToInclude [][]byte
		var totalBytes int64

		for _, seqTx := range sequencerTxs {
			// In a real implementation, you would need to:
			// 1. Fetch the actual transaction bytes from a transaction pool or cache
			// 2. Validate the transaction
			// 3. Check gas limits and other constraints

			// For now, we log that we would include this transaction
			h.logger.Debug("Would include sequencer transaction",
				"tx_hash", seqTx.TxHash,
				"sequence", seqTx.SequenceNumber,
				"sequencer_id", seqTx.SequencerID)

			// TODO: Implement actual transaction fetching
			// txBytes := h.fetchTransactionBytes(seqTx.TxHash)
			// if txBytes == nil {
			//     h.logger.Warn("Transaction bytes not found", "tx_hash", seqTx.TxHash)
			//     continue
			// }

			// Check if adding this tx would exceed max bytes
			// if totalBytes + int64(len(txBytes)) > req.MaxTxBytes {
			//     h.logger.Debug("Reached max tx bytes", "included_txs", len(txsToInclude))
			//     break
			// }

			// txsToInclude = append(txsToInclude, txBytes)
			// totalBytes += int64(len(txBytes))
		}

		h.logger.Info("Block proposal prepared",
			"height", req.Height,
			"tx_count", len(txsToInclude),
			"total_bytes", totalBytes,
			"sequencer_tx_count", len(sequencerTxs))

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

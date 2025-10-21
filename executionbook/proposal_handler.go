package executionbook

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"cosmossdk.io/log"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

// ProposalHandler handles block proposals using sequencer transactions
type ProposalHandler struct {
	book                  *ExecutionBook
	txDecoder             TxDecoder
	txEncoder             TxEncoder
	mempool               mempool.Mempool
	txSelector            baseapp.TxSelector // Transaction selector for validation
	quickBlockGasFraction float64            // Fraction of maxBlockGas for QuickBlockGasFraction txs (e.g., 0.2 for 1/5)
	fastPrepareProposal   bool               // Enable fast prepare proposal mode
	lastProcessedTxHashes [][]byte           // Last processed transaction hashes (for OnBlockCommit)
	mu                    sync.Mutex         // Mutex for thread-safe access to lastProcessedTxHashes
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
		txSelector:            baseapp.NewDefaultTxSelector(),
		quickBlockGasFraction: quickBlockGasFraction,
		fastPrepareProposal:   false, // Default to false, can be enabled via SetFastPrepareProposal
		logger:                cfg.Logger,
	}
}

// SetTxSelector sets the TxSelector function on the ProposalHandler.
func (h *ProposalHandler) SetTxSelector(ts baseapp.TxSelector) {
	h.txSelector = ts
}

// SetFastPrepareProposal enables fast prepare proposal mode
func (h *ProposalHandler) SetFastPrepareProposal(enabled bool) {
	h.fastPrepareProposal = enabled
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

		// Clear the tx selector on function exit
		defer h.txSelector.Clear()

		h.logger.Info("Preparing block proposal from execution book",
			"height", req.Height,
			"max_tx_bytes", req.MaxTxBytes,
			"quick_block_gas_fraction", h.quickBlockGasFraction,
			"fast_prepare_proposal", h.fastPrepareProposal)

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

		// Build transaction hash map from either mempool or req.Txs (for NoOpMempool)
		// This avoids repeated mempool iteration (O(m) instead of O(n*m))
		txHashMap := h.buildTxHashMap()

		// If mempool is empty (e.g., NoOpMempool), use transactions from req.Txs
		// CometBFT provides transactions via gossip in this field
		if len(txHashMap) == 0 && len(req.Txs) > 0 {
			h.logger.Debug("Mempool empty, building hash map from req.Txs", "tx_count", len(req.Txs))
			txHashMap = make(map[string][]byte)
			for _, txBytes := range req.Txs {
				txHash := CalculateTxHash(txBytes)
				txHashMap[string(txHash)] = txBytes
			}
		}

		h.logger.Debug("Built transaction hash map", "tx_count", len(txHashMap))

		var txsToInclude [][]byte
		var totalBytes int64
		var totalGas uint64
		sequencerTxCount := 0

		sequencerTxs := h.book.GetOrderedTransactions()
		h.logger.Debug("Retrieved sequencer transactions", "count", len(sequencerTxs))

		// If fast prepare proposal is enabled, use SelectTxForProposalFast for batch validation
		if h.fastPrepareProposal {
			// Build ordered transaction bytes array
			var orderedTxBytes [][]byte
			for _, seqTx := range sequencerTxs {
				txBytes, found := txHashMap[string(seqTx.TxHash)]
				if found {
					orderedTxBytes = append(orderedTxBytes, txBytes)
				} else {
					h.logger.Warn("Sequencer transaction bytes not found, skipping",
						"tx_hash", fmt.Sprintf("%x", seqTx.TxHash),
						"sequence", seqTx.SequenceNumber)
				}
			}

			// Use TxSelector for fast validation
			txsToInclude = h.txSelector.SelectTxForProposalFast(ctx, orderedTxBytes)
			sequencerTxCount = len(txsToInclude)

			// Calculate total bytes and gas for logging
			for _, txBytes := range txsToInclude {
				totalBytes += int64(len(txBytes))
				if tx, err := h.txDecoder(txBytes); err == nil {
					if gasTx, ok := tx.(mempool.GasTx); ok {
						totalGas += gasTx.GetGas()
					}
				}
			}

			h.logger.Debug("Fast proposal selection completed",
				"selected_txs", len(txsToInclude),
				"total_bytes", totalBytes,
				"total_gas", totalGas)
		} else {
			// Standard validation path - validate each transaction individually
			for _, seqTx := range sequencerTxs {
				// Lookup transaction bytes from hash map (O(1) instead of O(m))
				txBytes, found := txHashMap[string(seqTx.TxHash)]
				if !found {
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

				// Use TxSelector for validation if available
				sdkTx, _ := tx.(sdk.Tx)
				stop := h.txSelector.SelectTxForProposal(ctx, uint64(req.MaxTxBytes), uint64(quickBlockGasLimit), sdkTx, txBytes, txGas)
				if stop {
					h.logger.Debug("TxSelector indicated to stop adding transactions")
					break
				}

				// Add transaction from selector's selected list
				totalBytes += int64(len(txBytes))
				totalGas += txGas
				sequencerTxCount++

				h.logger.Debug("Added sequencer transaction",
					"tx_hash", fmt.Sprintf("%x", seqTx.TxHash),
					"sequence", seqTx.SequenceNumber,
					"gas", txGas,
					"total_gas", totalGas)
			}

			// Get selected transactions from selector
			txsToInclude = h.txSelector.SelectedTxs(ctx)
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
// buildTxHashMap builds a map of transaction hash -> transaction bytes
// by iterating the mempool once. This is more efficient than calling
// getTransactionBytes repeatedly (O(m) instead of O(n*m))
func (h *ProposalHandler) buildTxHashMap() map[string][]byte {
	txHashMap := make(map[string][]byte)

	if h.mempool == nil {
		return txHashMap
	}

	ctx := sdk.Context{}.WithContext(context.Background())
	iterator := h.mempool.Select(ctx, nil)

	for iterator != nil {
		memTx := iterator.Tx()

		// Get the actual SDK transaction from memTx
		sdkTx := memTx.Tx

		// Try to encode the transaction
		txBytes, err := h.encodeTx(sdkTx)
		if err == nil {
			// Calculate hash and store in map
			txHash := CalculateTxHash(txBytes)
			txHashMap[string(txHash)] = txBytes
		}

		iterator = iterator.Next()
	}

	return txHashMap
}

// encodeTx encodes an SDK transaction to bytes
func (h *ProposalHandler) encodeTx(tx sdk.Tx) ([]byte, error) {
	if h.txEncoder == nil {
		return nil, fmt.Errorf("tx encoder not configured")
	}
	return h.txEncoder(tx)
}

// SetLastProcessedTxHashes stores the transaction hashes from the last processed block
func (h *ProposalHandler) SetLastProcessedTxHashes(txHashes [][]byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.lastProcessedTxHashes = txHashes

	h.logger.Debug("Stored last processed tx hashes",
		"count", len(txHashes))
}

// GetLastProcessedTxHashes returns a copy of the last processed transaction hashes
func (h *ProposalHandler) GetLastProcessedTxHashes() [][]byte {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Make a copy to avoid concurrent modification issues
	txHashes := make([][]byte, len(h.lastProcessedTxHashes))
	copy(txHashes, h.lastProcessedTxHashes)

	return txHashes
}

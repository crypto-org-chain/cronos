package executionbook

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cosmossdk.io/log"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

// SequencerTransaction represents a transaction executed by the sequencer
type SequencerTransaction struct {
	TxHash         []byte // 32-byte transaction hash
	SequenceNumber uint64
	Signature      []byte // Max 65 bytes for secp256k1/Ed25519
	SequencerID    string // Identifier of the sequencer that executed this
	Timestamp      time.Time
	BlockHeight    uint64 // Block where it should be included
	Included       bool   // Whether it has been included in a block
}

// ExecutionBook stores and tracks sequencer-executed transactions
type ExecutionBook struct {
	// Map of txHash -> SequencerTransaction
	transactions map[string]*SequencerTransaction

	// Ordered list of transactions by sequence number
	orderedTxs []*SequencerTransaction

	// Expected next sequence number (global sequence, continuously incrementing)
	nextSequence uint64

	// Current block height being built
	currentBlockHeight uint64

	// Sequencer public keys for signature verification
	// Map of sequencerID -> public key
	sequencerPubKeys map[string]cryptotypes.PubKey

	// State file path for persistence
	stateFilePath string

	// Maximum number of pending transactions (0 = unlimited)
	bookSize int

	// Mutex for thread-safe access
	mu sync.RWMutex

	logger log.Logger
}

// ExecutionBookState represents the persisted state
type ExecutionBookState struct {
	Transactions       []*PersistedTransaction `json:"transactions"`
	NextSequence       uint64                  `json:"next_sequence"`
	CurrentBlockHeight uint64                  `json:"current_block_height"`
	SavedAt            time.Time               `json:"saved_at"`
}

// PersistedTransaction is the serializable version of SequencerTransaction
type PersistedTransaction struct {
	TxHash         string    `json:"tx_hash"`
	SequenceNumber uint64    `json:"sequence_number"`
	Signature      string    `json:"signature"`
	SequencerID    string    `json:"sequencer_id"`
	Timestamp      time.Time `json:"timestamp"`
	BlockHeight    uint64    `json:"block_height"`
	Included       bool      `json:"included"`
}

// ExecutionBookConfig configuration for the execution book
type ExecutionBookConfig struct {
	Logger           log.Logger
	SequencerPubKeys map[string]cryptotypes.PubKey // Map of sequencerID -> pubkey
	StateFilePath    string                        // Path to state file for persistence (optional)
	BookSize         int                           // Maximum pending transactions (0 = unlimited)
}

// NewExecutionBook creates a new execution book
func NewExecutionBook(cfg ExecutionBookConfig) *ExecutionBook {
	if cfg.Logger == nil {
		cfg.Logger = log.NewNopLogger()
	}

	if cfg.SequencerPubKeys == nil {
		cfg.SequencerPubKeys = make(map[string]cryptotypes.PubKey)
	}

	book := &ExecutionBook{
		transactions:       make(map[string]*SequencerTransaction),
		orderedTxs:         make([]*SequencerTransaction, 0),
		nextSequence:       0,
		currentBlockHeight: 0,
		sequencerPubKeys:   cfg.SequencerPubKeys,
		stateFilePath:      cfg.StateFilePath,
		bookSize:           cfg.BookSize,
		logger:             cfg.Logger,
	}

	// Try to recover state from file if path is provided
	if cfg.StateFilePath != "" {
		if err := book.LoadState(); err != nil {
			cfg.Logger.Warn("Failed to load ExecutionBook state, starting fresh",
				"error", err,
				"state_file", cfg.StateFilePath)
		} else {
			cfg.Logger.Info("ExecutionBook state recovered from file",
				"state_file", cfg.StateFilePath,
				"next_sequence", book.nextSequence,
				"current_block_height", book.currentBlockHeight,
				"transaction_count", len(book.transactions))
		}
	}

	bookSizeStr := "unlimited"
	if cfg.BookSize > 0 {
		bookSizeStr = fmt.Sprintf("%d", cfg.BookSize)
	}

	cfg.Logger.Info("ExecutionBook initialized",
		"sequencer_count", len(cfg.SequencerPubKeys),
		"persistence_enabled", cfg.StateFilePath != "",
		"book_size", bookSizeStr)

	return book
}

// SubmitSequencerTx submits a sequencer transaction to the execution book
// Returns error if signature is invalid or sequence number is out of order
func (eb *ExecutionBook) SubmitSequencerTx(
	txHash []byte,
	sequenceNumber uint64,
	signature []byte,
	sequencerID string,
) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Validate txHash length (must be exactly 32 bytes)
	if len(txHash) != 32 {
		return fmt.Errorf("invalid tx hash length: expected 32 bytes, got %d", len(txHash))
	}

	// Validate signature length (max 65 bytes for secp256k1/Ed25519)
	if len(signature) > 65 {
		return fmt.Errorf("signature too long: max 65 bytes, got %d", len(signature))
	}
	if len(signature) == 0 {
		return fmt.Errorf("signature cannot be empty")
	}

	// Verify sequencer is known
	sequencerPubKey, exists := eb.sequencerPubKeys[sequencerID]
	if !exists {
		return fmt.Errorf("unknown sequencer: %s", sequencerID)
	}

	// Verify signature
	if !eb.verifySequencerSignature(txHash, sequenceNumber, signature, sequencerPubKey) {
		return fmt.Errorf("invalid sequencer signature for tx %x", txHash)
	}

	// Check if transaction already exists
	txHashStr := hex.EncodeToString(txHash)
	if _, exists := eb.transactions[txHashStr]; exists {
		return fmt.Errorf("transaction %x already submitted", txHash)
	}

	// Check book size limit (if enabled)
	if eb.bookSize > 0 {
		pendingCount := 0
		for _, tx := range eb.transactions {
			if !tx.Included {
				pendingCount++
			}
		}
		if pendingCount >= eb.bookSize {
			return fmt.Errorf("execution book is full: %d/%d pending transactions", pendingCount, eb.bookSize)
		}
	}

	// Enforce strict sequence ordering - must be exactly the next expected sequence
	if sequenceNumber != eb.nextSequence {
		return fmt.Errorf("sequence number mismatch: expected %d, got %d (no gaps allowed)",
			eb.nextSequence, sequenceNumber)
	}

	// Create transaction entry
	tx := &SequencerTransaction{
		TxHash:         txHash,
		SequenceNumber: sequenceNumber,
		Signature:      signature,
		SequencerID:    sequencerID,
		Timestamp:      time.Now(),
		BlockHeight:    eb.currentBlockHeight,
		Included:       false,
	}

	// Store transaction
	eb.transactions[txHashStr] = tx
	eb.orderedTxs = append(eb.orderedTxs, tx)

	// Increment next expected sequence (global sequence, never resets)
	eb.nextSequence++

	eb.logger.Debug("Sequencer transaction submitted",
		"tx_hash", hex.EncodeToString(txHash),
		"sequence", sequenceNumber,
		"sequencer_id", sequencerID,
		"next_sequence", eb.nextSequence)

	// Save state after adding transaction
	go func() {
		if err := eb.SaveState(); err != nil {
			eb.logger.Error("Failed to save state after transaction submission", "error", err)
		}
	}()

	return nil
}

// GetOrderedTransactions returns transactions in sequence order for block building
// Only returns transactions that haven't been included yet
func (eb *ExecutionBook) GetOrderedTransactions() []*SequencerTransaction {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	// Return only non-included transactions in order
	result := make([]*SequencerTransaction, 0, len(eb.orderedTxs))
	for _, tx := range eb.orderedTxs {
		if !tx.Included {
			result = append(result, tx)
		}
	}

	return result
}

// MarkIncluded marks transactions as included in a block
// This should be called after block commit
func (eb *ExecutionBook) MarkIncluded(txHashes [][]byte, blockHeight uint64) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	for _, txHash := range txHashes {
		txHashStr := hex.EncodeToString(txHash)
		if tx, exists := eb.transactions[txHashStr]; exists {
			tx.Included = true
			tx.BlockHeight = blockHeight

			eb.logger.Debug("Transaction marked as included",
				"tx_hash", hex.EncodeToString(txHash),
				"block_height", blockHeight)
		}
	}
}

// CleanupIncludedTransactions removes transactions that have been included
// This is called after block finalization to free memory
func (eb *ExecutionBook) CleanupIncludedTransactions() int {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	cleanedCount := 0

	// Remove included transactions
	for txHash, tx := range eb.transactions {
		if tx.Included {
			delete(eb.transactions, txHash)
			cleanedCount++
		}
	}

	// Rebuild ordered list without included transactions
	newOrdered := make([]*SequencerTransaction, 0)
	for _, tx := range eb.orderedTxs {
		if !tx.Included {
			newOrdered = append(newOrdered, tx)
		}
	}
	eb.orderedTxs = newOrdered

	if cleanedCount > 0 {
		eb.logger.Info("Cleaned up included transactions",
			"count", cleanedCount,
			"remaining", len(eb.transactions))

		// Save state after cleanup
		go func() {
			if err := eb.SaveState(); err != nil {
				eb.logger.Error("Failed to save state after cleanup", "error", err)
			}
		}()
	}

	return cleanedCount
}

// UpdateBlockHeight updates the current block height being built
// Note: Sequence numbers are global and do NOT reset per block
// This should be called at the start of each new block
func (eb *ExecutionBook) UpdateBlockHeight(blockHeight uint64) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.currentBlockHeight = blockHeight

	eb.logger.Debug("Block height updated",
		"block_height", blockHeight,
		"next_sequence", eb.nextSequence)

	// Save state after block height update
	go func() {
		if err := eb.SaveState(); err != nil {
			eb.logger.Error("Failed to save state after block height update", "error", err)
		}
	}()
}

// GetNextSequence returns the expected next sequence number
func (eb *ExecutionBook) GetNextSequence() uint64 {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return eb.nextSequence
}

// GetTransactionCount returns the number of pending transactions
func (eb *ExecutionBook) GetTransactionCount() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	count := 0
	for _, tx := range eb.transactions {
		if !tx.Included {
			count++
		}
	}
	return count
}

// GetTransaction returns a transaction by hash
func (eb *ExecutionBook) GetTransaction(txHash []byte) (*SequencerTransaction, bool) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	txHashStr := hex.EncodeToString(txHash)
	tx, exists := eb.transactions[txHashStr]
	return tx, exists
}

// AddSequencer adds a new sequencer public key for verification
func (eb *ExecutionBook) AddSequencer(sequencerID string, pubKey cryptotypes.PubKey) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.sequencerPubKeys[sequencerID] = pubKey

	eb.logger.Info("Sequencer added",
		"sequencer_id", sequencerID,
		"key_type", pubKey.Type())
}

// RemoveSequencer removes a sequencer
func (eb *ExecutionBook) RemoveSequencer(sequencerID string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	delete(eb.sequencerPubKeys, sequencerID)

	eb.logger.Info("Sequencer removed",
		"sequencer_id", sequencerID)
}

// verifySequencerSignature verifies the sequencer's signature on a transaction
func (eb *ExecutionBook) verifySequencerSignature(
	txHash []byte,
	sequenceNumber uint64,
	signature []byte,
	pubKey cryptotypes.PubKey,
) bool {
	if pubKey == nil || len(signature) == 0 {
		return false
	}

	// Create canonical message for signing
	// Format: SEQUENCER_TX | txHash | sequenceNumber
	msg := eb.createSequencerMessage(txHash, sequenceNumber)

	// Verify signature
	return pubKey.VerifySignature(msg, signature)
}

// createSequencerMessage creates the canonical message format for sequencer signing
func (eb *ExecutionBook) createSequencerMessage(txHash []byte, sequenceNumber uint64) []byte {
	// Create deterministic message format
	// Structure: SEQUENCER_TX | txHash | sequenceNumber

	msg := []byte("SEQUENCER_TX|")
	msg = append(msg, txHash...)
	msg = append(msg, '|')

	// Add sequence number as 8 bytes (uint64)
	seqBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(seqBytes, sequenceNumber)
	msg = append(msg, seqBytes...)

	// Hash the message for signing
	hash := sha256.Sum256(msg)
	return hash[:]
}

// CreateSequencerSignature is a helper for sequencers to create signatures
// This can be used by the sequencer or relayer for testing
func CreateSequencerSignature(
	txHash []byte,
	sequenceNumber uint64,
	privKey cryptotypes.PrivKey,
) ([]byte, error) {
	if privKey == nil {
		return nil, fmt.Errorf("private key is nil")
	}

	// Create message
	msg := []byte("SEQUENCER_TX|")
	msg = append(msg, txHash...)
	msg = append(msg, '|')

	seqBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(seqBytes, sequenceNumber)
	msg = append(msg, seqBytes...)

	// Hash and sign
	hash := sha256.Sum256(msg)
	signature, err := privKey.Sign(hash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign: %w", err)
	}

	return signature, nil
}

// GetStats returns statistics about the execution book
func (eb *ExecutionBook) GetStats() ExecutionBookStats {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	pending := 0
	included := 0

	for _, tx := range eb.transactions {
		if tx.Included {
			included++
		} else {
			pending++
		}
	}

	return ExecutionBookStats{
		TotalTransactions:    len(eb.transactions),
		PendingTransactions:  pending,
		IncludedTransactions: included,
		NextSequence:         eb.nextSequence,
		CurrentBlockHeight:   eb.currentBlockHeight,
		SequencerCount:       len(eb.sequencerPubKeys),
	}
}

// ExecutionBookStats contains statistics about the execution book
type ExecutionBookStats struct {
	TotalTransactions    int
	PendingTransactions  int
	IncludedTransactions int
	NextSequence         uint64
	CurrentBlockHeight   uint64
	SequencerCount       int
}

// CalculateTxHash is a helper to calculate transaction hash from bytes
func CalculateTxHash(txBytes []byte) []byte {
	hash := sha256.Sum256(txBytes)
	return hash[:]
}

// SaveState persists the current state to disk
func (eb *ExecutionBook) SaveState() error {
	if eb.stateFilePath == "" {
		return nil // Persistence not enabled
	}

	eb.mu.RLock()
	defer eb.mu.RUnlock()

	// Create persisted state
	state := ExecutionBookState{
		Transactions:       make([]*PersistedTransaction, 0, len(eb.orderedTxs)),
		NextSequence:       eb.nextSequence,
		CurrentBlockHeight: eb.currentBlockHeight,
		SavedAt:            time.Now(),
	}

	// Convert transactions to persistable format
	for _, tx := range eb.orderedTxs {
		state.Transactions = append(state.Transactions, &PersistedTransaction{
			TxHash:         hex.EncodeToString(tx.TxHash),
			SequenceNumber: tx.SequenceNumber,
			Signature:      hex.EncodeToString(tx.Signature),
			SequencerID:    tx.SequencerID,
			Timestamp:      tx.Timestamp,
			BlockHeight:    tx.BlockHeight,
			Included:       tx.Included,
		})
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(eb.stateFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Write to temp file first, then rename (atomic operation)
	tempFile := eb.stateFilePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	if err := os.Rename(tempFile, eb.stateFilePath); err != nil {
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	eb.logger.Debug("ExecutionBook state saved",
		"file", eb.stateFilePath,
		"next_sequence", eb.nextSequence,
		"transaction_count", len(eb.transactions))

	return nil
}

// LoadState loads the state from disk
func (eb *ExecutionBook) LoadState() error {
	if eb.stateFilePath == "" {
		return nil // Persistence not enabled
	}

	// Check if file exists
	if _, err := os.Stat(eb.stateFilePath); os.IsNotExist(err) {
		return fmt.Errorf("state file does not exist")
	}

	// Read state file
	data, err := os.ReadFile(eb.stateFilePath)
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}

	// Unmarshal state
	var state ExecutionBookState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Restore state
	eb.nextSequence = state.NextSequence
	eb.currentBlockHeight = state.CurrentBlockHeight
	eb.transactions = make(map[string]*SequencerTransaction)
	eb.orderedTxs = make([]*SequencerTransaction, 0, len(state.Transactions))

	// Restore transactions
	for _, ptx := range state.Transactions {
		txHash, err := hex.DecodeString(ptx.TxHash)
		if err != nil {
			eb.logger.Error("Failed to decode tx hash, skipping transaction",
				"tx_hash", ptx.TxHash,
				"error", err)
			continue
		}

		signature, err := hex.DecodeString(ptx.Signature)
		if err != nil {
			eb.logger.Error("Failed to decode signature, skipping transaction",
				"tx_hash", ptx.TxHash,
				"error", err)
			continue
		}

		tx := &SequencerTransaction{
			TxHash:         txHash,
			SequenceNumber: ptx.SequenceNumber,
			Signature:      signature,
			SequencerID:    ptx.SequencerID,
			Timestamp:      ptx.Timestamp,
			BlockHeight:    ptx.BlockHeight,
			Included:       ptx.Included,
		}

		// Add to maps
		txHashStr := hex.EncodeToString(txHash)
		eb.transactions[txHashStr] = tx
		eb.orderedTxs = append(eb.orderedTxs, tx)
	}

	eb.logger.Info("ExecutionBook state loaded successfully",
		"file", eb.stateFilePath,
		"next_sequence", eb.nextSequence,
		"current_block_height", eb.currentBlockHeight,
		"transaction_count", len(eb.transactions),
		"saved_at", state.SavedAt)

	return nil
}

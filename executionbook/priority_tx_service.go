package executionbook

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

const (
	// StatusUnknown is the string representation for unknown status
	StatusUnknown = "unknown"
)

// PriorityTxService handles priority transaction submissions and preconfirmations
type PriorityTxService struct {
	app       *baseapp.BaseApp
	mempool   mempool.Mempool
	txDecoder sdk.TxDecoder
	logger    log.Logger

	// Preconfirmation tracking
	preconfirmations     map[string]*PreconfirmationInfo
	preconfirmationMutex sync.RWMutex

	// Transaction tracking
	txTracker     map[string]*TxTrackingInfo
	txTrackerLock sync.RWMutex

	// Configuration
	validatorAddress  string
	validatorPrivKey  cryptotypes.PrivKey // Private key for signing preconfirmations
	preconfirmTimeout time.Duration

	// Shutdown management
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// PreconfirmationInfo stores preconfirmation details
type PreconfirmationInfo struct {
	TxHash        string
	Timestamp     time.Time
	Validator     string
	PriorityLevel uint32
	ExpiresAt     time.Time
	Signature     []byte
}

// TxTrackingInfo stores transaction tracking information
type TxTrackingInfo struct {
	TxHash          string
	Status          TxStatusType
	InMempool       bool
	BlockHeight     int64
	MempoolPosition uint32
	Preconfirmation *PreconfirmationInfo
	Timestamp       time.Time
	TxBytes         []byte
}

// TxStatusType represents transaction status
type TxStatusType int

const (
	TxStatusUnknown TxStatusType = iota
	TxStatusPending
	TxStatusPreconfirmed
	TxStatusIncluded
	TxStatusRejected
	TxStatusExpired
)

// String returns the string representation of the transaction status
func (s TxStatusType) String() string {
	switch s {
	case TxStatusUnknown:
		return StatusUnknown
	case TxStatusPending:
		return "pending"
	case TxStatusPreconfirmed:
		return "preconfirmed"
	case TxStatusIncluded:
		return "included"
	case TxStatusRejected:
		return "rejected"
	case TxStatusExpired:
		return "expired"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// PriorityTxServiceConfig configuration for the service
type PriorityTxServiceConfig struct {
	App               *baseapp.BaseApp
	Mempool           mempool.Mempool
	TxDecoder         sdk.TxDecoder
	Logger            log.Logger
	ValidatorAddress  string
	ValidatorPrivKey  cryptotypes.PrivKey // Optional: Private key for signing preconfirmations
	PreconfirmTimeout time.Duration
}

// NewPriorityTxService creates a new priority transaction service
func NewPriorityTxService(cfg PriorityTxServiceConfig) *PriorityTxService {
	if cfg.PreconfirmTimeout == 0 {
		cfg.PreconfirmTimeout = 30 * time.Second // Default 30 seconds
	}

	if cfg.Logger == nil {
		cfg.Logger = log.NewNopLogger()
	}

	ctx, cancel := context.WithCancel(context.Background())

	service := &PriorityTxService{
		app:               cfg.App,
		mempool:           cfg.Mempool,
		txDecoder:         cfg.TxDecoder,
		logger:            cfg.Logger,
		validatorAddress:  cfg.ValidatorAddress,
		validatorPrivKey:  cfg.ValidatorPrivKey,
		preconfirmTimeout: cfg.PreconfirmTimeout,
		preconfirmations:  make(map[string]*PreconfirmationInfo),
		txTracker:         make(map[string]*TxTrackingInfo),
		ctx:               ctx,
		cancel:            cancel,
		done:              make(chan struct{}),
	}

	// Log whether signing is enabled
	if service.validatorPrivKey != nil {
		service.logger.Info("Priority tx service initialized with cryptographic signing enabled",
			"key_type", service.validatorPrivKey.Type())
	} else {
		service.logger.Info("Priority tx service initialized without signing (preconfirmations will be unsigned)")
	}

	// Start cleanup goroutine
	go service.cleanupExpiredPreconfirmations()

	return service
}

// SubmitPriorityTx handles priority transaction submission
func (s *PriorityTxService) SubmitPriorityTx(
	ctx context.Context,
	txBytes []byte,
	priorityLevel uint32,
) (*SubmitPriorityTxResult, error) {
	// Validate priority level
	if priorityLevel != 1 {
		return nil, fmt.Errorf("invalid priority level: must be 1")
	}

	// Decode transaction
	tx, err := s.txDecoder(txBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode transaction: %w", err)
	}

	// Calculate transaction hash
	txHash := s.calculateTxHash(txBytes)

	// Check if already in mempool
	if s.isTxInMempool(txHash) {
		return nil, fmt.Errorf("transaction already in mempool: %s", txHash)
	}

	// Validate transaction has priority marker in memo
	if !IsPriorityTx(tx) {
		// Add priority marker if not present
		s.logger.Info("transaction missing priority marker, adding it", "tx_hash", txHash)
	}

	// Insert into mempool
	err = s.mempool.Insert(ctx, tx)
	if err != nil {
		s.logger.Error("failed to insert transaction into mempool", "error", err, "tx_hash", txHash)
		return &SubmitPriorityTxResult{
			TxHash:   txHash,
			Accepted: false,
			Reason:   fmt.Sprintf("mempool insertion failed: %v", err),
		}, nil
	}

	// Create preconfirmation
	preconf := s.createPreconfirmation(txHash, priorityLevel)

	// Calculate mempool position before tracking (count existing + 1 for this tx)
	position := s.countPriorityTxsInMempool() + 1

	// Track transaction
	s.trackTransaction(txHash, txBytes, preconf, position)

	s.logger.Info("priority transaction accepted",
		"tx_hash", txHash,
		"priority_level", priorityLevel,
		"position", position,
	)

	return &SubmitPriorityTxResult{
		TxHash:          txHash,
		Accepted:        true,
		Preconfirmation: preconf,
		MempoolPosition: position,
	}, nil
}

// GetTxStatus returns the status of a transaction
func (s *PriorityTxService) GetTxStatus(txHash string) (*TxTrackingInfo, error) {
	s.txTrackerLock.RLock()
	defer s.txTrackerLock.RUnlock()

	info, exists := s.txTracker[txHash]
	if !exists {
		return &TxTrackingInfo{
			TxHash: txHash,
			Status: TxStatusUnknown,
		}, nil
	}

	// Create a copy to avoid mutating shared state under read lock
	infoCopy := &TxTrackingInfo{
		TxHash:          info.TxHash,
		Status:          info.Status,
		InMempool:       info.InMempool,
		BlockHeight:     info.BlockHeight,
		MempoolPosition: info.MempoolPosition,
		Preconfirmation: info.Preconfirmation,
		Timestamp:       info.Timestamp,
		TxBytes:         info.TxBytes,
	}

	// Update status in the copy if preconfirmation expired
	if infoCopy.Preconfirmation != nil && time.Now().After(infoCopy.Preconfirmation.ExpiresAt) {
		infoCopy.Status = TxStatusExpired
	}

	return infoCopy, nil
}

// GetMempoolStats returns statistics about the mempool
func (s *PriorityTxService) GetMempoolStats() *MempoolStats {
	s.txTrackerLock.RLock()
	defer s.txTrackerLock.RUnlock()

	stats := &MempoolStats{
		TotalTxs: uint32(s.mempool.CountTx()),
	}

	var priorityCount uint32
	var preconfirmedCount uint32
	var totalPriorityLevel uint32

	for _, info := range s.txTracker {
		if info.InMempool {
			if info.Preconfirmation != nil {
				priorityCount++
				totalPriorityLevel += info.Preconfirmation.PriorityLevel

				if info.Status == TxStatusPreconfirmed {
					preconfirmedCount++
				}
			}
		}
	}

	stats.PriorityTxs = priorityCount
	// Defensively clamp to prevent underflow
	if priorityCount >= stats.TotalTxs {
		if priorityCount > stats.TotalTxs {
			s.logger.Warn("Priority count unexpectedly exceeds total transactions",
				"priorityCount", priorityCount,
				"totalTxs", stats.TotalTxs)
		}
		stats.NormalTxs = 0
	} else {
		stats.NormalTxs = stats.TotalTxs - priorityCount
	}
	stats.PreconfirmedTxs = preconfirmedCount

	if priorityCount > 0 {
		stats.AvgPriorityLevel = float32(totalPriorityLevel) / float32(priorityCount)
	}

	return stats
}

// ListPriorityTxs returns a list of priority transactions
func (s *PriorityTxService) ListPriorityTxs(limit uint32) []*PriorityTxInfoResult {
	s.txTrackerLock.RLock()
	defer s.txTrackerLock.RUnlock()

	var results []*PriorityTxInfoResult
	var count uint32

	for _, info := range s.txTracker {
		if info.InMempool && info.Preconfirmation != nil {
			results = append(results, &PriorityTxInfoResult{
				TxHash:          info.TxHash,
				PriorityLevel:   info.Preconfirmation.PriorityLevel,
				Timestamp:       info.Timestamp.Unix(),
				SizeBytes:       uint32(len(info.TxBytes)),
				Preconfirmation: info.Preconfirmation,
				Position:        info.MempoolPosition,
			})

			count++
			if limit > 0 && count >= limit {
				break
			}
		}
	}

	return results
}

// MarkTxIncluded marks a transaction as included in a block
func (s *PriorityTxService) MarkTxIncluded(txHash string, blockHeight int64) {
	s.txTrackerLock.Lock()
	defer s.txTrackerLock.Unlock()

	if info, exists := s.txTracker[txHash]; exists {
		info.Status = TxStatusIncluded
		info.InMempool = false
		info.BlockHeight = blockHeight

		s.logger.Info("transaction included in block",
			"tx_hash", txHash,
			"block_height", blockHeight,
		)
	}
}

// createPreconfirmation creates a preconfirmation for a transaction
func (s *PriorityTxService) createPreconfirmation(txHash string, priorityLevel uint32) *PreconfirmationInfo {
	now := time.Now()
	expiresAt := now.Add(s.preconfirmTimeout)

	preconf := &PreconfirmationInfo{
		TxHash:        txHash,
		Timestamp:     now,
		Validator:     s.validatorAddress,
		PriorityLevel: priorityLevel,
		ExpiresAt:     expiresAt,
		Signature:     s.signPreconfirmation(txHash, priorityLevel),
	}

	s.preconfirmationMutex.Lock()
	s.preconfirmations[txHash] = preconf
	s.preconfirmationMutex.Unlock()

	return preconf
}

// signPreconfirmation creates a cryptographic signature for the preconfirmation
// using the validator's private key (Ed25519 or secp256k1).
// Returns nil if no private key is configured.
func (s *PriorityTxService) signPreconfirmation(txHash string, priorityLevel uint32) []byte {
	// If no private key is configured, return nil (unsigned preconfirmation)
	if s.validatorPrivKey == nil {
		return nil
	}

	// Create the message to sign
	// Format: txHash | priorityLevel | validatorAddress | timestamp
	msg := s.createPreconfirmationMessage(txHash, priorityLevel)

	// Sign the message hash with the validator's private key
	signature, err := s.validatorPrivKey.Sign(msg)
	if err != nil {
		s.logger.Error("Failed to sign preconfirmation",
			"tx_hash", txHash,
			"error", err)
		return nil
	}

	s.logger.Debug("Preconfirmation signed",
		"tx_hash", txHash,
		"priority_level", priorityLevel,
		"key_type", s.validatorPrivKey.Type(),
		"signature_len", len(signature))

	return signature
}

// createPreconfirmationMessage creates the canonical message format for signing
// This ensures consistent message format for both signing and verification
func (s *PriorityTxService) createPreconfirmationMessage(txHash string, priorityLevel uint32) []byte {
	// Create a deterministic message format
	// Structure: PRECONFIRM | txHash | priorityLevel | validatorAddress

	msg := []byte("PRECONFIRM|")
	msg = append(msg, []byte(txHash)...)
	msg = append(msg, '|')

	// Add priority level as 4 bytes
	priorityBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(priorityBytes, priorityLevel)
	msg = append(msg, priorityBytes...)
	msg = append(msg, '|')

	msg = append(msg, []byte(s.validatorAddress)...)

	// Hash the message for signing
	hash := sha256.Sum256(msg)
	return hash[:]
}

// VerifyPreconfirmationSignature verifies a preconfirmation signature
// This can be used by clients to verify the authenticity of a preconfirmation
func (s *PriorityTxService) VerifyPreconfirmationSignature(
	txHash string,
	priorityLevel uint32,
	signature []byte,
	pubKey cryptotypes.PubKey,
) bool {
	if pubKey == nil || len(signature) == 0 {
		return false
	}

	msg := s.createPreconfirmationMessage(txHash, priorityLevel)
	return pubKey.VerifySignature(msg, signature)
}

// trackTransaction adds a transaction to the tracking system
func (s *PriorityTxService) trackTransaction(txHash string, txBytes []byte, preconf *PreconfirmationInfo, position uint32) {
	s.txTrackerLock.Lock()
	defer s.txTrackerLock.Unlock()

	s.txTracker[txHash] = &TxTrackingInfo{
		TxHash:          txHash,
		Status:          TxStatusPreconfirmed,
		InMempool:       true,
		MempoolPosition: position,
		Preconfirmation: preconf,
		Timestamp:       time.Now(),
		TxBytes:         txBytes,
	}
}

// calculateTxHash computes the hash of a transaction
func (s *PriorityTxService) calculateTxHash(txBytes []byte) string {
	hash := sha256.Sum256(txBytes)
	return hex.EncodeToString(hash[:])
}

// isTxInMempool checks if a transaction is already in the mempool
func (s *PriorityTxService) isTxInMempool(txHash string) bool {
	s.txTrackerLock.RLock()
	defer s.txTrackerLock.RUnlock()

	if info, exists := s.txTracker[txHash]; exists {
		return info.InMempool
	}
	return false
}

// countPriorityTxsInMempool counts the number of priority transactions currently in mempool
// These are transactions that have received the priority boost (DefaultPriorityBoost = 1_000_000_000)
// and are waiting to be included in a block
func (s *PriorityTxService) countPriorityTxsInMempool() uint32 {
	s.txTrackerLock.RLock()
	defer s.txTrackerLock.RUnlock()

	// Count transactions that are:
	// 1. Currently in the mempool (InMempool = true)
	// 2. Have a preconfirmation (meaning they were submitted as priority level 1 txs)
	// 3. These transactions have their priority boosted by DefaultPriorityBoost in the mempool
	var count uint32
	for _, info := range s.txTracker {
		if info.InMempool && info.Preconfirmation != nil {
			count++
		}
	}

	s.logger.Debug("counted priority transactions in mempool",
		"count", count,
		"total_mempool_txs", s.mempool.CountTx(),
		"priority_boost", DefaultPriorityBoost,
	)

	return count
}

// cleanupExpiredPreconfirmations periodically removes expired preconfirmations
func (s *PriorityTxService) cleanupExpiredPreconfirmations() {
	defer close(s.done)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("cleanup goroutine shutting down")
			ticker.Stop()
			return
		case <-ticker.C:
			// Acquire locks in consistent order: txTrackerLock first, then preconfirmationMutex
			// This prevents deadlock with other code paths
			s.txTrackerLock.Lock()
			s.preconfirmationMutex.Lock()

			now := time.Now()
			for txHash, preconf := range s.preconfirmations {
				if now.After(preconf.ExpiresAt) {
					delete(s.preconfirmations, txHash)

					if info, exists := s.txTracker[txHash]; exists && info.Status == TxStatusPreconfirmed {
						info.Status = TxStatusExpired
					}

					s.logger.Debug("cleaned up expired preconfirmation", "tx_hash", txHash)
				}
			}

			// Unlock in reverse order
			s.preconfirmationMutex.Unlock()
			s.txTrackerLock.Unlock()
		}
	}
}

// Stop gracefully shuts down the service and waits for cleanup goroutine to finish
func (s *PriorityTxService) Stop() {
	s.logger.Info("stopping priority tx service")
	s.cancel()
	<-s.done
	s.logger.Info("priority tx service stopped")
}

// Result types

// SubmitPriorityTxResult is the result of submitting a priority transaction
type SubmitPriorityTxResult struct {
	TxHash          string
	Accepted        bool
	Reason          string
	Preconfirmation *PreconfirmationInfo
	MempoolPosition uint32
}

// MempoolStats contains mempool statistics
type MempoolStats struct {
	TotalTxs         uint32
	PriorityTxs      uint32
	NormalTxs        uint32
	PreconfirmedTxs  uint32
	AvgPriorityLevel float32
	MempoolSizeBytes uint64
}

// PriorityTxInfoResult contains information about a priority transaction
type PriorityTxInfoResult struct {
	TxHash          string
	PriorityLevel   uint32
	Timestamp       int64
	SizeBytes       uint32
	Preconfirmation *PreconfirmationInfo
	Position        uint32
}

// Key loading utilities

// LoadPrivKeyFromBytes loads a private key from raw bytes
// For Ed25519: expects 64 bytes (32-byte private key + 32-byte public key) or 32 bytes (private key only, will derive public key)
// For secp256k1: expects 32 bytes (private key)
// Defaults to Ed25519 if keyType is not specified
func LoadPrivKeyFromBytes(keyBytes []byte, keyType string) (cryptotypes.PrivKey, error) {
	switch keyType {
	case "ed25519", "":
		// Ed25519 is the default for Cosmos SDK validators
		if len(keyBytes) == 32 {
			// If only private key is provided, we need to derive the public key
			// Cosmos SDK Ed25519 PrivKey expects the full 64 bytes (private + public)
			privKey := ed25519.GenPrivKeyFromSecret(keyBytes)
			return privKey, nil
		} else if len(keyBytes) == 64 {
			// Full key provided (private + public)
			return &ed25519.PrivKey{Key: keyBytes}, nil
		} else {
			return nil, fmt.Errorf("invalid Ed25519 key length: expected 32 or 64 bytes, got %d", len(keyBytes))
		}
	case "secp256k1":
		if len(keyBytes) != 32 {
			return nil, fmt.Errorf("invalid secp256k1 key length: expected 32 bytes, got %d", len(keyBytes))
		}
		return &secp256k1.PrivKey{Key: keyBytes}, nil
	default:
		return nil, fmt.Errorf("unsupported key type: %s (supported: ed25519, secp256k1)", keyType)
	}
}

// LoadPrivKeyFromHex loads a private key from a hex-encoded string
func LoadPrivKeyFromHex(hexKey, keyType string) (cryptotypes.PrivKey, error) {
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid hex key: %w", err)
	}
	return LoadPrivKeyFromBytes(keyBytes, keyType)
}

// GetPublicKey returns the public key for the validator if a private key is configured
func (s *PriorityTxService) GetPublicKey() cryptotypes.PubKey {
	if s.validatorPrivKey == nil {
		return nil
	}
	return s.validatorPrivKey.PubKey()
}

// IsSigningEnabled returns whether the service is configured with a private key for signing
func (s *PriorityTxService) IsSigningEnabled() bool {
	return s.validatorPrivKey != nil
}

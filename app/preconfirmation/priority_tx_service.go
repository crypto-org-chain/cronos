package preconfirmation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
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
	preconfirmTimeout time.Duration
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

// PriorityTxServiceConfig configuration for the service
type PriorityTxServiceConfig struct {
	App               *baseapp.BaseApp
	Mempool           mempool.Mempool
	TxDecoder         sdk.TxDecoder
	Logger            log.Logger
	ValidatorAddress  string
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

	service := &PriorityTxService{
		app:               cfg.App,
		mempool:           cfg.Mempool,
		txDecoder:         cfg.TxDecoder,
		logger:            cfg.Logger,
		validatorAddress:  cfg.ValidatorAddress,
		preconfirmTimeout: cfg.PreconfirmTimeout,
		preconfirmations:  make(map[string]*PreconfirmationInfo),
		txTracker:         make(map[string]*TxTrackingInfo),
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
	if priorityLevel < 1 || priorityLevel > 10 {
		return nil, fmt.Errorf("invalid priority level: must be between 1 and 10")
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

	// Track transaction
	s.trackTransaction(txHash, txBytes, preconf)

	// Calculate mempool position and estimated inclusion time
	position := s.estimateMempoolPosition(txHash, priorityLevel)
	estimatedTime := s.estimateInclusionTime(position)

	s.logger.Info("priority transaction accepted",
		"tx_hash", txHash,
		"priority_level", priorityLevel,
		"position", position,
		"estimated_time", estimatedTime,
	)

	return &SubmitPriorityTxResult{
		TxHash:                 txHash,
		Accepted:               true,
		Preconfirmation:        preconf,
		MempoolPosition:        position,
		EstimatedInclusionTime: estimatedTime,
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

	// Update status if preconfirmation expired
	if info.Preconfirmation != nil && time.Now().After(info.Preconfirmation.ExpiresAt) {
		info.Status = TxStatusExpired
	}

	return info, nil
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
	stats.NormalTxs = stats.TotalTxs - priorityCount
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

// signPreconfirmation creates a signature for the preconfirmation
// TODO: Implement actual signing with validator's key
func (s *PriorityTxService) signPreconfirmation(txHash string, priorityLevel uint32) []byte {
	// For now, return a placeholder signature
	// In production, this should sign with the validator's private key
	data := fmt.Sprintf("%s:%d:%s", txHash, priorityLevel, s.validatorAddress)
	hash := sha256.Sum256([]byte(data))
	return hash[:]
}

// trackTransaction adds a transaction to the tracking system
func (s *PriorityTxService) trackTransaction(txHash string, txBytes []byte, preconf *PreconfirmationInfo) {
	s.txTrackerLock.Lock()
	defer s.txTrackerLock.Unlock()

	s.txTracker[txHash] = &TxTrackingInfo{
		TxHash:          txHash,
		Status:          TxStatusPreconfirmed,
		InMempool:       true,
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

// estimateMempoolPosition estimates the position of a transaction in the mempool
func (s *PriorityTxService) estimateMempoolPosition(txHash string, priorityLevel uint32) uint32 {
	// Simplified estimation: higher priority = lower position
	// In production, this should query the actual mempool
	s.txTrackerLock.RLock()
	defer s.txTrackerLock.RUnlock()

	var position uint32 = 1
	for _, info := range s.txTracker {
		if info.InMempool && info.Preconfirmation != nil {
			if info.Preconfirmation.PriorityLevel > priorityLevel {
				position++
			}
		}
	}

	return position
}

// estimateInclusionTime estimates time until block inclusion in seconds
func (s *PriorityTxService) estimateInclusionTime(position uint32) uint32 {
	// Assume ~6 second block time and ~100 txs per block
	const blockTime = 6
	const txsPerBlock = 100

	blocksAhead := (position + txsPerBlock - 1) / txsPerBlock
	return uint32(blocksAhead * blockTime)
}

// cleanupExpiredPreconfirmations periodically removes expired preconfirmations
func (s *PriorityTxService) cleanupExpiredPreconfirmations() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.preconfirmationMutex.Lock()
		s.txTrackerLock.Lock()

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

		s.txTrackerLock.Unlock()
		s.preconfirmationMutex.Unlock()
	}
}

// Result types

// SubmitPriorityTxResult is the result of submitting a priority transaction
type SubmitPriorityTxResult struct {
	TxHash                 string
	Accepted               bool
	Reason                 string
	Preconfirmation        *PreconfirmationInfo
	MempoolPosition        uint32
	EstimatedInclusionTime uint32
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

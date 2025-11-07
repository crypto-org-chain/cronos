package relayer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cosmossdk.io/log"
)

// CheckpointState contains the relayer's monitoring state for recovery
type CheckpointState struct {
	// Last finalized block height for Cronos EVM chain
	LastFinalityBlockHeight uint64 `json:"last_finality_block_height"`

	// Pending attestations (by tx hash)
	PendingAttestations map[string]*PendingAttestation `json:"pending_attestations"`

	// Timestamp of last checkpoint
	CheckpointedAt time.Time `json:"checkpointed_at"`

	// Relayer version/build info
	Version string `json:"version,omitempty"`
}

// PendingAttestation represents a pending attestation for recovery
type PendingAttestation struct {
	TxHash         string    `json:"tx_hash"`
	AttestationIDs []uint64  `json:"attestation_ids,omitempty"`
	AttestationID  uint64    `json:"attestation_id,omitempty"` // For single attestation
	ChainID        string    `json:"chain_id"`
	BlockHeight    uint64    `json:"block_height"`
	StartHeight    uint64    `json:"start_height,omitempty"` // For batch
	EndHeight      uint64    `json:"end_height,omitempty"`   // For batch
	SubmittedAt    time.Time `json:"submitted_at"`
}

// CheckpointManager manages checkpoint state persistence
type CheckpointManager struct {
	checkpointPath string
	logger         log.Logger
	mu             sync.RWMutex

	// Current state
	state *CheckpointState

	// Auto-save configuration
	autoSaveInterval time.Duration
	stopCh           chan struct{}
}

// NewCheckpointManager creates a new checkpoint manager
func NewCheckpointManager(checkpointPath string, logger log.Logger, autoSaveInterval time.Duration) (*CheckpointManager, error) {
	if checkpointPath == "" {
		return nil, fmt.Errorf("checkpoint path cannot be empty")
	}

	// Ensure directory exists
	dir := filepath.Dir(checkpointPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create checkpoint directory: %w", err)
	}

	cm := &CheckpointManager{
		checkpointPath:   checkpointPath,
		logger:           logger.With("component", "checkpoint_manager"),
		autoSaveInterval: autoSaveInterval,
		stopCh:           make(chan struct{}),
		state: &CheckpointState{
			PendingAttestations: make(map[string]*PendingAttestation),
		},
	}

	// Try to load existing checkpoint
	if err := cm.Load(); err != nil {
		if os.IsNotExist(err) {
			cm.logger.Info("No existing checkpoint found, starting fresh")
		} else {
			cm.logger.Warn("Failed to load checkpoint, starting fresh", "error", err)
		}
	}

	return cm, nil
}

// Start begins the auto-save goroutine
func (cm *CheckpointManager) Start(ctx context.Context) error {
	cm.logger.Info("Starting checkpoint manager",
		"path", cm.checkpointPath,
		"auto_save_interval", cm.autoSaveInterval,
	)

	go cm.autoSaveLoop(ctx)

	return nil
}

// Stop stops the checkpoint manager and performs final save
func (cm *CheckpointManager) Stop() error {
	cm.logger.Info("Stopping checkpoint manager")

	close(cm.stopCh)

	// Perform final checkpoint save
	if err := cm.Save(); err != nil {
		cm.logger.Error("Failed to save final checkpoint", "error", err)
		return err
	}

	cm.logger.Info("Checkpoint manager stopped")
	return nil
}

// autoSaveLoop periodically saves the checkpoint
func (cm *CheckpointManager) autoSaveLoop(ctx context.Context) {
	ticker := time.NewTicker(cm.autoSaveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-cm.stopCh:
			return
		case <-ticker.C:
			if err := cm.Save(); err != nil {
				cm.logger.Error("Auto-save checkpoint failed", "error", err)
			} else {
				cm.logger.Debug("Auto-saved checkpoint")
			}
		}
	}
}

// UpdateLastFinalityBlockHeight updates the last finalized block height
func (cm *CheckpointManager) UpdateLastFinalityBlockHeight(height uint64) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if height > cm.state.LastFinalityBlockHeight {
		cm.state.LastFinalityBlockHeight = height
		cm.logger.Debug("Updated last finality block height", "height", height)
	}
}

// GetLastFinalityBlockHeight returns the last finalized block height
func (cm *CheckpointManager) GetLastFinalityBlockHeight() uint64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.state.LastFinalityBlockHeight
}

// AddPendingAttestation adds a pending attestation to the checkpoint
func (cm *CheckpointManager) AddPendingAttestation(pa *PendingAttestation) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if pa.TxHash != "" {
		cm.state.PendingAttestations[pa.TxHash] = pa
		cm.logger.Debug("Added pending attestation to checkpoint",
			"tx_hash", pa.TxHash,
			"chain_id", pa.ChainID,
			"start_height", pa.StartHeight,
			"end_height", pa.EndHeight,
		)
	}
}

// RemovePendingAttestation removes a pending attestation from the checkpoint
func (cm *CheckpointManager) RemovePendingAttestation(txHash, chainID string, blockHeight uint64) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if txHash != "" {
		delete(cm.state.PendingAttestations, txHash)
		cm.logger.Debug("Removed pending attestation from checkpoint",
			"tx_hash", txHash,
			"chain_id", chainID,
			"block_height", blockHeight,
		)
	}
}

// GetPendingAttestations returns a copy of all pending attestations
func (cm *CheckpointManager) GetPendingAttestations() map[string]*PendingAttestation {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Return copies to avoid race conditions
	byTxHash := make(map[string]*PendingAttestation, len(cm.state.PendingAttestations))
	for k, v := range cm.state.PendingAttestations {
		copy := *v
		byTxHash[k] = &copy
	}

	return byTxHash
}

// Save persists the current checkpoint state to disk
func (cm *CheckpointManager) Save() error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Update checkpoint timestamp
	cm.state.CheckpointedAt = time.Now()

	// Marshal to JSON
	data, err := json.MarshalIndent(cm.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint state: %w", err)
	}

	// Write to temporary file first (atomic write)
	tmpPath := cm.checkpointPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write temporary checkpoint: %w", err)
	}

	// Rename to actual checkpoint file (atomic operation)
	if err := os.Rename(tmpPath, cm.checkpointPath); err != nil {
		return fmt.Errorf("failed to rename checkpoint file: %w", err)
	}

	cm.logger.Debug("Saved checkpoint",
		"path", cm.checkpointPath,
		"last_finality_height", cm.state.LastFinalityBlockHeight,
		"pending_count", len(cm.state.PendingAttestations),
	)

	return nil
}

// Load loads the checkpoint state from disk
func (cm *CheckpointManager) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Read checkpoint file
	data, err := os.ReadFile(cm.checkpointPath)
	if err != nil {
		return err
	}

	// Unmarshal JSON
	var state CheckpointState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal checkpoint state: %w", err)
	}

	// Initialize maps if nil
	if state.PendingAttestations == nil {
		state.PendingAttestations = make(map[string]*PendingAttestation)
	}

	cm.state = &state

	cm.logger.Info("Loaded checkpoint",
		"path", cm.checkpointPath,
		"last_finality_height", state.LastFinalityBlockHeight,
		"pending_count", len(state.PendingAttestations),
		"checkpointed_at", state.CheckpointedAt,
	)

	return nil
}

// GetState returns a copy of the current checkpoint state
func (cm *CheckpointManager) GetState() CheckpointState {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Return a deep copy
	stateCopy := CheckpointState{
		LastFinalityBlockHeight: cm.state.LastFinalityBlockHeight,
		CheckpointedAt:          cm.state.CheckpointedAt,
		Version:                 cm.state.Version,
		PendingAttestations:     make(map[string]*PendingAttestation, len(cm.state.PendingAttestations)),
	}

	for k, v := range cm.state.PendingAttestations {
		copy := *v
		stateCopy.PendingAttestations[k] = &copy
	}

	return stateCopy
}

// Clear clears all checkpoint state
func (cm *CheckpointManager) Clear() error {
	cm.mu.Lock()
	cm.state = &CheckpointState{
		PendingAttestations: make(map[string]*PendingAttestation),
	}
	cm.mu.Unlock()

	cm.logger.Info("Cleared checkpoint state")

	// Save the cleared state (Save has its own lock)
	return cm.Save()
}

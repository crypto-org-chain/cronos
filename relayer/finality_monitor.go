package relayer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"

	"cosmossdk.io/log"
)

const (
	// Event types from attestation chain
	eventTypeBatchAttested = "attestation.v1.EventBatchBlockAttested"

	// Event query for subscription
	finalityEventQuery = "tm.event = 'Tx'"

	// Timeout for pending attestation tracking
	pendingAttestationTimeout = 5 * time.Minute
)

// pendingAttestation tracks an attestation that was submitted but not yet confirmed
type pendingAttestation struct {
	TxHash        string
	AttestationID uint64
	ChainID       string
	BlockHeight   uint64
	SubmittedAt   time.Time
}

// finalityMonitor implements FinalityMonitor interface
type finalityMonitor struct {
	client        rpcclient.Client
	config        *Config
	logger        log.Logger
	finalityStore FinalityStore

	running       bool
	stopCh        chan struct{}
	subscribers   []chan *FinalityInfo
	subscribersMu sync.RWMutex

	eventCh <-chan coretypes.ResultEvent

	// Track pending attestations
	pendingAttestations map[string]*pendingAttestation // key: tx_hash
	pendingMu           sync.RWMutex

	// Checkpoint manager for crash recovery
	checkpointManager *CheckpointManager
}

// NewFinalityMonitor creates a new finality monitor
func NewFinalityMonitor(
	client rpcclient.Client,
	config *Config,
	logger log.Logger,
	finalityStore FinalityStore,
) (FinalityMonitor, error) {
	// Create checkpoint manager
	checkpointPath := config.CheckpointPath
	if checkpointPath == "" {
		checkpointPath = "./data/relayer_checkpoint.json"
	}

	checkpointManager, err := NewCheckpointManager(
		checkpointPath,
		logger,
		30*time.Second, // Auto-save every 30 seconds
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create checkpoint manager: %w", err)
	}

	fm := &finalityMonitor{
		client:              client,
		config:              config,
		logger:              logger.With("component", "finality_monitor"),
		finalityStore:       finalityStore,
		stopCh:              make(chan struct{}),
		subscribers:         make([]chan *FinalityInfo, 0),
		pendingAttestations: make(map[string]*pendingAttestation),
		checkpointManager:   checkpointManager,
	}

	// Restore pending attestations from checkpoint
	if err := fm.restoreFromCheckpoint(); err != nil {
		logger.Warn("Failed to restore from checkpoint", "error", err)
	}

	return fm, nil
}

// TrackBatchAttestation tracks a batch attestation (called by block forwarder)
func (fm *finalityMonitor) TrackBatchAttestation(txHash string, attestationIDs []uint64, chainID string, startHeight uint64, endHeight uint64) {
	fm.pendingMu.Lock()
	defer fm.pendingMu.Unlock()

	// Track the batch transaction by tx hash
	pending := &pendingAttestation{
		TxHash:      txHash,
		ChainID:     chainID,
		BlockHeight: startHeight, // Use start height as primary
		SubmittedAt: time.Now(),
	}
	fm.pendingAttestations[txHash] = pending

	fm.logger.Info("Tracking batch attestation",
		"tx_hash", txHash,
		"chain_id", chainID,
		"start_height", startHeight,
		"end_height", endHeight,
		"attestation_count", len(attestationIDs),
	)

	// Save to checkpoint
	if fm.checkpointManager != nil {
		fm.checkpointManager.AddPendingAttestation(&PendingAttestation{
			TxHash:         txHash,
			AttestationIDs: attestationIDs,
			ChainID:        chainID,
			BlockHeight:    startHeight,
			StartHeight:    startHeight,
			EndHeight:      endHeight,
			SubmittedAt:    pending.SubmittedAt,
		})
	}
}

// Start begins monitoring finality events
func (fm *finalityMonitor) Start(ctx context.Context) error {
	if fm.running {
		return fmt.Errorf("finality monitor already running")
	}

	fm.logger.Info("Starting finality monitor")

	// Start CometBFT client if not already started
	if !fm.client.IsRunning() {
		if err := fm.client.Start(); err != nil {
			return fmt.Errorf("failed to start RPC client: %w", err)
		}
	}

	// Subscribe to transaction events (which contain our custom events)
	eventCh, err := fm.client.Subscribe(ctx, "finality-monitor", finalityEventQuery)
	if err != nil {
		return fmt.Errorf("failed to subscribe to events: %w", err)
	}
	fm.eventCh = eventCh

	fm.running = true

	// Start event processing goroutine
	go fm.processEvents(ctx)

	// Start cleanup goroutine for expired pending attestations
	go fm.cleanupExpiredAttestations(ctx)

	fm.logger.Info("Finality monitor started successfully")
	return nil
}

// Stop stops the finality monitor
func (fm *finalityMonitor) Stop() error {
	if !fm.running {
		return nil
	}

	fm.logger.Info("Stopping finality monitor")

	// Signal stop
	close(fm.stopCh)

	// Unsubscribe from events
	if fm.client != nil && fm.client.IsRunning() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := fm.client.Unsubscribe(ctx, "finality-monitor", finalityEventQuery); err != nil {
			fm.logger.Warn("Failed to unsubscribe from events", "error", err)
		}
	}

	// Close subscriber channels
	fm.subscribersMu.Lock()
	for _, ch := range fm.subscribers {
		close(ch)
	}
	fm.subscribers = nil
	fm.subscribersMu.Unlock()

	fm.running = false
	fm.logger.Info("Finality monitor stopped")
	return nil
}

// processEvents processes events from the attestation chain
func (fm *finalityMonitor) processEvents(ctx context.Context) {
	fm.logger.Info("Event processing goroutine started")

	for {
		select {
		case <-ctx.Done():
			fm.logger.Info("Context cancelled, stopping event processing")
			return

		case <-fm.stopCh:
			fm.logger.Info("Stop signal received, stopping event processing")
			return

		case event := <-fm.eventCh:
			if event.Data == nil {
				continue
			}

			// Try to get tx hash from the event
			if resultTx, ok := event.Data.(coretypes.ResultTx); ok {
				txHash := resultTx.Hash.String()
				fm.processTxResult(txHash, &resultTx.TxResult)
			}
		}
	}
}

// processTxResult processes a transaction result
func (fm *finalityMonitor) processTxResult(txHash string, txResult *abci.ExecTxResult) {
	if txResult == nil {
		return
	}

	fm.logger.Debug("Processing transaction result",
		"tx_hash", txHash,
		"code", txResult.Code,
		"events", len(txResult.Events),
	)

	// Check if this is a pending attestation
	fm.pendingMu.RLock()
	pending, isPending := fm.pendingAttestations[txHash]
	fm.pendingMu.RUnlock()

	if isPending {
		fm.logger.Info("Found pending attestation transaction",
			"tx_hash", txHash,
			"chain_id", pending.ChainID,
			"block_height", pending.BlockHeight,
		)
	}

	// Process events in the transaction
	for _, event := range txResult.Events {
		switch event.Type {
		case eventTypeBatchAttested:
			fm.handleBatchAttestedEvent(txHash, event)
		}
	}
}

// handleBatchAttestedEvent handles EventBatchBlockAttested
func (fm *finalityMonitor) handleBatchAttestedEvent(txHash string, event abci.Event) {
	fm.logger.Debug("Processing batch attested event", "type", event.Type, "tx_hash", txHash)

	// Parse event attributes
	var (
		chainID        string
		startHeight    uint64
		endHeight      uint64
		blockCount     uint32
		finalizedCount uint32
		relayer        string
		attestationIDs []uint64
	)

	for _, attr := range event.Attributes {
		key := string(attr.Key)
		value := string(attr.Value)

		switch key {
		case "chain_id":
			chainID = value
		case "start_height":
			if height, err := strconv.ParseUint(value, 10, 64); err == nil {
				startHeight = height
			}
		case "end_height":
			if height, err := strconv.ParseUint(value, 10, 64); err == nil {
				endHeight = height
			}
		case "block_count":
			if count, err := strconv.ParseUint(value, 10, 32); err == nil {
				blockCount = uint32(count)
			}
		case "finalized_count":
			if count, err := strconv.ParseUint(value, 10, 32); err == nil {
				finalizedCount = uint32(count)
			}
		case "relayer":
			relayer = value
		case "attestation_ids":
			// Parse JSON array of IDs
			if err := json.Unmarshal([]byte(value), &attestationIDs); err != nil {
				fm.logger.Warn("Failed to parse attestation IDs", "error", err)
			}
		}
	}

	// Verify this matches a pending batch attestation
	fm.pendingMu.Lock()
	var latency time.Duration
	if txHash != "" {
		if pending, exists := fm.pendingAttestations[txHash]; exists {
			latency = time.Since(pending.SubmittedAt)
			fm.logger.Info("Batch attestation confirmed",
				"tx_hash", txHash,
				"chain_id", chainID,
				"start_height", startHeight,
				"end_height", endHeight,
				"block_count", blockCount,
				"latency", latency,
			)
			delete(fm.pendingAttestations, txHash)

			// Remove from checkpoint
			if fm.checkpointManager != nil {
				fm.checkpointManager.RemovePendingAttestation(txHash, chainID, startHeight)
			}
		}
	}
	fm.pendingMu.Unlock()

	fm.logger.Info("Batch blocks attested",
		"chain_id", chainID,
		"start_height", startHeight,
		"end_height", endHeight,
		"block_count", blockCount,
		"finalized_count", finalizedCount,
		"relayer", relayer,
		"attestation_ids", attestationIDs,
		"tx_hash", txHash,
	)

	// Store finality info for each block in the batch
	processedAt := time.Now().Unix()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for i, attestationID := range attestationIDs {
		height := startHeight + uint64(i)
		if height > endHeight {
			break
		}

		finalityInfo := &FinalityInfo{
			AttestationID:     attestationID,
			ChainID:           chainID,
			BlockHeight:       height,
			Finalized:         true,
			FinalizedAt:       processedAt,
			AttestationTxHash: []byte(txHash),
		}

		if err := fm.finalityStore.SaveFinalityInfo(ctx, finalityInfo); err != nil {
			fm.logger.Error("Failed to store batch finality info",
				"error", err,
				"block_height", height,
			)
		} else {
			// Notify subscribers
			fm.notifySubscribers(finalityInfo)
		}
	}
}

// cleanupExpiredAttestations removes expired pending attestations
func (fm *finalityMonitor) cleanupExpiredAttestations(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-fm.stopCh:
			return
		case <-ticker.C:
			fm.cleanupExpired()
		}
	}
}

// cleanupExpired removes expired pending attestations
func (fm *finalityMonitor) cleanupExpired() {
	fm.pendingMu.Lock()
	defer fm.pendingMu.Unlock()

	now := time.Now()
	expiredCount := 0

	// Check each pending attestation
	for txHash, pending := range fm.pendingAttestations {
		if now.Sub(pending.SubmittedAt) > pendingAttestationTimeout {
			fm.logger.Warn("Pending attestation expired",
				"tx_hash", txHash,
				"chain_id", pending.ChainID,
				"block_height", pending.BlockHeight,
				"age", now.Sub(pending.SubmittedAt),
			)
			delete(fm.pendingAttestations, txHash)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		fm.logger.Info("Cleaned up expired attestations", "count", expiredCount)
	}
}

// GetPendingAttestations returns the count of pending attestations
func (fm *finalityMonitor) GetPendingAttestations() int {
	fm.pendingMu.RLock()
	defer fm.pendingMu.RUnlock()
	return len(fm.pendingAttestations)
}

// notifySubscribers sends finality info to all subscribers
func (fm *finalityMonitor) notifySubscribers(info *FinalityInfo) {
	fm.subscribersMu.RLock()
	defer fm.subscribersMu.RUnlock()

	for _, ch := range fm.subscribers {
		select {
		case ch <- info:
			// Successfully sent
		default:
			// Channel is full, skip
			fm.logger.Warn("Subscriber channel full, skipping notification")
		}
	}
}

// SubscribeFinality returns a channel that receives finality updates
func (fm *finalityMonitor) SubscribeFinality(ctx context.Context) (<-chan *FinalityInfo, error) {
	finalityCh := make(chan *FinalityInfo, 100)

	fm.subscribersMu.Lock()
	fm.subscribers = append(fm.subscribers, finalityCh)
	fm.subscribersMu.Unlock()

	fm.logger.Debug("New finality subscriber added",
		"total_subscribers", len(fm.subscribers),
	)

	return finalityCh, nil
}

// GetFinalityStatus retrieves finality status for a block
func (fm *finalityMonitor) GetFinalityStatus(ctx context.Context, chainID string, height uint64) (*FinalityInfo, error) {
	fm.logger.Debug("GetFinalityStatus called",
		"chain_id", chainID,
		"block_height", height,
	)

	// Query from finality store directly
	// (pending attestations are tracked by tx hash, not individual blocks)
	return fm.finalityStore.GetFinalityInfo(ctx, chainID, height)
}

// TrackBatchAttestationFinalized tracks a batch attestation that's already finalized (sync mode)
func (fm *finalityMonitor) TrackBatchAttestationFinalized(
	txHash string,
	attestationIDs []uint64,
	chainID string,
	firstHeight, lastHeight uint64,
	finalizedCount uint32,
) {
	fm.logger.Info("Tracking finalized batch attestation (sync mode)",
		"tx_hash", txHash,
		"chain_id", chainID,
		"first_height", firstHeight,
		"last_height", lastHeight,
		"attestation_count", len(attestationIDs),
		"finalized_count", finalizedCount,
	)

	// In sync mode, blocks are already finalized on the attestation chain
	// We can immediately mark them as finalized and notify subscribers

	finalizedAt := time.Now().Unix()

	// Create finality info for each block in the batch
	for i, attestationID := range attestationIDs {
		blockHeight := firstHeight + uint64(i)

		finalityInfo := &FinalityInfo{
			AttestationID:     attestationID,
			ChainID:           chainID,
			BlockHeight:       blockHeight,
			Finalized:         true,
			FinalizedAt:       finalizedAt,
			AttestationTxHash: []byte(txHash),
		}

		// Save to finality store if available
		if fm.finalityStore != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := fm.finalityStore.SaveFinalityInfo(ctx, finalityInfo); err != nil {
				fm.logger.Error("Failed to save finality info to store",
					"error", err,
					"chain_id", chainID,
					"height", blockHeight,
					"attestation_id", attestationID,
				)
			}
			cancel()
		}

		// Update checkpoint with last finality height
		if fm.checkpointManager != nil {
			fm.checkpointManager.UpdateLastFinalityBlockHeight(blockHeight)
		}

		// Emit finality event to subscribers
		fm.subscribersMu.RLock()
		for _, sub := range fm.subscribers {
			select {
			case sub <- finalityInfo:
				fm.logger.Debug("Emitted finality event to subscriber",
					"chain_id", chainID,
					"height", blockHeight,
					"attestation_id", attestationID,
				)
			default:
				fm.logger.Warn("Subscriber channel full, dropping finality event",
					"chain_id", chainID,
					"height", blockHeight,
				)
			}
		}
		fm.subscribersMu.RUnlock()
	}

	fm.logger.Info("Finalized batch attestation tracked successfully",
		"tx_hash", txHash,
		"chain_id", chainID,
		"blocks_finalized", len(attestationIDs),
		"finalized_count", finalizedCount,
	)
}

// restoreFromCheckpoint restores pending attestations from checkpoint
func (fm *finalityMonitor) restoreFromCheckpoint() error {
	byTxHash := fm.checkpointManager.GetPendingAttestations()

	fm.pendingMu.Lock()
	defer fm.pendingMu.Unlock()

	// Restore pending attestations by tx hash
	for txHash, pa := range byTxHash {
		fm.pendingAttestations[txHash] = &pendingAttestation{
			TxHash:        pa.TxHash,
			AttestationID: pa.AttestationID,
			ChainID:       pa.ChainID,
			BlockHeight:   pa.BlockHeight,
			SubmittedAt:   pa.SubmittedAt,
		}
	}

	fm.logger.Info("Restored from checkpoint",
		"last_finality_height", fm.checkpointManager.GetLastFinalityBlockHeight(),
		"pending_attestations", len(fm.pendingAttestations),
	)

	return nil
}

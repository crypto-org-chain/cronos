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
	eventTypeBlockAttested = "attestation.v1.EventBlockAttested"
	eventTypeBatchAttested = "attestation.v1.EventBatchBlockAttested"

	// Alternative event type formats
	eventTypeBlockAttestedAlt = "block_attested"

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
	pendingByBlock      map[string]*pendingAttestation // key: chainID:blockHeight
	pendingMu           sync.RWMutex
}

// NewFinalityMonitor creates a new finality monitor
func NewFinalityMonitor(
	client rpcclient.Client,
	config *Config,
	logger log.Logger,
	finalityStore FinalityStore,
) (FinalityMonitor, error) {
	return &finalityMonitor{
		client:              client,
		config:              config,
		logger:              logger.With("component", "finality_monitor"),
		finalityStore:       finalityStore,
		stopCh:              make(chan struct{}),
		subscribers:         make([]chan *FinalityInfo, 0),
		pendingAttestations: make(map[string]*pendingAttestation),
		pendingByBlock:      make(map[string]*pendingAttestation),
	}, nil
}

// TrackAttestation tracks a submitted attestation (called by block forwarder)
func (fm *finalityMonitor) TrackAttestation(txHash string, attestationID uint64, chainID string, blockHeight uint64) {
	fm.pendingMu.Lock()
	defer fm.pendingMu.Unlock()

	pending := &pendingAttestation{
		TxHash:        txHash,
		AttestationID: attestationID,
		ChainID:       chainID,
		BlockHeight:   blockHeight,
		SubmittedAt:   time.Now(),
	}

	fm.pendingAttestations[txHash] = pending
	blockKey := fmt.Sprintf("%s:%d", chainID, blockHeight)
	fm.pendingByBlock[blockKey] = pending

	fm.logger.Debug("Tracking attestation",
		"tx_hash", txHash,
		"attestation_id", attestationID,
		"chain_id", chainID,
		"block_height", blockHeight,
	)
}

// TrackBatchAttestation tracks a batch attestation (called by block forwarder)
func (fm *finalityMonitor) TrackBatchAttestation(txHash string, attestationIDs []uint64, chainID string, startHeight uint64, endHeight uint64) {
	fm.pendingMu.Lock()
	defer fm.pendingMu.Unlock()

	// Track the batch transaction
	pending := &pendingAttestation{
		TxHash:      txHash,
		ChainID:     chainID,
		BlockHeight: startHeight, // Use start height as primary
		SubmittedAt: time.Now(),
	}
	fm.pendingAttestations[txHash] = pending

	// Track each block in the batch
	for i, attestationID := range attestationIDs {
		height := startHeight + uint64(i)
		if height > endHeight {
			break
		}

		blockPending := &pendingAttestation{
			TxHash:        txHash,
			AttestationID: attestationID,
			ChainID:       chainID,
			BlockHeight:   height,
			SubmittedAt:   time.Now(),
		}

		blockKey := fmt.Sprintf("%s:%d", chainID, height)
		fm.pendingByBlock[blockKey] = blockPending
	}

	fm.logger.Debug("Tracking batch attestation",
		"tx_hash", txHash,
		"attestation_ids", attestationIDs,
		"chain_id", chainID,
		"start_height", startHeight,
		"end_height", endHeight,
	)
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

			// Extract transaction result
			var txHash string

			// Try to get tx hash from the event
			if resultTx, ok := event.Data.(coretypes.ResultTx); ok {
				txHash = resultTx.Hash.String()
				fm.processTxResult(txHash, &resultTx.TxResult)
			} else if eventData, ok := event.Data.(coretypes.ResultEvent); ok {
				// Alternative event format
				if eventData.Events != nil {
					if txHashAttr, exists := eventData.Events["tx.hash"]; exists && len(txHashAttr) > 0 {
						txHash = txHashAttr[0]
					}
				}
				// Process transaction events if available
				fm.processEventData(txHash, eventData)
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
		case eventTypeBlockAttested, eventTypeBlockAttestedAlt:
			fm.handleBlockAttestedEvent(txHash, event)

		case eventTypeBatchAttested:
			fm.handleBatchAttestedEvent(txHash, event)
		}
	}
}

// processEventData processes event data from ResultEvent
func (fm *finalityMonitor) processEventData(txHash string, eventData coretypes.ResultEvent) {
	// This is for alternative event formats
	// Process based on event type
	fm.logger.Debug("Processing event data", "tx_hash", txHash)
}

// handleBlockAttestedEvent handles EventBlockAttested
func (fm *finalityMonitor) handleBlockAttestedEvent(txHash string, event abci.Event) {
	fm.logger.Debug("Processing block attested event", "type", event.Type, "tx_hash", txHash)

	// Parse event attributes
	var (
		attestationID uint64
		chainID       string
		blockHeight   uint64
		relayer       string
		finalityProof []byte
		processedAt   int64
	)

	for _, attr := range event.Attributes {
		key := string(attr.Key)
		value := string(attr.Value)

		switch key {
		case "attestation_id":
			if id, err := strconv.ParseUint(value, 10, 64); err == nil {
				attestationID = id
			}
		case "chain_id":
			chainID = value
		case "block_height":
			if height, err := strconv.ParseUint(value, 10, 64); err == nil {
				blockHeight = height
			}
		case "relayer":
			relayer = value
		case "finality_proof":
			finalityProof = []byte(value)
		case "processed_at":
			if ts, err := strconv.ParseInt(value, 10, 64); err == nil {
				processedAt = ts
			}
		}
	}

	if processedAt == 0 {
		processedAt = time.Now().Unix()
	}

	// Verify this matches a pending attestation
	fm.pendingMu.Lock()
	blockKey := fmt.Sprintf("%s:%d", chainID, blockHeight)
	pending, exists := fm.pendingByBlock[blockKey]

	if exists {
		// Verify tx hash matches (if we have it)
		if txHash != "" && pending.TxHash != txHash {
			fm.logger.Warn("Transaction hash mismatch for attestation",
				"expected_tx_hash", pending.TxHash,
				"received_tx_hash", txHash,
				"attestation_id", attestationID,
				"chain_id", chainID,
				"block_height", blockHeight,
			)
		}

		// Verify attestation ID matches (if we have it)
		if pending.AttestationID != 0 && pending.AttestationID != attestationID {
			fm.logger.Warn("Attestation ID mismatch",
				"expected_attestation_id", pending.AttestationID,
				"received_attestation_id", attestationID,
				"chain_id", chainID,
				"block_height", blockHeight,
			)
		}

		// Remove from pending
		delete(fm.pendingByBlock, blockKey)
		if pending.TxHash != "" {
			delete(fm.pendingAttestations, pending.TxHash)
		}

		fm.logger.Info("Block attestation confirmed",
			"attestation_id", attestationID,
			"chain_id", chainID,
			"block_height", blockHeight,
			"tx_hash", txHash,
			"latency", time.Since(pending.SubmittedAt),
			"relayer", relayer,
		)
	} else {
		fm.logger.Debug("Block attested (not tracked)",
			"attestation_id", attestationID,
			"chain_id", chainID,
			"block_height", blockHeight,
			"relayer", relayer,
		)
	}
	fm.pendingMu.Unlock()

	// The attestation chain has verified and stored the block data
	// Mark as finalized in our store
	finalityInfo := &FinalityInfo{
		AttestationID:     attestationID,
		ChainID:           chainID,
		BlockHeight:       blockHeight,
		Finalized:         true, // Attested = finalized
		FinalizedAt:       processedAt,
		ValidatorCount:    1, // At least one validator attested
		FinalityProof:     finalityProof,
		FinalitySignature: finalityProof,
		AttestationTxHash: []byte(txHash),
	}

	// Store in finality store
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := fm.finalityStore.SaveFinalityInfo(ctx, finalityInfo); err != nil {
		fm.logger.Error("Failed to store finality info", "error", err)
	} else {
		fm.logger.Debug("Stored finality info", "attestation_id", attestationID)

		// Notify subscribers
		fm.notifySubscribers(finalityInfo)
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
	if txHash != "" {
		if pending, exists := fm.pendingAttestations[txHash]; exists {
			fm.logger.Info("Batch attestation confirmed",
				"tx_hash", txHash,
				"chain_id", chainID,
				"start_height", startHeight,
				"end_height", endHeight,
				"block_count", blockCount,
				"latency", time.Since(pending.SubmittedAt),
			)
			delete(fm.pendingAttestations, txHash)
		}
	}

	// Remove individual blocks from pending
	for height := startHeight; height <= endHeight; height++ {
		blockKey := fmt.Sprintf("%s:%d", chainID, height)
		delete(fm.pendingByBlock, blockKey)
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
			ValidatorCount:    1,
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

	// Check each pending block
	for blockKey, pending := range fm.pendingByBlock {
		if now.Sub(pending.SubmittedAt) > pendingAttestationTimeout {
			delete(fm.pendingByBlock, blockKey)
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
	return len(fm.pendingByBlock)
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

	// Check if still pending
	fm.pendingMu.RLock()
	blockKey := fmt.Sprintf("%s:%d", chainID, height)
	pending, isPending := fm.pendingByBlock[blockKey]
	fm.pendingMu.RUnlock()

	if isPending {
		fm.logger.Debug("Block attestation still pending",
			"chain_id", chainID,
			"block_height", height,
			"tx_hash", pending.TxHash,
			"pending_duration", time.Since(pending.SubmittedAt),
		)
		// Return pending status
		return &FinalityInfo{
			AttestationID: pending.AttestationID,
			ChainID:       chainID,
			BlockHeight:   height,
			Finalized:     false,
		}, nil
	}

	// Query from finality store
	return fm.finalityStore.GetFinalityInfo(ctx, chainID, height)
}

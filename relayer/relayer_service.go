package relayer

import (
	"context"
	"fmt"
	"sync"
	"time"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/client"

	"cosmossdk.io/log"
)

// RelayerService is the main service coordinating all relayer components
type RelayerService struct {
	config *Config
	logger log.Logger

	// Monitors
	sourceMonitor      ChainMonitor
	attestationMonitor ChainMonitor

	// Components
	blockForwarder   BlockForwarder
	finalityMonitor  FinalityMonitor
	finalityStore    FinalityStore
	forcedTxMonitor  ForcedTxMonitor
	forcedTxExecutor ForcedTxExecutor

	// Control
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   bool
	runningMu sync.RWMutex

	// Status
	status   RelayerStatus
	statusMu sync.RWMutex
}

// NewRelayerService creates a new relayer service with all components
func NewRelayerService(
	config *Config,
	logger log.Logger,
	sourceClientCtx client.Context,
	attestationClientCtx client.Context,
) (*RelayerService, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is nil")
	}

	rs := &RelayerService{
		config: config,
		logger: logger,
		status: RelayerStatus{
			SourceChainID:      config.SourceChainID,
			AttestationChainID: config.AttestationChainID,
			UpdatedAt:          time.Now(),
		},
	}

	// Create finality store
	finalityStore, err := NewFinalityStoreFromConfig(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create finality store: %w", err)
	}
	rs.finalityStore = finalityStore

	// Create source chain monitor (Cronos)
	sourceMonitor, err := NewChainMonitor(
		config.SourceRPC,
		"cronos",
		config.SourceChainID,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create source monitor: %w", err)
	}
	rs.sourceMonitor = sourceMonitor

	// Create attestation chain RPC client
	attestationClient, err := rpchttp.New(config.AttestationRPC, "/websocket")
	if err != nil {
		return nil, fmt.Errorf("failed to create attestation RPC client: %w", err)
	}

	// Create attestation chain monitor
	attestationMonitor, err := NewChainMonitor(
		config.AttestationRPC,
		"attestation",
		config.AttestationChainID,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create attestation monitor: %w", err)
	}
	rs.attestationMonitor = attestationMonitor

	// Create block forwarder
	blockForwarder, err := NewBlockForwarder(
		attestationClientCtx,
		config,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create block forwarder: %w", err)
	}
	rs.blockForwarder = blockForwarder

	// Create finality monitor
	finalityMonitor, err := NewFinalityMonitor(
		attestationClient,
		config,
		logger,
		finalityStore,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create finality monitor: %w", err)
	}
	rs.finalityMonitor = finalityMonitor

	// Create forced TX monitor
	forcedTxMonitor, err := NewForcedTxMonitor(
		attestationClient,
		attestationClientCtx,
		config,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create forced tx monitor: %w", err)
	}
	rs.forcedTxMonitor = forcedTxMonitor

	// Create forced TX executor
	forcedTxExecutor, err := NewForcedTxExecutor(
		sourceClientCtx,
		attestationClientCtx,
		config,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create forced tx executor: %w", err)
	}
	rs.forcedTxExecutor = forcedTxExecutor

	return rs, nil
}

// Start starts all relayer components
func (rs *RelayerService) Start(ctx context.Context) error {
	rs.runningMu.Lock()
	defer rs.runningMu.Unlock()

	if rs.running {
		return fmt.Errorf("relayer service already running")
	}

	rs.logger.Info("Starting Cronos Attestation Layer Relayer",
		"source_chain", rs.config.SourceChainID,
		"attestation_chain", rs.config.AttestationChainID,
	)

	rs.ctx, rs.cancel = context.WithCancel(ctx)

	// Start monitors
	if err := rs.sourceMonitor.Start(rs.ctx); err != nil {
		return fmt.Errorf("failed to start source monitor: %w", err)
	}

	if err := rs.attestationMonitor.Start(rs.ctx); err != nil {
		return fmt.Errorf("failed to start attestation monitor: %w", err)
	}

	if err := rs.finalityMonitor.Start(rs.ctx); err != nil {
		return fmt.Errorf("failed to start finality monitor: %w", err)
	}

	if err := rs.forcedTxMonitor.Start(rs.ctx); err != nil {
		return fmt.Errorf("failed to start forced tx monitor: %w", err)
	}

	// Start worker goroutines
	rs.wg.Add(3)
	go rs.blockForwardingWorker()
	go rs.finalityRelayWorker()
	go rs.forcedTxWorker()

	rs.running = true
	rs.updateStatus(func(s *RelayerStatus) {
		s.Running = true
		s.UpdatedAt = time.Now()
	})

	rs.logger.Info("Relayer service started successfully")

	return nil
}

// Stop stops all relayer components gracefully
func (rs *RelayerService) Stop() error {
	rs.runningMu.Lock()
	defer rs.runningMu.Unlock()

	if !rs.running {
		return fmt.Errorf("relayer service not running")
	}

	rs.logger.Info("Stopping relayer service")

	// Cancel context to stop all workers
	rs.cancel()

	// Wait for workers to finish
	done := make(chan struct{})
	go func() {
		rs.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		rs.logger.Info("All workers stopped")
	case <-time.After(30 * time.Second):
		rs.logger.Warn("Timeout waiting for workers to stop")
	}

	// Stop monitors
	if err := rs.sourceMonitor.Stop(); err != nil {
		rs.logger.Error("Failed to stop source monitor", "error", err)
	}

	if err := rs.attestationMonitor.Stop(); err != nil {
		rs.logger.Error("Failed to stop attestation monitor", "error", err)
	}

	if err := rs.finalityMonitor.Stop(); err != nil {
		rs.logger.Error("Failed to stop finality monitor", "error", err)
	}

	if err := rs.forcedTxMonitor.Stop(); err != nil {
		rs.logger.Error("Failed to stop forced tx monitor", "error", err)
	}

	// Close finality store
	if err := rs.finalityStore.Close(); err != nil {
		rs.logger.Error("Failed to close finality store", "error", err)
	}

	rs.running = false
	rs.updateStatus(func(s *RelayerStatus) {
		s.Running = false
		s.UpdatedAt = time.Now()
	})

	rs.logger.Info("Relayer service stopped")

	return nil
}

// blockForwardingWorker monitors source chain and forwards blocks
func (rs *RelayerService) blockForwardingWorker() {
	defer rs.wg.Done()

	rs.logger.Info("Block forwarding worker started")

	// Subscribe to new blocks
	blockCh, err := rs.sourceMonitor.SubscribeNewBlocks(rs.ctx)
	if err != nil {
		rs.logger.Error("Failed to subscribe to new blocks", "error", err)
		rs.updateStatusError(err)
		return
	}

	// Batch processing
	batchTicker := time.NewTicker(5 * time.Second)
	defer batchTicker.Stop()

	var pendingBlocks []*BlockData

	for {
		select {
		case <-rs.ctx.Done():
			rs.logger.Info("Block forwarding worker stopped")
			return

		case block := <-blockCh:
			if block == nil {
				continue
			}

			rs.logger.Debug("New block received",
				"height", block.BlockHeight,
				"chain", block.ChainID,
			)

			// Add to batch
			pendingBlocks = append(pendingBlocks, block)

			// Forward immediately if batch is full
			if len(pendingBlocks) >= int(rs.config.BlockBatchSize) {
				rs.forwardBlockBatch(pendingBlocks)
				pendingBlocks = nil
			}

		case <-batchTicker.C:
			// Forward pending blocks on timer
			if len(pendingBlocks) > 0 {
				rs.forwardBlockBatch(pendingBlocks)
				pendingBlocks = nil
			}
		}
	}
}

// forwardBlockBatch forwards a batch of blocks
func (rs *RelayerService) forwardBlockBatch(blocks []*BlockData) {
	if len(blocks) == 0 {
		return
	}

	rs.logger.Info("Forwarding block batch",
		"count", len(blocks),
		"first_height", blocks[0].BlockHeight,
		"last_height", blocks[len(blocks)-1].BlockHeight,
	)

	var err error
	var attestationIDs []uint64

	if len(blocks) == 1 {
		// Single block
		var id uint64
		id, err = rs.blockForwarder.ForwardBlock(rs.ctx, blocks[0])
		if err == nil {
			attestationIDs = []uint64{id}
		}
	} else {
		// Batch
		attestationIDs, err = rs.blockForwarder.BatchForwardBlocks(rs.ctx, blocks)
	}

	if err != nil {
		rs.logger.Error("Failed to forward blocks",
			"count", len(blocks),
			"error", err,
		)
		rs.updateStatusError(err)
		return
	}

	rs.logger.Info("Blocks forwarded successfully",
		"count", len(blocks),
		"attestation_ids", attestationIDs,
	)

	// Update status
	lastBlock := blocks[len(blocks)-1]
	rs.updateStatus(func(s *RelayerStatus) {
		s.LastBlockForwarded = lastBlock.BlockHeight
		s.UpdatedAt = time.Now()
	})
}

// finalityRelayWorker monitors finality events
func (rs *RelayerService) finalityRelayWorker() {
	defer rs.wg.Done()

	rs.logger.Info("Finality relay worker started")

	// Subscribe to finality events
	finalityCh, err := rs.finalityMonitor.SubscribeFinality(rs.ctx)
	if err != nil {
		rs.logger.Error("Failed to subscribe to finality events", "error", err)
		rs.updateStatusError(err)
		return
	}

	for {
		select {
		case <-rs.ctx.Done():
			rs.logger.Info("Finality relay worker stopped")
			return

		case finality := <-finalityCh:
			if finality == nil {
				continue
			}

			rs.logger.Info("Block finalized on attestation layer",
				"chain", finality.ChainID,
				"height", finality.BlockHeight,
				"attestation_id", finality.AttestationID,
				"validator_count", finality.ValidatorCount,
			)

			// Update status
			rs.updateStatus(func(s *RelayerStatus) {
				s.LastFinalityReceived = finality.BlockHeight
				s.FinalizedBlocksCount++
				s.UpdatedAt = time.Now()
			})
		}
	}
}

// forcedTxWorker monitors and executes forced transactions
func (rs *RelayerService) forcedTxWorker() {
	defer rs.wg.Done()

	rs.logger.Info("Forced transaction worker started")

	// Subscribe to forced tx events
	forcedTxCh, err := rs.forcedTxMonitor.SubscribeForcedTx(rs.ctx)
	if err != nil {
		rs.logger.Error("Failed to subscribe to forced tx events", "error", err)
		rs.updateStatusError(err)
		return
	}

	// Periodic poll for pending forced txs
	pollTicker := time.NewTicker(rs.config.ForcedTxPollInterval)
	defer pollTicker.Stop()

	for {
		select {
		case <-rs.ctx.Done():
			rs.logger.Info("Forced transaction worker stopped")
			return

		case tx := <-forcedTxCh:
			if tx == nil {
				continue
			}

			rs.logger.Info("New forced transaction received",
				"forced_tx_id", tx.ForcedTxID,
				"target_chain", tx.TargetChainID,
				"priority", tx.Priority,
				"type", tx.TxType,
			)

			// Execute with retry
			if err := rs.executeForcedTxWithRetry(tx); err != nil {
				rs.logger.Error("Failed to execute forced tx",
					"forced_tx_id", tx.ForcedTxID,
					"error", err,
				)
				rs.updateStatusError(err)
				continue
			}

			rs.logger.Info("Forced transaction executed successfully",
				"forced_tx_id", tx.ForcedTxID,
			)

			// Update status
			rs.updateStatus(func(s *RelayerStatus) {
				s.LastForcedTxProcessed = tx.ForcedTxID
				s.UpdatedAt = time.Now()
			})

		case <-pollTicker.C:
			// Periodically check for pending forced txs
			rs.processPendingForcedTxs()
		}
	}
}

// executeForcedTxWithRetry executes a forced transaction with retry logic
func (rs *RelayerService) executeForcedTxWithRetry(tx *ForcedTx) error {
	var lastErr error

	for attempt := uint(0); attempt < rs.config.MaxRetries; attempt++ {
		err := rs.forcedTxExecutor.ExecuteForcedTx(rs.ctx, tx)
		if err == nil {
			return nil
		}

		lastErr = err
		rs.logger.Warn("Forced tx execution attempt failed",
			"forced_tx_id", tx.ForcedTxID,
			"attempt", attempt+1,
			"error", err,
		)

		// Wait before retry
		select {
		case <-rs.ctx.Done():
			return rs.ctx.Err()
		case <-time.After(rs.config.RetryDelay):
			continue
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// processPendingForcedTxs checks and processes pending forced transactions
func (rs *RelayerService) processPendingForcedTxs() {
	pending, err := rs.forcedTxMonitor.GetPendingForcedTxs(rs.ctx, rs.config.SourceChainID)
	if err != nil {
		rs.logger.Error("Failed to get pending forced txs", "error", err)
		return
	}

	if len(pending) == 0 {
		return
	}

	rs.logger.Info("Processing pending forced transactions", "count", len(pending))

	// Update status
	rs.updateStatus(func(s *RelayerStatus) {
		s.PendingForcedTxCount = len(pending)
		s.UpdatedAt = time.Now()
	})

	// Execute batch
	if err := rs.forcedTxExecutor.BatchExecuteForcedTx(rs.ctx, pending); err != nil {
		rs.logger.Error("Failed to batch execute forced txs", "error", err)
		rs.updateStatusError(err)
	}
}

// GetStatus returns the current relayer status
func (rs *RelayerService) GetStatus() RelayerStatus {
	rs.statusMu.RLock()
	defer rs.statusMu.RUnlock()

	// Return a copy
	return rs.status
}

// GetFinalityStoreStats returns finality store statistics
func (rs *RelayerService) GetFinalityStoreStats() (*FinalityStoreStats, error) {
	if rs.finalityStore == nil {
		return nil, fmt.Errorf("finality store not initialized")
	}
	return rs.finalityStore.GetStats(context.Background(), rs.config.SourceChainID)
}

// IsRunning returns whether the relayer is currently running
func (rs *RelayerService) IsRunning() bool {
	rs.runningMu.RLock()
	defer rs.runningMu.RUnlock()
	return rs.running
}

// Helper methods

func (rs *RelayerService) updateStatus(fn func(*RelayerStatus)) {
	rs.statusMu.Lock()
	defer rs.statusMu.Unlock()
	fn(&rs.status)
}

func (rs *RelayerService) updateStatusError(err error) {
	rs.updateStatus(func(s *RelayerStatus) {
		s.LastError = err.Error()
		s.UpdatedAt = time.Now()
	})
}

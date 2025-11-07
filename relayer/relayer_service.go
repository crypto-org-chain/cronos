package relayer

import (
	"context"
	"fmt"
	"sync"
	"time"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cometbft/cometbft/types"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/client"
)

// RelayerService is the main service coordinating all relayer components
type RelayerService struct {
	config *Config
	logger log.Logger

	// Monitors
	sourceMonitor      ChainMonitor
	attestationMonitor ChainMonitor

	// Components
	blockForwarder  BlockForwarder
	finalityMonitor FinalityMonitor
	finalityStore   FinalityStore

	// RPC server (optional)
	rpcServer *RPCServer

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

	// Create finality monitor (before block forwarder so it can be passed in)
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

	// Create block forwarder (with finality monitor for tracking)
	blockForwarder, err := NewBlockForwarder(
		attestationClientCtx,
		config,
		logger,
		finalityMonitor,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create block forwarder: %w", err)
	}
	rs.blockForwarder = blockForwarder

	// Create RPC server if enabled
	if config.RPCEnabled && config.RPCConfig != nil {
		rpcServer, err := NewRPCServer(rs, config.RPCConfig, logger)
		if err != nil {
			logger.Warn("Failed to create RPC server", "error", err)
		} else {
			rs.rpcServer = rpcServer
			logger.Info("RPC server created", "addr", config.RPCConfig.ListenAddr)
		}
	}

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

	// Subscribe to new blocks from source chain
	rs.logger.Info("Subscribing to new blocks from source chain")
	blockCh, err := rs.sourceMonitor.SubscribeNewBlocks(rs.ctx)
	if err != nil {
		return fmt.Errorf("failed to subscribe to new blocks: %w", err)
	}

	// Start block processing worker
	rs.wg.Add(1)
	go rs.processNewBlocks(blockCh)

	if err := rs.attestationMonitor.Start(rs.ctx); err != nil {
		return fmt.Errorf("failed to start attestation monitor: %w", err)
	}

	if err := rs.finalityMonitor.Start(rs.ctx); err != nil {
		return fmt.Errorf("failed to start finality monitor: %w", err)
	}

	// Start worker goroutines
	rs.wg.Add(2)
	go rs.blockForwardingWorker()
	go rs.finalityRelayWorker()

	rs.running = true
	rs.updateStatus(func(s *RelayerStatus) {
		s.Running = true
		s.UpdatedAt = time.Now()
	})

	// Start RPC server if configured
	if rs.rpcServer != nil {
		if err := rs.rpcServer.Start(); err != nil {
			rs.logger.Error("Failed to start RPC server", "error", err)
		} else {
			rs.logger.Info("RPC server started", "addr", rs.config.RPCConfig.ListenAddr)
		}
	}

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

	// Stop RPC server if running
	if rs.rpcServer != nil {
		if err := rs.rpcServer.Stop(); err != nil {
			rs.logger.Error("Failed to stop RPC server", "error", err)
		} else {
			rs.logger.Info("RPC server stopped")
		}
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

	var pendingBlocks []*types.EventDataNewBlock
	var lastForwardedHeight uint64

	// Initialize from checkpoint if available
	if rs.finalityMonitor != nil {
		if checkpointMgr := rs.getCheckpointManager(); checkpointMgr != nil {
			lastFinalityHeight := checkpointMgr.GetLastFinalityBlockHeight()
			if lastFinalityHeight > 0 {
				lastForwardedHeight = lastFinalityHeight
				rs.logger.Info("Initialized last forwarded height from checkpoint",
					"height", lastForwardedHeight,
				)
			}
		}
	}

	for {
		select {
		case <-rs.ctx.Done():
			// Forward any remaining blocks before shutdown
			if len(pendingBlocks) > 0 {
				rs.logger.Info("Forwarding remaining blocks before shutdown", "count", len(pendingBlocks))
				rs.forwardBlockBatch(pendingBlocks)
			}
			rs.logger.Info("Block forwarding worker stopped")
			return

		case block := <-blockCh:
			if block == nil {
				continue
			}

			blockHeight := uint64(block.Block.Height)

			rs.logger.Debug("New block received",
				"height", blockHeight,
				"chain_id", block.Block.ChainID,
			)

			// Detect and fill gaps
			if lastForwardedHeight > 0 && blockHeight > lastForwardedHeight+1 {
				gap := blockHeight - lastForwardedHeight - 1
				rs.logger.Warn("Gap detected in block stream, filling missing blocks",
					"last_forwarded", lastForwardedHeight,
					"received_height", blockHeight,
					"gap_size", gap,
				)

				// Query and fill missing blocks
				missingBlocks := rs.fillBlockGap(lastForwardedHeight+1, blockHeight-1)
				if len(missingBlocks) > 0 {
					pendingBlocks = append(pendingBlocks, missingBlocks...)
					rs.logger.Info("Added missing blocks to batch",
						"count", len(missingBlocks),
						"start", lastForwardedHeight+1,
						"end", blockHeight-1,
					)
				}
			}

			// Add current block to batch
			pendingBlocks = append(pendingBlocks, block)

			// Forward immediately if batch is full
			if len(pendingBlocks) >= int(rs.config.BlockBatchSize) {
				rs.forwardBlockBatch(pendingBlocks)
				lastForwardedHeight = uint64(pendingBlocks[len(pendingBlocks)-1].Block.Height)
				pendingBlocks = nil
			}

		case <-batchTicker.C:
			// Forward pending blocks on timer
			if len(pendingBlocks) > 0 {
				rs.forwardBlockBatch(pendingBlocks)
				lastForwardedHeight = uint64(pendingBlocks[len(pendingBlocks)-1].Block.Height)
				pendingBlocks = nil
			}
		}
	}
}

// fillBlockGap queries and returns missing blocks between startHeight and endHeight (inclusive)
func (rs *RelayerService) fillBlockGap(startHeight, endHeight uint64) []*types.EventDataNewBlock {
	if startHeight > endHeight {
		return nil
	}

	rs.logger.Info("Querying missing blocks to fill gap",
		"start_height", startHeight,
		"end_height", endHeight,
		"count", endHeight-startHeight+1,
	)

	var missingBlocks []*types.EventDataNewBlock

	for height := startHeight; height <= endHeight; height++ {
		block, err := rs.sourceMonitor.GetBlock(rs.ctx, height)
		if err != nil {
			rs.logger.Error("Failed to query missing block",
				"height", height,
				"error", err,
			)
			// Continue trying to get other blocks
			continue
		}

		missingBlocks = append(missingBlocks, block)
	}

	rs.logger.Info("Successfully queried missing blocks",
		"requested", endHeight-startHeight+1,
		"retrieved", len(missingBlocks),
	)

	return missingBlocks
}

// forwardBlockBatch forwards a batch of blocks
func (rs *RelayerService) forwardBlockBatch(blocks []*types.EventDataNewBlock) {
	if len(blocks) == 0 {
		return
	}

	// Verify blocks are in ascending order
	for i := 1; i < len(blocks); i++ {
		prevHeight := uint64(blocks[i-1].Block.Height)
		currHeight := uint64(blocks[i].Block.Height)
		if currHeight != prevHeight+1 {
			rs.logger.Error("Blocks in batch are not in continuous ascending order",
				"prev_height", prevHeight,
				"curr_height", currHeight,
				"expected", prevHeight+1,
			)
			// Don't forward invalid batch
			return
		}
	}

	firstHeight := uint64(blocks[0].Block.Height)
	lastHeight := uint64(blocks[len(blocks)-1].Block.Height)

	rs.logger.Info("Forwarding block batch",
		"count", len(blocks),
		"chain_id", blocks[0].Block.ChainID,
		"first_height", firstHeight,
		"last_height", lastHeight,
	)

	// Always use BatchForwardBlocks (works for single or multiple blocks)
	attestationIDs, err := rs.blockForwarder.BatchForwardBlocks(rs.ctx, blocks)
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
		s.LastBlockForwarded = uint64(lastBlock.Block.Height)
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

// getCheckpointManager retrieves the checkpoint manager from finality monitor (helper for internal use)
func (rs *RelayerService) getCheckpointManager() *CheckpointManager {
	// Type assert to access internal field
	if fm, ok := rs.finalityMonitor.(*finalityMonitor); ok {
		return fm.checkpointManager
	}
	return nil
}

// GetFinalityInfo retrieves finality information for a specific block
func (rs *RelayerService) GetFinalityInfo(ctx context.Context, chainID string, height uint64) (*FinalityInfo, error) {
	if rs.finalityMonitor == nil {
		return nil, fmt.Errorf("finality monitor not available")
	}

	return rs.finalityMonitor.GetFinalityStatus(ctx, chainID, height)
}

// GetCheckpointState returns the current checkpoint state
func (rs *RelayerService) GetCheckpointState() (uint64, map[string]*PendingAttestation) {
	if checkpointMgr := rs.getCheckpointManager(); checkpointMgr != nil {
		lastHeight := checkpointMgr.GetLastFinalityBlockHeight()
		pendingMap := checkpointMgr.GetPendingAttestations()
		return lastHeight, pendingMap
	}
	return 0, make(map[string]*PendingAttestation)
}

// GetPendingAttestationsCount returns the count of pending attestations
func (rs *RelayerService) GetPendingAttestationsCount() int {
	if rs.finalityMonitor == nil {
		return 0
	}

	return rs.finalityMonitor.GetPendingAttestations()
}

// processNewBlocks processes blocks from the subscription channel
func (rs *RelayerService) processNewBlocks(blockCh <-chan *types.EventDataNewBlock) {
	defer rs.wg.Done()

	rs.logger.Info("Started block processing worker")

	// Batch configuration
	batchSize := int(rs.config.BlockBatchSize)
	if batchSize == 0 {
		batchSize = 10 // Default batch size
	}

	batchTimeout := rs.config.BlockPollInterval
	if batchTimeout == 0 {
		batchTimeout = 2 * time.Second // Default batch timeout
	}

	// Buffered blocks for batching
	var batch []*types.EventDataNewBlock
	batchTimer := time.NewTimer(batchTimeout)
	defer batchTimer.Stop()

	for {
		select {
		case <-rs.ctx.Done():
			rs.logger.Info("Context canceled, stopping block processing")
			// Forward any remaining blocks in batch
			if len(batch) > 0 {
				rs.logger.Info("Forwarding remaining blocks in batch", "count", len(batch))
				rs.forwardBlockBatch(batch)
			}
			return

		case blockData, ok := <-blockCh:
			if !ok {
				rs.logger.Warn("Block channel closed")
				// Forward any remaining blocks
				if len(batch) > 0 {
					rs.logger.Info("Forwarding remaining blocks", "count", len(batch))
					rs.forwardBlockBatch(batch)
				}
				return
			}

			// Add block to batch
			batch = append(batch, blockData)
			rs.logger.Debug("Added block to batch",
				"height", blockData.Block.Height,
				"batch_size", len(batch),
			)

			// Forward if batch is full
			if len(batch) >= batchSize {
				rs.logger.Info("Batch full, forwarding blocks", "count", len(batch))
				rs.forwardBlockBatch(batch)
				batch = nil // Reset batch
				batchTimer.Reset(batchTimeout)
			}

		case <-batchTimer.C:
			// Forward batch on timeout
			if len(batch) > 0 {
				rs.logger.Info("Batch timeout, forwarding blocks", "count", len(batch))
				rs.forwardBlockBatch(batch)
				batch = nil // Reset batch
			}
			batchTimer.Reset(batchTimeout)
		}
	}
}

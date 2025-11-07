package relayer

import (
	"context"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cometbft/cometbft/types"

	"cosmossdk.io/log"
)

// chainMonitor implements ChainMonitor interface
type chainMonitor struct {
	client    rpcclient.Client
	logger    log.Logger
	chainID   string
	chainName string
	running   bool
}

// NewChainMonitor creates a new chain monitor
func NewChainMonitor(rpcURL, chainName, chainID string, logger log.Logger) (ChainMonitor, error) {
	client, err := rpchttp.New(rpcURL, "/websocket")
	if err != nil {
		return nil, fmt.Errorf("failed to create RPC client: %w", err)
	}

	return &chainMonitor{
		client:    client,
		logger:    logger.With("component", "chain_monitor", "chain", chainName),
		chainID:   chainID,
		chainName: chainName,
	}, nil
}

// Start begins monitoring the chain
func (cm *chainMonitor) Start(ctx context.Context) error {
	if cm.running {
		return fmt.Errorf("chain monitor already running")
	}

	cm.logger.Info("Starting chain monitor", "chain", cm.chainName, "chain_id", cm.chainID)

	if err := cm.client.Start(); err != nil {
		return fmt.Errorf("failed to start RPC client: %w", err)
	}

	// Verify connection
	status, err := cm.client.Status(ctx)
	if err != nil {
		return fmt.Errorf("failed to get chain status: %w", err)
	}

	cm.logger.Info("Connected to chain",
		"chain", cm.chainName,
		"chain_id", status.NodeInfo.Network,
		"latest_height", status.SyncInfo.LatestBlockHeight,
	)

	cm.running = true
	return nil
}

// Stop stops the monitor
func (cm *chainMonitor) Stop() error {
	if !cm.running {
		return nil
	}

	cm.logger.Info("Stopping chain monitor", "chain", cm.chainName)

	if err := cm.client.Stop(); err != nil {
		cm.logger.Error("Failed to stop RPC client", "error", err)
	}

	cm.running = false
	return nil
}

// GetLatestHeight returns the latest block height
func (cm *chainMonitor) GetLatestHeight(ctx context.Context) (uint64, error) {
	status, err := cm.client.Status(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get status: %w", err)
	}

	return uint64(status.SyncInfo.LatestBlockHeight), nil
}

// GetBlock retrieves block data for a specific height
func (cm *chainMonitor) GetBlock(ctx context.Context, height uint64) (*types.EventDataNewBlock, error) {
	cm.logger.Debug("Fetching block", "height", height, "chain", cm.chainName)

	// Get block
	h := int64(height)
	block, err := cm.client.Block(ctx, &h)
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %w", err)
	}

	// Get block results
	blockResults, err := cm.client.BlockResults(ctx, &h)
	if err != nil {
		return nil, fmt.Errorf("failed to get block results: %w", err)
	}

	// Create EventDataNewBlock directly
	// Convert ResultBlockResults to ResponseFinalizeBlock
	resultFinalizeBlock := abci.ResponseFinalizeBlock{
		Events:                blockResults.FinalizeBlockEvents,
		TxResults:             blockResults.TxsResults,
		ValidatorUpdates:      blockResults.ValidatorUpdates,
		ConsensusParamUpdates: blockResults.ConsensusParamUpdates,
		AppHash:               blockResults.AppHash,
	}

	eventData := &types.EventDataNewBlock{
		Block:               block.Block,
		BlockID:             block.BlockID,
		ResultFinalizeBlock: resultFinalizeBlock,
	}

	cm.logger.Debug("Fetched block",
		"height", height,
		"chain", cm.chainName,
		"chain_id", block.Block.ChainID,
		"timestamp", block.Block.Time,
		"block_hash", fmt.Sprintf("%X", block.BlockID.Hash),
		"app_hash", fmt.Sprintf("%X", block.Block.AppHash),
		"tx_count", len(blockResults.TxsResults),
	)

	return eventData, nil
}

// SubscribeNewBlocks subscribes to new block events via WebSocket
func (cm *chainMonitor) SubscribeNewBlocks(ctx context.Context) (<-chan *types.EventDataNewBlock, error) {
	cm.logger.Info("Subscribing to new block events", "chain", cm.chainName)

	// Create channel for block data
	blockCh := make(chan *types.EventDataNewBlock, 10) // Buffer of 10 blocks

	// Subscribe to NewBlock events
	query := "tm.event='NewBlock'"

	// Start the client if not already started (WebSocket connection)
	if !cm.client.IsRunning() {
		if err := cm.client.Start(); err != nil {
			close(blockCh)
			return nil, fmt.Errorf("failed to start RPC client: %w", err)
		}
	}

	// Subscribe to events
	subscription, err := cm.client.Subscribe(ctx, cm.chainName, query)
	if err != nil {
		close(blockCh)
		return nil, fmt.Errorf("failed to subscribe to new block events: %w", err)
	}

	cm.logger.Info("Successfully subscribed to new block events",
		"chain", cm.chainName,
		"query", query,
	)

	// Start goroutine to process events
	go func() {
		defer close(blockCh)
		defer func() {
			// Unsubscribe when done
			if err := cm.client.Unsubscribe(ctx, cm.chainName, query); err != nil {
				cm.logger.Error("Failed to unsubscribe", "error", err, "chain", cm.chainName)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				cm.logger.Info("Context canceled, stopping block subscription", "chain", cm.chainName)
				return

			case result, ok := <-subscription:
				if !ok {
					cm.logger.Warn("Subscription channel closed", "chain", cm.chainName)
					return
				}

				// Extract block data from event
				if result.Data == nil {
					cm.logger.Warn("Received nil data in event", "chain", cm.chainName)
					continue
				}

				// Parse NewBlock event
				newBlockEvent, ok := result.Data.(types.EventDataNewBlock)
				if !ok {
					cm.logger.Warn("Unexpected event data type",
						"chain", cm.chainName,
						"type", fmt.Sprintf("%T", result.Data),
					)
					continue
				}

				block := newBlockEvent.Block
				if block == nil {
					cm.logger.Warn("Received nil block in NewBlock event", "chain", cm.chainName)
					continue
				}

				height := uint64(block.Height)

				cm.logger.Debug("Received new block event",
					"chain", cm.chainName,
					"chain_id", block.ChainID,
					"height", height,
					"hash", fmt.Sprintf("%X", block.Hash()),
					"time", block.Time,
				)

				// Send block data to channel (non-blocking)
				select {
				case blockCh <- &newBlockEvent:
					cm.logger.Debug("Sent block data to channel",
						"chain", cm.chainName,
						"height", height,
					)
				case <-ctx.Done():
					return
				default:
					cm.logger.Warn("Block channel full, dropping block",
						"chain", cm.chainName,
						"height", height,
					)
				}
			}
		}
	}()

	return blockCh, nil
}

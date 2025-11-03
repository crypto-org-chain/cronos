package relayer

import (
	"context"
	"fmt"

	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"

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
func (cm *chainMonitor) GetBlock(ctx context.Context, height uint64) (*BlockData, error) {
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

	// Construct BlockData from Block and BlockResults
	blockData := &BlockData{
		ChainID:     cm.chainID,
		BlockHeight: height,
		Timestamp:   block.Block.Time.Unix(),
		// Block data
		BlockHash:   block.BlockID.Hash,
		AppHash:     block.Block.AppHash,
		BlockHeader: block.Block.Header,
		// Block results data
		TxResults:             blockResults.TxsResults,
		FinalizeBlockEvents:   blockResults.FinalizeBlockEvents,
		ValidatorUpdates:      blockResults.ValidatorUpdates,
		ConsensusParamUpdates: blockResults.ConsensusParamUpdates,
	}

	cm.logger.Debug("Fetched block",
		"height", height,
		"chain", cm.chainName,
		"timestamp", blockData.Timestamp,
		"block_hash", fmt.Sprintf("%X", blockData.BlockHash),
		"app_hash", fmt.Sprintf("%X", blockData.AppHash),
		"tx_count", len(blockData.TxResults),
	)

	return blockData, nil
}

// SubscribeNewBlocks subscribes to new block events
func (cm *chainMonitor) SubscribeNewBlocks(ctx context.Context) (<-chan *BlockData, error) {
	if !cm.running {
		return nil, fmt.Errorf("chain monitor not running")
	}

	blockCh := make(chan *BlockData, 100)

	// TODO: Implement when needed for actual block monitoring
	// Will subscribe to "tm.event='NewBlock'" and forward block data

	cm.logger.Warn("SubscribeNewBlocks not yet fully implemented")

	return blockCh, nil
}

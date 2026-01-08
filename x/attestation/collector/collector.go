package collector

import (
	"context"
	"fmt"
	"sync"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	rpcclient "github.com/cometbft/cometbft/rpc/client"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"

	"github.com/crypto-org-chain/cronos/x/attestation/types"
)

// BlockDataCollector collects full block data from CometBFT for attestation
// It subscribes to new block events and stores complete block data locally
type BlockDataCollector struct {
	cdc    codec.BinaryCodec
	db     dbm.DB
	client rpcclient.Client
	logger log.Logger

	mu      sync.RWMutex
	running bool
	cancel  context.CancelFunc
}

// Ensure BlockDataCollector implements keeper.BlockDataCollector interface
var _ interface {
	GetBlockData(height uint64) (*types.BlockAttestationData, error)
	GetBlockDataRange(startHeight, endHeight uint64) ([]*types.BlockAttestationData, error)
} = (*BlockDataCollector)(nil)

// NewBlockDataCollector creates a new block data collector
func NewBlockDataCollector(
	cdc codec.BinaryCodec,
	db dbm.DB,
	client rpcclient.Client,
	logger log.Logger,
) *BlockDataCollector {
	return &BlockDataCollector{
		cdc:     cdc,
		db:      db,
		client:  client,
		logger:  logger,
		running: false,
	}
}

// SetClient sets the CometBFT RPC client (useful for late initialization)
func (c *BlockDataCollector) SetClient(client rpcclient.Client) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.client = client
}

// Start begins collecting block data from CometBFT
func (c *BlockDataCollector) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("collector already running")
	}

	if c.client == nil {
		c.mu.Unlock()
		return fmt.Errorf("CometBFT client not set")
	}

	// Ensure the RPC client is started
	if !c.client.IsRunning() {
		if err := c.client.Start(); err != nil {
			c.mu.Unlock()
			return fmt.Errorf("failed to start CometBFT RPC client: %w", err)
		}
	}

	// Create cancellable context
	collectorCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.running = true
	c.mu.Unlock()

	// Subscribe to new block events
	const subscriber = "block-data-collector"
	const query = "tm.event='NewBlock'"

	eventCh, err := c.client.Subscribe(collectorCtx, subscriber, query, 100)
	if err != nil {
		c.mu.Lock()
		c.running = false
		c.cancel = nil
		c.mu.Unlock()
		return fmt.Errorf("failed to subscribe to blocks: %w", err)
	}

	// Start collection goroutine
	go c.collectLoop(collectorCtx, eventCh)

	return nil
}

// Stop stops the block data collector
func (c *BlockDataCollector) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return fmt.Errorf("collector not running")
	}

	if c.cancel != nil {
		c.cancel()
	}
	c.running = false

	return nil
}

// collectLoop is the main collection loop
func (c *BlockDataCollector) collectLoop(ctx context.Context, eventCh <-chan ctypes.ResultEvent) {
	c.logger.Debug("Block data collector loop started")

	for {
		select {
		case <-ctx.Done():
			c.logger.Debug("Block data collector loop stopped (context cancelled)")
			return

		case event := <-eventCh:
			// Extract block data from event
			eventData, ok := event.Data.(cmttypes.EventDataNewBlock)
			if !ok {
				c.logger.Debug("Received non-block event, skipping",
					"event_type", fmt.Sprintf("%T", event.Data))
				continue
			}

			c.logger.Debug("Collecting block data",
				"height", eventData.Block.Height,
				"hash", fmt.Sprintf("%X", eventData.Block.Hash()))

			if err := c.collectAndStoreBlock(eventData); err != nil {
				// Log error but continue collecting
				c.logger.Error("failed to collect block",
					"height", eventData.Block.Height,
					"error", err,
				)
			} else {
				c.logger.Debug("Successfully collected block data",
					"height", eventData.Block.Height)
			}
		}
	}
}

// collectAndStoreBlock collects full block data and stores it locally
func (c *BlockDataCollector) collectAndStoreBlock(eventData cmttypes.EventDataNewBlock) error {
	block := eventData.Block
	height := block.Height

	attestationData := &types.BlockAttestationData{
		BlockHeight: uint64(height),
		AppHash:     block.Header.AppHash,
	}

	// Log field lengths for debugging
	c.logger.Info("collected block attestation data", "height", height)

	// Store in local database
	return c.storeBlockData(uint64(height), attestationData)
}

// storeBlockData stores block attestation data in the local database
func (c *BlockDataCollector) storeBlockData(height uint64, data *types.BlockAttestationData) error {
	key := getBlockDataKey(height)
	bz := c.cdc.MustMarshal(data)

	return c.db.Set(key, bz)
}

// GetBlockData retrieves stored block data by height
func (c *BlockDataCollector) GetBlockData(height uint64) (*types.BlockAttestationData, error) {
	key := getBlockDataKey(height)
	bz, err := c.db.Get(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get block data for height %d: %w", height, err)
	}

	if bz == nil {
		return nil, fmt.Errorf("block data not found for height %d", height)
	}

	var data types.BlockAttestationData
	if err := c.cdc.Unmarshal(bz, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block data: %w", err)
	}

	return &data, nil
}

// GetBlockDataRange retrieves a range of block data
// If the collector is not running, it will try to collect the blocks on-demand
func (c *BlockDataCollector) GetBlockDataRange(startHeight, endHeight uint64) ([]*types.BlockAttestationData, error) {
	// Auto-start collector if it has a client but isn't running
	c.ensureStarted()

	var result []*types.BlockAttestationData

	for h := startHeight; h <= endHeight; h++ {
		data, err := c.GetBlockData(h)
		if err != nil {
			// If collector is running and data is missing, try to fetch on-demand
			if c.client != nil {
				// Try to fetch this specific block
				if fetchedData, fetchErr := c.fetchBlockOnDemand(h); fetchErr == nil {
					result = append(result, fetchedData)
					continue
				}
			}
			// Skip missing block silently - it's before collector started or not available
			continue
		}
		result = append(result, data)
	}

	if len(result) == 0 {
		c.logger.Debug("no block data available in requested range",
			"start_height", startHeight,
			"end_height", endHeight,
			"collector_running", c.IsRunning(),
		)
		return nil, fmt.Errorf("no block data found in range %d-%d", startHeight, endHeight)
	}

	return result, nil
}

// ensureStarted auto-starts the collector if it has a client but isn't running
func (c *BlockDataCollector) ensureStarted() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If already running or no client, nothing to do
	if c.running || c.client == nil {
		return
	}

	// Ensure the RPC client is started
	if !c.client.IsRunning() {
		if err := c.client.Start(); err != nil {
			c.logger.Error("failed to start RPC client for collector", "error", err)
			return
		}
	}

	// Create cancellable context
	collectorCtx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.running = true

	// Subscribe to new block events
	const subscriber = "block-data-collector"
	const query = "tm.event='NewBlock'"

	eventCh, err := c.client.Subscribe(collectorCtx, subscriber, query, 100)
	if err != nil {
		c.logger.Error("failed to auto-start collector subscription", "error", err)
		c.running = false
		c.cancel = nil
		return
	}

	// Start collection goroutine
	go c.collectLoop(collectorCtx, eventCh)

	c.logger.Info("Block data collector auto-started")
}

// fetchBlockOnDemand fetches a specific block's data on-demand via RPC
func (c *BlockDataCollector) fetchBlockOnDemand(height uint64) (*types.BlockAttestationData, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("RPC client not available")
	}

	// Query the block
	h := int64(height)
	blockRes, err := client.Block(context.Background(), &h)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block: %w", err)
	}

	// Create attestation data
	block := blockRes.Block
	attestationData := &types.BlockAttestationData{
		BlockHeight: uint64(height),
		AppHash:     block.Header.AppHash,
	}

	// Store it for future use
	if err := c.storeBlockData(height, attestationData); err != nil {
		c.logger.Warn("failed to store on-demand block data", "height", height, "error", err)
		// Return data even if storage fails
	}

	return attestationData, nil
}

// getBlockDataKey returns the database key for block data at a given height
func getBlockDataKey(height uint64) []byte {
	prefix := []byte("block_data:")
	heightBytes := sdk.Uint64ToBigEndian(height)
	return append(prefix, heightBytes...)
}

// IsRunning returns whether the collector is currently running
func (c *BlockDataCollector) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

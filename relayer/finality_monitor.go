package relayer

import (
	"context"
	"fmt"

	rpcclient "github.com/cometbft/cometbft/rpc/client"

	"cosmossdk.io/log"
)

// finalityMonitor implements FinalityMonitor interface
type finalityMonitor struct {
	client        rpcclient.Client
	config        *Config
	logger        log.Logger
	finalityStore FinalityStore
	running       bool
}

// NewFinalityMonitor creates a new finality monitor
func NewFinalityMonitor(
	client rpcclient.Client,
	config *Config,
	logger log.Logger,
	finalityStore FinalityStore,
) (FinalityMonitor, error) {
	return &finalityMonitor{
		client:        client,
		config:        config,
		logger:        logger.With("component", "finality_monitor"),
		finalityStore: finalityStore,
	}, nil
}

// Start begins monitoring finality events
func (fm *finalityMonitor) Start(ctx context.Context) error {
	if fm.running {
		return fmt.Errorf("finality monitor already running")
	}

	fm.logger.Info("Starting finality monitor")

	// TODO: Implement when attestation chain is deployed
	// Will subscribe to EventBlockFinalized events from attestation chain
	// and update finality store accordingly

	fm.running = true
	return nil
}

// Stop stops the finality monitor
func (fm *finalityMonitor) Stop() error {
	if !fm.running {
		return nil
	}

	fm.logger.Info("Stopping finality monitor")
	fm.running = false
	return nil
}

// SubscribeFinality returns a channel that receives finality updates
func (fm *finalityMonitor) SubscribeFinality(ctx context.Context) (<-chan *FinalityInfo, error) {
	finalityCh := make(chan *FinalityInfo, 100)

	// TODO: Implement when attestation chain is deployed
	// Will subscribe to finality events and forward to channel

	return finalityCh, nil
}

// GetFinalityStatus retrieves finality status for a block
func (fm *finalityMonitor) GetFinalityStatus(ctx context.Context, chainID string, height uint64) (*FinalityInfo, error) {
	fm.logger.Debug("GetFinalityStatus called",
		"chain_id", chainID,
		"block_height", height,
	)

	// Query from finality store
	return fm.finalityStore.GetFinalityInfo(ctx, chainID, height)
}

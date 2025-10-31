package relayer

import (
	"context"

	"github.com/cosmos/cosmos-sdk/client"

	"cosmossdk.io/log"
)

// blockForwarder implements BlockForwarder interface
type blockForwarder struct {
	clientCtx client.Context
	config    *Config
	logger    log.Logger
}

// NewBlockForwarder creates a new block forwarder
func NewBlockForwarder(
	clientCtx client.Context,
	config *Config,
	logger log.Logger,
) (BlockForwarder, error) {
	return &blockForwarder{
		clientCtx: clientCtx,
		config:    config,
		logger:    logger.With("component", "block_forwarder"),
	}, nil
}

// ForwardBlock sends ABCI block data to attestation layer
func (bf *blockForwarder) ForwardBlock(ctx context.Context, blockData *BlockData) (uint64, error) {
	bf.logger.Debug("ForwardBlock called",
		"chain_id", blockData.ChainID,
		"block_height", blockData.BlockHeight,
	)

	// TODO: Implement when attestation chain is deployed
	// Will create and broadcast MsgSubmitBlockAttestation transaction

	// Return block height as placeholder attestation ID
	return blockData.BlockHeight, nil
}

// BatchForwardBlocks forwards multiple blocks to attestation layer
func (bf *blockForwarder) BatchForwardBlocks(ctx context.Context, blocks []*BlockData) ([]uint64, error) {
	if len(blocks) == 0 {
		return nil, nil
	}

	bf.logger.Debug("BatchForwardBlocks called", "count", len(blocks))

	// TODO: Implement batch forwarding when attestation chain is ready

	// Return block heights as placeholder attestation IDs
	ids := make([]uint64, len(blocks))
	for i, block := range blocks {
		ids[i] = block.BlockHeight
	}
	return ids, nil
}

// GetAttestationStatus checks if a block has been attested
func (bf *blockForwarder) GetAttestationStatus(ctx context.Context, chainID string, height uint64) (*AttestationStatus, error) {
	bf.logger.Debug("GetAttestationStatus called",
		"chain_id", chainID,
		"block_height", height,
	)

	// TODO: Query attestation chain using QueryClient

	return &AttestationStatus{
		Attested:      false,
		AttestationID: 0,
		Finalized:     false,
		FinalizedAt:   0,
	}, nil
}

package relayer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"

	"cosmossdk.io/log"

	attestationtypes "github.com/crypto-org-chain/cronos/relayer/types"
)

// blockForwarder implements BlockForwarder interface
type blockForwarder struct {
	clientCtx       client.Context
	config          *Config
	logger          log.Logger
	finalityMonitor FinalityMonitor
}

// NewBlockForwarder creates a new block forwarder
func NewBlockForwarder(
	clientCtx client.Context,
	config *Config,
	logger log.Logger,
	finalityMonitor FinalityMonitor,
) (BlockForwarder, error) {
	return &blockForwarder{
		clientCtx:       clientCtx,
		config:          config,
		logger:          logger.With("component", "block_forwarder"),
		finalityMonitor: finalityMonitor,
	}, nil
}

// ForwardBlock sends block data to attestation layer
func (bf *blockForwarder) ForwardBlock(ctx context.Context, blockData *BlockData) (uint64, error) {
	bf.logger.Info("Forwarding block to attestation layer",
		"chain_id", blockData.ChainID,
		"block_height", blockData.BlockHeight,
	)

	// Create MsgSubmitBlockAttestation
	msg, err := bf.createBlockAttestationMsg(blockData)
	if err != nil {
		return 0, fmt.Errorf("failed to create block attestation message: %w", err)
	}

	// Broadcast transaction
	txResp, err := bf.broadcastTx(ctx, msg)
	if err != nil {
		return 0, fmt.Errorf("failed to broadcast block attestation: %w", err)
	}

	// Parse attestation ID from response
	attestationID, err := bf.parseAttestationID(txResp)
	if err != nil {
		bf.logger.Warn("Failed to parse attestation ID from response",
			"error", err,
			"tx_hash", txResp.TxHash,
		)
		// Return block height as fallback
		return blockData.BlockHeight, nil
	}

	bf.logger.Info("Block forwarded successfully",
		"block_height", blockData.BlockHeight,
		"attestation_id", attestationID,
		"tx_hash", txResp.TxHash,
	)

	// Track attestation in finality monitor
	if bf.finalityMonitor != nil {
		bf.finalityMonitor.TrackAttestation(
			txResp.TxHash,
			attestationID,
			blockData.ChainID,
			blockData.BlockHeight,
		)
	}

	return attestationID, nil
}

// BatchForwardBlocks forwards multiple blocks to attestation layer
func (bf *blockForwarder) BatchForwardBlocks(ctx context.Context, blocks []*BlockData) ([]uint64, error) {
	if len(blocks) == 0 {
		return nil, nil
	}

	bf.logger.Info("Batch forwarding blocks to attestation layer",
		"count", len(blocks),
		"first_height", blocks[0].BlockHeight,
		"last_height", blocks[len(blocks)-1].BlockHeight,
	)

	// Create MsgSubmitBatchBlockAttestation
	msg, err := bf.createBatchBlockAttestationMsg(blocks)
	if err != nil {
		return nil, fmt.Errorf("failed to create batch block attestation message: %w", err)
	}

	// Broadcast transaction
	txResp, err := bf.broadcastTx(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast batch block attestation: %w", err)
	}

	// Parse attestation IDs from response
	attestationIDs, err := bf.parseBatchAttestationIDs(txResp)
	if err != nil {
		bf.logger.Warn("Failed to parse attestation IDs from response",
			"error", err,
			"tx_hash", txResp.TxHash,
		)
		// Return block heights as fallback
		ids := make([]uint64, len(blocks))
		for i, block := range blocks {
			ids[i] = block.BlockHeight
		}
		return ids, nil
	}

	bf.logger.Info("Batch blocks forwarded successfully",
		"count", len(blocks),
		"attestation_ids", attestationIDs,
		"tx_hash", txResp.TxHash,
	)

	// Track batch attestation in finality monitor
	if bf.finalityMonitor != nil {
		bf.finalityMonitor.TrackBatchAttestation(
			txResp.TxHash,
			attestationIDs,
			blocks[0].ChainID,
			blocks[0].BlockHeight,
			blocks[len(blocks)-1].BlockHeight,
		)
	}

	return attestationIDs, nil
}

// createBlockAttestationMsg creates a MsgSubmitBlockAttestation from BlockData
func (bf *blockForwarder) createBlockAttestationMsg(blockData *BlockData) (*attestationtypes.MsgSubmitBlockAttestation, error) {
	// Get relayer address
	relayerAddr := bf.clientCtx.GetFromAddress()
	if relayerAddr == nil {
		return nil, fmt.Errorf("relayer address not found in client context")
	}

	// Encode block header
	blockHeaderBytes, err := bf.encodeBlockHeader(blockData.BlockHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to encode block header: %w", err)
	}

	// Encode tx results
	txResultsBytes, err := bf.encodeTxResults(blockData.TxResults)
	if err != nil {
		return nil, fmt.Errorf("failed to encode tx results: %w", err)
	}

	// Encode finalize block events
	finalizeBlockEventsBytes, err := bf.encodeEvents(blockData.FinalizeBlockEvents)
	if err != nil {
		return nil, fmt.Errorf("failed to encode finalize block events: %w", err)
	}

	// Encode validator updates
	validatorUpdatesBytes, err := bf.encodeValidatorUpdates(blockData.ValidatorUpdates)
	if err != nil {
		return nil, fmt.Errorf("failed to encode validator updates: %w", err)
	}

	// Encode consensus param updates
	consensusParamUpdatesBytes, err := bf.encodeConsensusParamUpdates(blockData.ConsensusParamUpdates)
	if err != nil {
		return nil, fmt.Errorf("failed to encode consensus param updates: %w", err)
	}

	// Create message
	msg := &attestationtypes.MsgSubmitBlockAttestation{
		Relayer:               relayerAddr.String(),
		ChainId:               blockData.ChainID,
		BlockHeight:           blockData.BlockHeight,
		Timestamp:             blockData.Timestamp,
		BlockHash:             blockData.BlockHash,
		AppHash:               blockData.AppHash,
		BlockHeader:           blockHeaderBytes,
		TxResults:             txResultsBytes,
		FinalizeBlockEvents:   finalizeBlockEventsBytes,
		ValidatorUpdates:      validatorUpdatesBytes,
		ConsensusParamUpdates: consensusParamUpdatesBytes,
		Signature:             blockData.Signature,
	}

	return msg, nil
}

// createBatchBlockAttestationMsg creates a MsgSubmitBatchBlockAttestation from multiple BlockData
func (bf *blockForwarder) createBatchBlockAttestationMsg(blocks []*BlockData) (*attestationtypes.MsgSubmitBatchBlockAttestation, error) {
	// Get relayer address
	relayerAddr := bf.clientCtx.GetFromAddress()
	if relayerAddr == nil {
		return nil, fmt.Errorf("relayer address not found in client context")
	}

	// All blocks should be from the same chain
	chainID := blocks[0].ChainID

	// Create attestation data for each block
	attestations := make([]*attestationtypes.BlockAttestationData, len(blocks))
	for i, block := range blocks {
		if block.ChainID != chainID {
			return nil, fmt.Errorf("all blocks must be from the same chain")
		}

		// Encode block data
		blockHeaderBytes, err := bf.encodeBlockHeader(block.BlockHeader)
		if err != nil {
			return nil, fmt.Errorf("failed to encode block header for height %d: %w", block.BlockHeight, err)
		}

		txResultsBytes, err := bf.encodeTxResults(block.TxResults)
		if err != nil {
			return nil, fmt.Errorf("failed to encode tx results for height %d: %w", block.BlockHeight, err)
		}

		finalizeBlockEventsBytes, err := bf.encodeEvents(block.FinalizeBlockEvents)
		if err != nil {
			return nil, fmt.Errorf("failed to encode finalize block events for height %d: %w", block.BlockHeight, err)
		}

		validatorUpdatesBytes, err := bf.encodeValidatorUpdates(block.ValidatorUpdates)
		if err != nil {
			return nil, fmt.Errorf("failed to encode validator updates for height %d: %w", block.BlockHeight, err)
		}

		consensusParamUpdatesBytes, err := bf.encodeConsensusParamUpdates(block.ConsensusParamUpdates)
		if err != nil {
			return nil, fmt.Errorf("failed to encode consensus param updates for height %d: %w", block.BlockHeight, err)
		}

		attestations[i] = &attestationtypes.BlockAttestationData{
			BlockHeight:           block.BlockHeight,
			Timestamp:             block.Timestamp,
			BlockHash:             block.BlockHash,
			AppHash:               block.AppHash,
			BlockHeader:           blockHeaderBytes,
			TxResults:             txResultsBytes,
			FinalizeBlockEvents:   finalizeBlockEventsBytes,
			ValidatorUpdates:      validatorUpdatesBytes,
			ConsensusParamUpdates: consensusParamUpdatesBytes,
		}
	}

	// Create batch message
	msg := &attestationtypes.MsgSubmitBatchBlockAttestation{
		Relayer:      relayerAddr.String(),
		ChainId:      chainID,
		Attestations: attestations,
	}

	return msg, nil
}

// broadcastTx broadcasts a transaction to the attestation chain
func (bf *blockForwarder) broadcastTx(ctx context.Context, msg sdk.Msg) (*sdk.TxResponse, error) {
	// Create transaction factory with gas price as string
	gasPriceStr := bf.config.GasPrices.String()

	txf := tx.Factory{}.
		WithChainID(bf.clientCtx.ChainID).
		WithKeybase(bf.clientCtx.Keyring).
		WithTxConfig(bf.clientCtx.TxConfig).
		WithGasPrices(gasPriceStr).
		WithGasAdjustment(bf.config.GasAdjustment).
		WithAccountRetriever(bf.clientCtx.AccountRetriever).
		WithSimulateAndExecute(true) // Auto-calculate gas

	// Prepare account (fetch account number and sequence)
	if err := txf.AccountRetriever().EnsureExists(bf.clientCtx, bf.clientCtx.GetFromAddress()); err != nil {
		return nil, fmt.Errorf("failed to ensure account exists: %w", err)
	}

	// Get account details
	num, seq, err := txf.AccountRetriever().GetAccountNumberSequence(bf.clientCtx, bf.clientCtx.GetFromAddress())
	if err != nil {
		return nil, fmt.Errorf("failed to get account number and sequence: %w", err)
	}
	txf = txf.WithAccountNumber(num).WithSequence(seq)

	// Build transaction
	txBuilder, err := txf.BuildUnsignedTx(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to build unsigned tx: %w", err)
	}

	// Calculate and set gas if needed
	if txf.SimulateAndExecute() {
		_, adjusted, err := tx.CalculateGas(bf.clientCtx, txf, msg)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate gas: %w", err)
		}
		txf = txf.WithGas(adjusted)
		txBuilder, err = txf.BuildUnsignedTx(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to rebuild tx with gas: %w", err)
		}
	}

	// Sign transaction
	if err := tx.Sign(ctx, txf, bf.clientCtx.GetFromName(), txBuilder, true); err != nil {
		return nil, fmt.Errorf("failed to sign tx: %w", err)
	}

	// Encode transaction
	txBytes, err := bf.clientCtx.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, fmt.Errorf("failed to encode tx: %w", err)
	}

	// Set broadcast mode (async or sync)
	broadcastMode := bf.config.BroadcastMode
	if broadcastMode == "" {
		broadcastMode = "async" // Default to async for better performance
	}

	bf.logger.Debug("Broadcasting transaction",
		"mode", broadcastMode,
		"from", bf.clientCtx.GetFromAddress().String(),
	)

	// Update client context with broadcast mode
	broadcastCtx := bf.clientCtx.WithBroadcastMode(broadcastMode)

	// Broadcast transaction
	res, err := broadcastCtx.BroadcastTx(txBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast tx: %w", err)
	}

	// For async mode, we get immediate response without CheckTx validation
	// Code will be 0 if successfully submitted to mempool
	// For sync mode, Code 0 means passed CheckTx
	if res.Code != 0 {
		return nil, fmt.Errorf("transaction failed with code %d: %s", res.Code, res.RawLog)
	}

	bf.logger.Debug("Transaction broadcast successful",
		"mode", broadcastMode,
		"tx_hash", res.TxHash,
		"code", res.Code,
	)

	return res, nil
}

// parseAttestationID extracts the attestation ID from transaction response
func (bf *blockForwarder) parseAttestationID(txResp *sdk.TxResponse) (uint64, error) {
	// Look for attestation_id in events
	for _, event := range txResp.Events {
		if event.Type == "submit_block_attestation" || event.Type == "attestation.v1.MsgSubmitBlockAttestation" {
			for _, attr := range event.Attributes {
				if attr.Key == "attestation_id" {
					var id uint64
					if _, err := fmt.Sscanf(attr.Value, "%d", &id); err == nil {
						return id, nil
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("attestation_id not found in transaction events")
}

// parseBatchAttestationIDs extracts multiple attestation IDs from transaction response
func (bf *blockForwarder) parseBatchAttestationIDs(txResp *sdk.TxResponse) ([]uint64, error) {
	// Look for attestation_ids in events
	for _, event := range txResp.Events {
		if event.Type == "submit_batch_block_attestation" || event.Type == "attestation.v1.MsgSubmitBatchBlockAttestation" {
			for _, attr := range event.Attributes {
				if attr.Key == "attestation_ids" {
					// Parse JSON array of IDs
					var ids []uint64
					if err := json.Unmarshal([]byte(attr.Value), &ids); err == nil {
						return ids, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("attestation_ids not found in transaction events")
}

// Encoding helper functions

func (bf *blockForwarder) encodeBlockHeader(header interface{}) ([]byte, error) {
	if header == nil {
		return nil, nil
	}

	// If it's already a proto.Message, marshal it
	if protoMsg, ok := header.(proto.Message); ok {
		return proto.Marshal(protoMsg)
	}

	// Otherwise, use JSON encoding as fallback
	return json.Marshal(header)
}

func (bf *blockForwarder) encodeTxResults(results interface{}) ([]byte, error) {
	if results == nil {
		return nil, nil
	}

	// If it's a proto.Message, marshal it
	if protoMsg, ok := results.(proto.Message); ok {
		return proto.Marshal(protoMsg)
	}

	// For slice of proto messages
	return json.Marshal(results)
}

func (bf *blockForwarder) encodeEvents(events interface{}) ([]byte, error) {
	if events == nil {
		return nil, nil
	}

	// Encode events as JSON
	return json.Marshal(events)
}

func (bf *blockForwarder) encodeValidatorUpdates(updates interface{}) ([]byte, error) {
	if updates == nil {
		return nil, nil
	}

	// Encode validator updates as JSON
	return json.Marshal(updates)
}

func (bf *blockForwarder) encodeConsensusParamUpdates(updates interface{}) ([]byte, error) {
	if updates == nil {
		return nil, nil
	}

	// Encode consensus param updates as JSON
	return json.Marshal(updates)
}

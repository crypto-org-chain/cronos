package relayer

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/log"
	"github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"

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

// BatchForwardBlocks forwards multiple blocks to attestation layer
func (bf *blockForwarder) BatchForwardBlocks(ctx context.Context, blocks []*types.EventDataNewBlock) ([]uint64, error) {
	if len(blocks) == 0 {
		return nil, nil
	}

	bf.logger.Info("Batch forwarding blocks to attestation layer",
		"count", len(blocks),
		"first_height", blocks[0].Block.Height,
		"last_height", blocks[len(blocks)-1].Block.Height,
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

	// Parse MsgSubmitBatchBlockAttestationResponse from transaction response
	batchResp, err := bf.parseBatchAttestationResponse(txResp)
	if err != nil {
		bf.logger.Error("Failed to parse batch attestation response",
			"error", err,
			"tx_hash", txResp.TxHash,
		)
		return nil, fmt.Errorf("failed to parse batch attestation response: %w", err)
	}

	attestationIDs := batchResp.AttestationIds
	finalizedCount := batchResp.FinalizedCount

	// Verify attestation IDs are in continuously ascending order
	if err := bf.verifyAttestationIDsOrder(attestationIDs); err != nil {
		bf.logger.Error("Attestation IDs are not in correct order",
			"error", err,
			"attestation_ids", attestationIDs,
		)
		return nil, fmt.Errorf("invalid attestation IDs order: %w", err)
	}

	bf.logger.Info("Batch blocks forwarded successfully",
		"count", len(blocks),
		"attestation_ids", attestationIDs,
		"finalized_count", finalizedCount,
		"tx_hash", txResp.TxHash,
	)

	// Track in finality monitor based on broadcast mode
	if bf.finalityMonitor != nil {
		isSyncMode := bf.config.BroadcastMode == "sync"

		if isSyncMode {
			// Sync mode: verify finalized_count matches attestation count
			if finalizedCount != uint32(len(attestationIDs)) {
				bf.logger.Warn("Finalized count mismatch in sync mode",
					"finalized_count", finalizedCount,
					"attestation_count", len(attestationIDs),
				)
			}

			// In sync mode, blocks are already finalized
			bf.finalityMonitor.TrackBatchAttestationFinalized(
				txResp.TxHash,
				attestationIDs,
				blocks[0].Block.Header.ChainID,
				uint64(blocks[0].Block.Height),
				uint64(blocks[len(blocks)-1].Block.Height),
				finalizedCount,
			)
		} else {
			// Async mode: track as pending, finality confirmed later via events
			bf.finalityMonitor.TrackBatchAttestation(
				txResp.TxHash,
				attestationIDs,
				blocks[0].Block.Header.ChainID,
				uint64(blocks[0].Block.Height),
				uint64(blocks[len(blocks)-1].Block.Height),
			)
		}
	}

	return attestationIDs, nil
}

// createBatchBlockAttestationMsg creates a MsgSubmitBatchBlockAttestation from multiple EventDataNewBlock
func (bf *blockForwarder) createBatchBlockAttestationMsg(blocks []*types.EventDataNewBlock) (*attestationtypes.MsgSubmitBatchBlockAttestation, error) {
	// Get relayer address
	relayerAddr := bf.clientCtx.GetFromAddress()
	if relayerAddr == nil {
		return nil, fmt.Errorf("relayer address not found in client context")
	}

	// All blocks should be from the same chain
	chainID := blocks[0].Block.Header.ChainID

	// Create attestation data for each block
	attestations := make([]*attestationtypes.BlockAttestationData, len(blocks))
	for i, eventData := range blocks {
		if eventData.Block.Header.ChainID != chainID {
			return nil, fmt.Errorf("all blocks must be from the same chain")
		}

		// Extract data from EventDataNewBlock
		block := eventData.Block
		blockID := eventData.BlockID
		results := eventData.ResultFinalizeBlock

		// Encode block header
		blockHeaderBytes, err := bf.encodeBlockHeader(block.Header)
		if err != nil {
			return nil, fmt.Errorf("failed to encode block header for height %d: %w", block.Height, err)
		}

		// Encode tx results
		txResultsBytes, err := bf.encodeTxResults(results.TxResults)
		if err != nil {
			return nil, fmt.Errorf("failed to encode tx results for height %d: %w", block.Height, err)
		}

		// Encode finalize block events
		finalizeBlockEventsBytes, err := bf.encodeEvents(results.Events)
		if err != nil {
			return nil, fmt.Errorf("failed to encode finalize block events for height %d: %w", block.Height, err)
		}

		// Encode validator updates
		validatorUpdatesBytes, err := bf.encodeValidatorUpdates(results.ValidatorUpdates)
		if err != nil {
			return nil, fmt.Errorf("failed to encode validator updates for height %d: %w", block.Height, err)
		}

		// Encode consensus param updates
		consensusParamUpdatesBytes, err := bf.encodeConsensusParamUpdates(results.ConsensusParamUpdates)
		if err != nil {
			return nil, fmt.Errorf("failed to encode consensus param updates for height %d: %w", block.Height, err)
		}

		// Encode raw transaction data (for reconstruction)
		transactionsBytes, err := bf.encodeTransactions(block.Data.Txs)
		if err != nil {
			return nil, fmt.Errorf("failed to encode transactions for height %d: %w", block.Height, err)
		}

		// Encode evidence
		evidenceBytes, err := bf.encodeEvidence(block.Evidence)
		if err != nil {
			return nil, fmt.Errorf("failed to encode evidence for height %d: %w", block.Height, err)
		}

		// Encode last commit
		lastCommitBytes, err := bf.encodeLastCommit(block.LastCommit)
		if err != nil {
			return nil, fmt.Errorf("failed to encode last commit for height %d: %w", block.Height, err)
		}

		attestations[i] = &attestationtypes.BlockAttestationData{
			// Indexing fields
			BlockHeight: uint64(block.Height),
			BlockHash:   blockID.Hash,
			// Complete block structure (block_header includes timestamp and app_hash)
			BlockHeader:  blockHeaderBytes,
			Transactions: transactionsBytes,
			Evidence:     evidenceBytes,
			LastCommit:   lastCommitBytes,
			// Execution data
			TxResults:             txResultsBytes,
			FinalizeBlockEvents:   finalizeBlockEventsBytes,
			ValidatorUpdates:      validatorUpdatesBytes,
			ConsensusParamUpdates: consensusParamUpdatesBytes,
		}
	}

	// Sign the batch attestation
	signature, err := bf.signBatchAttestation(chainID, attestations)
	if err != nil {
		return nil, fmt.Errorf("failed to sign batch attestation: %w", err)
	}

	// Convert []*BlockAttestationData to []BlockAttestationData (dereference pointers)
	valueAttestations := make([]attestationtypes.BlockAttestationData, len(attestations))
	for i, att := range attestations {
		if att != nil {
			valueAttestations[i] = *att
		}
	}

	// Create batch message
	msg := &attestationtypes.MsgSubmitBatchBlockAttestation{
		Relayer:      relayerAddr.String(),
		ChainId:      chainID,
		Attestations: valueAttestations,
		Signature:    signature,
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

// parseBatchAttestationResponse extracts MsgSubmitBatchBlockAttestationResponse from transaction response
func (bf *blockForwarder) parseBatchAttestationResponse(txResp *sdk.TxResponse) (*attestationtypes.MsgSubmitBatchBlockAttestationResponse, error) {
	// The response should be in txResp.Data or in the Msg response
	// Try to unmarshal from transaction result
	if txResp.Data != "" {
		// Data is hex-encoded
		data, err := json.Marshal(txResp)
		if err == nil {
			var resp attestationtypes.MsgSubmitBatchBlockAttestationResponse
			if err := json.Unmarshal(data, &resp); err == nil {
				return &resp, nil
			}
		}
	}

	return nil, fmt.Errorf("%s", txResp.String())
}

// verifyAttestationIDsOrder verifies that attestation IDs are in continuously ascending order
func (bf *blockForwarder) verifyAttestationIDsOrder(ids []uint64) error {
	if len(ids) == 0 {
		return fmt.Errorf("empty attestation IDs")
	}

	if len(ids) == 1 {
		return nil // Single ID is always valid
	}

	// Check each ID is exactly 1 more than the previous
	for i := 1; i < len(ids); i++ {
		if ids[i] != ids[i-1]+1 {
			return fmt.Errorf("attestation IDs not continuously ascending: ids[%d]=%d, ids[%d]=%d (expected %d)",
				i-1, ids[i-1], i, ids[i], ids[i-1]+1)
		}
	}

	return nil
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

// signBatchAttestation creates a signature over the batch attestation data
func (bf *blockForwarder) signBatchAttestation(chainID string, attestations []*attestationtypes.BlockAttestationData) ([]byte, error) {
	// Create signing data by concatenating all attestation data
	var signingData []byte

	// Add chain ID
	signingData = append(signingData, []byte(chainID)...)

	// Add each attestation data
	for _, att := range attestations {
		// Marshal attestation to get deterministic bytes
		attBytes, err := proto.Marshal(att)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal attestation for signing: %w", err)
		}
		signingData = append(signingData, attBytes...)
	}

	// Get key name from client context
	keyName := bf.clientCtx.GetFromName()
	if keyName == "" {
		return nil, fmt.Errorf("no key name in client context")
	}

	// Sign the data (using direct mode - SIGN_MODE_DIRECT = 1)
	signature, _, err := bf.clientCtx.Keyring.Sign(keyName, signingData, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to sign batch attestation: %w", err)
	}

	bf.logger.Debug("Signed batch attestation",
		"chain_id", chainID,
		"attestation_count", len(attestations),
		"signature_length", len(signature),
	)

	return signature, nil
}

// encodeTransactions encodes raw transaction data
func (bf *blockForwarder) encodeTransactions(txs types.Txs) ([]byte, error) {
	if txs == nil || len(txs) == 0 {
		return nil, nil
	}
	// Txs is []Tx where Tx is []byte
	// Encode as array of byte arrays
	return json.Marshal(txs)
}

// encodeEvidence encodes block evidence
func (bf *blockForwarder) encodeEvidence(evidence types.EvidenceData) ([]byte, error) {
	// EvidenceData contains evidence list
	return json.Marshal(evidence)
}

// encodeLastCommit encodes the last commit (validator signatures)
func (bf *blockForwarder) encodeLastCommit(commit *types.Commit) ([]byte, error) {
	if commit == nil {
		return nil, nil
	}
	// Commit contains block signatures from validators
	return json.Marshal(commit)
}

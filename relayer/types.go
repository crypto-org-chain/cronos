package relayer

import (
	"context"
	"time"

	"github.com/cometbft/cometbft/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// FinalityInfo represents finality information from attestation layer
type FinalityInfo struct {
	AttestationID     uint64 `json:"attestation_id"`
	ChainID           string `json:"chain_id"`
	BlockHeight       uint64 `json:"block_height"`
	Finalized         bool   `json:"finalized"`
	FinalizedAt       int64  `json:"finalized_at"`
	FinalityProof     []byte `json:"finality_proof,omitempty"`
	AttestationTxHash []byte `json:"attestation_tx_hash"`
}

// ForcedTx represents a forced transaction from attestation layer
type ForcedTx struct {
	ForcedTxID      uint64       `json:"forced_tx_id"`
	Submitter       string       `json:"submitter"`
	TargetChainID   string       `json:"target_chain_id"`
	TargetChainType string       `json:"target_chain_type"`
	Priority        uint32       `json:"priority"`
	TxType          ForcedTxType `json:"tx_type"`
	TransactionData []byte       `json:"transaction_data"`
	Deadline        uint64       `json:"deadline"`
	Metadata        string       `json:"metadata,omitempty"`
	SubmittedAt     int64        `json:"submitted_at"`
	Executed        bool         `json:"executed"`
	ExecutedAt      int64        `json:"executed_at,omitempty"`
	ExecutionTxHash []byte       `json:"execution_tx_hash,omitempty"`
}

// ForcedTxType defines the type of forced transaction
type ForcedTxType uint8

const (
	ForcedTxTypeNormal ForcedTxType = iota
	ForcedTxTypeGovernance
	ForcedTxTypeEscapeHatch
	ForcedTxTypeEmergency
)

// ChainMonitor monitors a Cosmos SDK chain
type ChainMonitor interface {
	// Start begins monitoring the chain
	Start(ctx context.Context) error

	// Stop stops the monitor
	Stop() error

	// GetLatestHeight returns the latest block height
	GetLatestHeight(ctx context.Context) (uint64, error)

	// GetBlock retrieves block data for a specific height
	GetBlock(ctx context.Context, height uint64) (*types.EventDataNewBlock, error)

	// SubscribeNewBlocks subscribes to new block events
	SubscribeNewBlocks(ctx context.Context) (<-chan *types.EventDataNewBlock, error)
}

// BlockForwarder forwards ABCI blocks to attestation layer
type BlockForwarder interface {
	// BatchForwardBlocks forwards blocks to attestation layer (supports single or multiple blocks)
	BatchForwardBlocks(ctx context.Context, blocks []*types.EventDataNewBlock) ([]uint64, error)
}

// AttestationStatus represents the attestation status of a block
type AttestationStatus struct {
	Attested      bool   `json:"attested"`
	AttestationID uint64 `json:"attestation_id"`
	Finalized     bool   `json:"finalized"`
	FinalizedAt   int64  `json:"finalized_at"`
}

// FinalityMonitor monitors finality events from attestation layer
type FinalityMonitor interface {
	// Start begins monitoring finality events
	Start(ctx context.Context) error

	// Stop stops the monitor
	Stop() error

	// TrackBatchAttestation tracks a batch attestation (called by block forwarder) - async mode
	TrackBatchAttestation(txHash string, attestationIDs []uint64, chainID string, startHeight, endHeight uint64)

	// TrackBatchAttestationFinalized tracks a batch attestation that's already finalized - sync mode
	TrackBatchAttestationFinalized(txHash string, attestationIDs []uint64, chainID string, firstHeight, lastHeight uint64, finalizedCount uint32)

	// GetPendingAttestations returns the count of pending attestations
	GetPendingAttestations() int

	// SubscribeFinality subscribes to finality events
	SubscribeFinality(ctx context.Context) (<-chan *FinalityInfo, error)

	// GetFinalityStatus retrieves finality status for a block
	GetFinalityStatus(ctx context.Context, chainID string, height uint64) (*FinalityInfo, error)
}

// FinalityStore persists finality information
type FinalityStore interface {
	// SaveFinalityInfo saves finality information
	SaveFinalityInfo(ctx context.Context, info *FinalityInfo) error

	// GetFinalityInfo retrieves finality information
	GetFinalityInfo(ctx context.Context, chainID string, height uint64) (*FinalityInfo, error)

	// GetLatestFinalized returns the latest finalized block height for a chain
	GetLatestFinalized(ctx context.Context, chainID string) (uint64, error)

	// ListPendingFinality lists blocks pending finality
	ListPendingFinality(ctx context.Context, chainID string, limit int) ([]*FinalityInfo, error)

	// GetStats returns statistics about the finality store
	GetStats(ctx context.Context, chainID string) (*FinalityStoreStats, error)

	// Close closes the finality store
	Close() error
}

// ForcedTxMonitor monitors forced transactions from attestation layer
type ForcedTxMonitor interface {
	// Start begins monitoring forced transactions
	Start(ctx context.Context) error

	// Stop stops the monitor
	Stop() error

	// SubscribeForcedTx subscribes to forced transaction events
	SubscribeForcedTx(ctx context.Context) (<-chan *ForcedTx, error)

	// GetPendingForcedTxs retrieves pending forced transactions for target chain
	GetPendingForcedTxs(ctx context.Context, targetChainID string) ([]*ForcedTx, error)
}

// ForcedTxExecutor executes forced transactions on the target chain
type ForcedTxExecutor interface {
	// ExecuteForcedTx executes a forced transaction
	ExecuteForcedTx(ctx context.Context, tx *ForcedTx) error

	// BatchExecuteForcedTx executes multiple forced transactions
	BatchExecuteForcedTx(ctx context.Context, txs []*ForcedTx) error

	// ConfirmExecution reports execution back to attestation layer
	ConfirmExecution(ctx context.Context, forcedTxID uint64, executionTxHash []byte, executionHeight uint64) error
}

// Config holds the relayer configuration
type Config struct {
	// Source chain (Cronos) configuration
	SourceChainID string `json:"source_chain_id"`
	SourceRPC     string `json:"source_rpc"`
	SourceGRPC    string `json:"source_grpc"`

	// Attestation layer configuration
	AttestationChainID string `json:"attestation_chain_id"`
	AttestationRPC     string `json:"attestation_rpc"`
	AttestationGRPC    string `json:"attestation_grpc"`

	// Relayer configuration
	RelayerMnemonic string `json:"relayer_mnemonic"`
	RelayerAddress  string `json:"relayer_address"`

	// Performance tuning
	BlockBatchSize uint          `json:"block_batch_size"`
	MaxRetries     uint          `json:"max_retries"`
	RetryDelay     time.Duration `json:"retry_delay"`

	// Monitoring intervals
	BlockPollInterval    time.Duration `json:"block_poll_interval"`
	FinalityPollInterval time.Duration `json:"finality_poll_interval"`
	ForcedTxPollInterval time.Duration `json:"forced_tx_poll_interval"`

	// Gas configuration
	GasAdjustment float64      `json:"gas_adjustment"`
	GasPrices     sdk.DecCoins `json:"gas_prices"`

	// Transaction broadcast configuration
	BroadcastMode string `json:"broadcast_mode"` // "sync" or "async"

	// Data store configuration
	FinalityStoreType string `json:"finality_store_type"` // "memory", "leveldb", "rocksdb"
	FinalityStorePath string `json:"finality_store_path"`

	// Checkpoint configuration for crash recovery
	CheckpointPath string `json:"checkpoint_path"` // Path to checkpoint file

	// RPC server configuration (optional)
	RPCEnabled bool       `json:"rpc_enabled"` // Enable RPC server
	RPCConfig  *RPCConfig `json:"rpc_config,omitempty"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		BlockBatchSize:       10,
		MaxRetries:           3,
		RetryDelay:           5 * time.Second,
		BlockPollInterval:    2 * time.Second,
		FinalityPollInterval: 5 * time.Second,
		ForcedTxPollInterval: 3 * time.Second,
		GasAdjustment:        1.5,
		BroadcastMode:        "async", // Use async mode for better performance
		FinalityStoreType:    "leveldb",
	}
}

// EventBlockAttested represents the block attestation event
type EventBlockAttested struct {
	AttestationID uint64 `json:"attestation_id"`
	ChainID       string `json:"chain_id"`
	BlockHeight   uint64 `json:"block_height"`
	Relayer       string `json:"relayer"`
	Finalized     bool   `json:"finalized"`
	FinalityProof []byte `json:"finality_proof,omitempty"`
	ProcessedAt   int64  `json:"processed_at"`
}

// EventBlockFinalized represents the block finality event
type EventBlockFinalized struct {
	ChainID           string `json:"chain_id"`
	BlockHeight       uint64 `json:"block_height"`
	FinalizedAt       int64  `json:"finalized_at"`
	ValidatorCount    uint32 `json:"validator_count"`
	FinalitySignature []byte `json:"finality_signature,omitempty"`
	AttestationTxHash []byte `json:"attestation_tx_hash"`
}

// EventForcedTxSubmitted represents the forced transaction submission event
type EventForcedTxSubmitted struct {
	ForcedTxID      uint64       `json:"forced_tx_id"`
	Submitter       string       `json:"submitter"`
	TargetChainID   string       `json:"target_chain_id"`
	TargetChainType string       `json:"target_chain_type"`
	Priority        uint32       `json:"priority"`
	TxType          ForcedTxType `json:"tx_type"`
	Deadline        uint64       `json:"deadline"`
	SubmittedAt     int64        `json:"submitted_at"`
}

// RelayerStatus represents the status of the relayer
type RelayerStatus struct {
	Running              bool      `json:"running"`
	SourceChainID        string    `json:"source_chain_id"`
	AttestationChainID   string    `json:"attestation_chain_id"`
	LastBlockForwarded   uint64    `json:"last_block_forwarded"`
	LastFinalityReceived uint64    `json:"last_finality_received"`
	FinalizedBlocksCount uint64    `json:"finalized_blocks_count"`
	LastError            string    `json:"last_error,omitempty"`
	UpdatedAt            time.Time `json:"updated_at"`
}

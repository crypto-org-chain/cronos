package config

import (
	"fmt"
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SetBech32Prefixes sets the global prefixes to be used when serializing addresses and public keys to Bech32 strings.
func SetBech32Prefixes(config *sdk.Config) {
	config.SetBech32PrefixForAccount(Bech32PrefixAccAddr, Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(Bech32PrefixValAddr, Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(Bech32PrefixConsAddr, Bech32PrefixConsPub)
}

type CronosConfig struct {
	// Set to true to disable tx replacement.
	DisableTxReplacement bool `mapstructure:"disable-tx-replacement"`
	// Set to true to disable optimistic execution.
	DisableOptimisticExecution bool `mapstructure:"disable-optimistic-execution"`
	// Capacity of the sharded LRU tx-decode cache. Set to 0 to disable.
	TxDecodeCacheSize int `mapstructure:"tx-decode-cache-size"`
	// Per-entry raw payload byte cap. Txs larger than this are decoded but
	// not cached, bounding heap impact. Should not exceed mempool.max-tx-bytes.
	TxDecodeCacheMaxTxBytes int `mapstructure:"tx-decode-cache-max-tx-bytes"`
	// MempoolGossipTTL is the re-gossip suppression window for mempool.type=app:
	// a tx reaped for gossip is not re-broadcast until this elapses. Bounds the
	// AppReactor's per-tick re-broadcast of the whole pool. <=0 uses the default.
	MempoolGossipTTL time.Duration `mapstructure:"mempool-gossip-ttl"`
	// MempoolGossipMaxPerReap caps txs returned per gossip reap (mempool.type=app),
	// spreading a large pool across reap ticks instead of one libp2p batch.
	// <=0 disables the count cap (only reap_max_bytes/reap_max_gas apply).
	MempoolGossipMaxPerReap int `mapstructure:"mempool-gossip-max-per-reap"`
	// MempoolRecheckBatchSize caps the number of candidate txs re-validated per
	// Commit cycle, bounding RunTx(ReCheck) time under deep pools. <=0 = unlimited.
	MempoolRecheckBatchSize int `mapstructure:"mempool-recheck-batch-size"`
}

// Defaults live here (not app/) because app/ imports this package and both
// DefaultCronosConfig() and app's New() need them.
const (
	DefaultTxDecodeCacheSize       = 10000
	DefaultTxDecodeCacheMaxTxBytes = 65536
	DefaultTxEncodeCacheSize       = 10000
	// DefaultMempoolGossipTTL re-gossips a resident tx at most ~once per window;
	// far above CometBFT's 500ms ReapInterval so steady state suppresses re-reap.
	DefaultMempoolGossipTTL = 15 * time.Second
	// DefaultMempoolTxsPerBlock is one block's tx budget (~2900 = cronos mainnet
	// empirical block size). Used as both the gossip-reap cap (one tick ≈ one block
	// interval) and the recheck-batch cap (one commit ≈ one block of senders).
	DefaultMempoolTxsPerBlock      = 2900
	DefaultMempoolGossipMaxPerReap = DefaultMempoolTxsPerBlock
	DefaultMempoolRecheckBatchSize = DefaultMempoolTxsPerBlock
)

const (
	NodeTypeDefault   = ""
	NodeTypeValidator = "validator"
	NodeTypeRPC       = "rpc"
	NodeTypeArchive   = "archive"
)

type RocksDBConfig struct {
	// Defines the tuning profile for RocksDB based on the node's primary workload.
	// Valid values: "", "validator", "rpc", "archive"
	NodeType string `mapstructure:"node_type"`
}

func (c *RocksDBConfig) Validate() error {
	normalized := strings.ToLower(strings.TrimSpace(c.NodeType))
	switch normalized {
	case NodeTypeDefault, NodeTypeValidator, NodeTypeRPC, NodeTypeArchive:
		c.NodeType = normalized
		return nil
	default:
		return fmt.Errorf("invalid rocksdb.node_type %q: allowed values are %q, %q, %q, or %q (empty)",
			c.NodeType, NodeTypeValidator, NodeTypeRPC, NodeTypeArchive, NodeTypeDefault)
	}
}

func DefaultCronosConfig() CronosConfig {
	return CronosConfig{
		DisableTxReplacement:       false,
		DisableOptimisticExecution: false,
		TxDecodeCacheSize:          DefaultTxDecodeCacheSize,
		TxDecodeCacheMaxTxBytes:    DefaultTxDecodeCacheMaxTxBytes,
		MempoolGossipTTL:           DefaultMempoolGossipTTL,
		MempoolGossipMaxPerReap:    DefaultMempoolGossipMaxPerReap,
		MempoolRecheckBatchSize:    DefaultMempoolRecheckBatchSize,
	}
}

func DefaultRocksDBConfig() RocksDBConfig {
	return RocksDBConfig{
		NodeType: "",
	}
}

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
	// Capacity of the sharded LRU tx encode/decode cache.
	// 0 = derive from MaxTxPerBlock at startup (2×, or -1 when unlimited). -1 = disable.
	MempoolTxCacheSize int `mapstructure:"tx-cache-size"`
	// Per-entry raw payload byte cap. Txs larger than this are decoded but
	// not cached, bounding heap impact. Should not exceed mempool.max_tx_bytes.
	MempoolTxCacheMaxTxBytes int `mapstructure:"tx-cache-max-tx-bytes"`
	// MempoolGossipTTL is the re-gossip suppression window for mempool.type=app:
	// a tx reaped for gossip is not re-broadcast until this elapses. Bounds the
	// AppReactor's per-tick re-broadcast of the whole pool. <=0 uses the default.
	MempoolGossipTTL time.Duration `mapstructure:"mempool-gossip-ttl"`
	// MaxTxPerBlock is the shared budget used as both the gossip-reap cap
	// (txs per 500ms tick ≈ one block) and the recheck-batch cap (candidates per
	// Commit cycle ≈ one block of senders). <=0 uses the default.
	MaxTxPerBlock int `mapstructure:"mempool-txs-per-block"`
	// MempoolTxTTL evicts mempool.type=app txs older than DefaultMempoolTTLNumBlocks
	// blocks (by arrival height), draining proposal-skipped txs whose sender never
	// commits. Set to false to disable.
	MempoolTxTTL bool `mapstructure:"mempool-tx-ttl"`
	// RPCPendingTxCache caches the PendingTxs() pool-scan result for
	// DefaultMempoolPendingCacheTTL, so repeated RPC reads don't each re-walk the
	// pool. Set to false to disable.
	RPCPendingTxCache bool `mapstructure:"rpc-pending-tx-cache"`
}

// Defaults live here (not app/) because app/ imports this package and both
// DefaultCronosConfig() and app's New() need them.
const (
	DefaultTxCacheMaxTxBytes = 65536
	// DefaultMempoolGossipTTL re-gossips a resident tx at most ~once per window;
	// far above CometBFT's 500ms ReapInterval so steady state suppresses re-reap.
	DefaultMempoolGossipTTL = 15 * time.Second
	// DefaultMempoolTxsPerBlock is one block's tx budget (~2900 = cronos mainnet
	// empirical block size). Governs both the gossip-reap cap (one tick ≈ one
	// block interval) and the recheck-batch cap (one commit ≈ one block of senders).
	DefaultMempoolTxsPerBlock = 2900
	// DefaultTxCacheSize is the tx encode/decode cache capacity when tx-cache-size
	// is unset (0): two full blocks so the cache survives one proposal + one gossip
	// reap cycle without eviction pressure.
	DefaultTxCacheSize = 2 * DefaultMempoolTxsPerBlock
	// DefaultMempoolTTLNumBlocks evicts mempool.type=app txs older than this many
	// blocks by arrival height, draining proposal-skipped txs that never commit.
	DefaultMempoolTTLNumBlocks = 120

	DefaultMempoolPendingCacheTTL = 100 * time.Millisecond
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
		MempoolTxCacheSize:         0, // 0 = derive: 2×MaxTxPerBlock at startup, -1 when unlimited
		MempoolTxCacheMaxTxBytes:   DefaultTxCacheMaxTxBytes,
		MempoolGossipTTL:           DefaultMempoolGossipTTL,
		MaxTxPerBlock:              DefaultMempoolTxsPerBlock,
		MempoolTxTTL:               true,
		RPCPendingTxCache:          true,
	}
}

func DefaultRocksDBConfig() RocksDBConfig {
	return RocksDBConfig{
		NodeType: "",
	}
}

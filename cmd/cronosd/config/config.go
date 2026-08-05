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
	// Capacity of the tx encode/decode cache. Default 0 (derive from 2×MaxTxPerBlock).
	// -1 disables the cache.
	MempoolTxCacheSize int `mapstructure:"mempool-tx-cache-size"`
	// Per-entry byte cap for the tx encode/decode cache. Default 65536 (64 KiB).
	MempoolTxCacheMaxTxBytes int `mapstructure:"mempool-tx-cache-max-tx-bytes"`
	// MempoolGossipTTL is the re-gossip suppression window for mempool.type=app:
	// a tx reaped for gossip is not re-broadcast until this elapses. Bounds the
	// AppReactor's per-tick re-broadcast of the whole pool. <=0 uses the default.
	MempoolGossipTTL time.Duration `mapstructure:"mempool-gossip-ttl"`
	// Tx budget per block for mempool.type=app. Default 2900. 0 = unlimited.
	MaxTxPerBlock int `mapstructure:"mempool-txs-per-block"`
	// Evicts mempool.type=app txs older than 120 blocks by arrival height.
	// Default true. false disables.
	MempoolTxTTL bool `mapstructure:"mempool-tx-ttl"`
	// Caches the PendingTxs() pool-scan result, invalidated on tx admission
	// and block completion. Default true. false always walks the pool.
	RPCPendingTxCache bool `mapstructure:"rpc-pending-tx-cache"`
}

const (
	DefaultTxCacheMaxTxBytes = 65536
	// DefaultMempoolGossipTTL is the re-gossip suppression window used when MempoolGossipTTL is unset.
	DefaultMempoolGossipTTL = 15 * time.Second
	// DefaultMempoolTxsPerBlock is the tx-per-block budget used when MaxTxPerBlock is unset.
	DefaultMempoolTxsPerBlock = 2900
	// DefaultTxCacheSize is the tx encode/decode cache capacity used when MempoolTxCacheSize is unset (0).
	DefaultTxCacheSize = 2 * DefaultMempoolTxsPerBlock
	// DefaultMempoolTTLNumBlocks evicts mempool.type=app txs older than this many
	// blocks by arrival height, draining proposal-skipped txs that never commit.
	DefaultMempoolTTLNumBlocks = 120
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

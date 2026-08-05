package config

// DefaultCronosConfigTemplate defines the configuration template for cronos configuration
const DefaultCronosConfigTemplate = `
###############################################################################
###                             Cronos Configuration                       ###
###############################################################################

[cronos]

# Set to true to disable tx replacement.
disable-tx-replacement = {{ .Cronos.DisableTxReplacement }}

# Set to true to disable optimistic execution (not recommended on validator nodes).
disable-optimistic-execution = {{ .Cronos.DisableOptimisticExecution }}

# Capacity of the sharded LRU tx encode/decode cache.
# 0 = derive from mempool-txs-per-block at startup (2×, default 5800). -1 = disable entirely.
mempool-tx-cache-size = {{ .Cronos.MempoolTxCacheSize }}

# Per-entry raw payload byte cap for the tx encode/decode cache. Default 65536 (64 KiB).
mempool-tx-cache-max-tx-bytes = {{ .Cronos.MempoolTxCacheMaxTxBytes }}

# Re-gossip suppression window for mempool.type=app. A tx reaped for gossip is
# not re-broadcast until this elapses, stopping the AppReactor from flooding the
# whole pool to peers every reap_interval (~500ms). Default "15s".
mempool-gossip-ttl = "{{ .Cronos.MempoolGossipTTL }}"

# Tx budget per block for mempool.type=app. Default 2900. 0 = unlimited.
mempool-txs-per-block = {{ .Cronos.MaxTxPerBlock }}

# Evicts mempool.type=app txs older than 120 blocks by arrival height.
# Default true. false disables.
mempool-tx-ttl = {{ .Cronos.MempoolTxTTL }}

# Caches the PendingTxs() pool-scan result, invalidated on tx admission and
# block completion. Default true. false always walks the pool.
rpc-pending-tx-cache = {{ .Cronos.RPCPendingTxCache }}
`

// DefaultRocksDBConfigTemplate defines the configuration template for rocksdb configuration
const DefaultRocksDBConfigTemplate = `
###############################################################################
###                             RocksDB Configuration                       ###
###############################################################################

[rocksdb]

# Defines the tuning profile for RocksDB based on the node's primary workload.
# This is an experimental feature for performance optimization.
# Valid values:
# - ""          (default): standard configuration, safe for all nodes.
# - "validator" : optimizes for lowest latency point-lookups (state reads) during block execution.
# - "rpc"       : optimizes for highly concurrent read workloads (eth_calls, state queries) with lock-free caches.
# - "archive"   : optimizes for massive historical data scanning and sequential reads (eth_getLogs).
node_type = "{{ .RocksDB.NodeType }}"
`

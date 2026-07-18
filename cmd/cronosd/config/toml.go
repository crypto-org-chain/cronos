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
tx-cache-size = {{ .Cronos.TxCacheSize }}

# Per-entry raw payload byte cap for the tx encode/decode cache. Txs larger than this
# are decoded normally but not cached, bounding heap use against large txs.
# Worst-case raw-byte footprint is roughly tx-cache-size * this value.
# Should not exceed mempool.max_tx_bytes. Default 65536 (64 KiB) covers >p99 of
# EVM tx sizes.
tx-cache-max-tx-bytes = {{ .Cronos.TxCacheMaxTxBytes }}

# Re-gossip suppression window for mempool.type=app. A tx reaped for gossip is
# not re-broadcast until this elapses, stopping the AppReactor from flooding the
# whole pool to peers every reap_interval (~500ms). Default "15s".
mempool-gossip-ttl = "{{ .Cronos.MempoolGossipTTL }}"

# Tx budget per block for mempool.type=app (cronos mainnet empirical: ~2900).
# Controls both the gossip-reap cap (txs per 500ms reap tick ≈ one block) and
# the recheck-batch cap (candidates re-validated per Commit cycle ≈ one block of
# senders). 0 = unlimited. Default 2900.
mempool-txs-per-block = {{ .Cronos.MempoolTxsPerBlock }}

# Evict mempool.type=app txs older than this many blocks (by arrival height),
# independent of per-tx TimeoutHeight (EVM txs have TimeoutHeight 0 = never
# expire). Drains txs the proposal keeps skipping (baseFee gate, blocklist) whose
# sender never commits. 0 = disabled. Default 120.
mempool-ttl-num-blocks = {{ .Cronos.MempoolTTLNumBlocks }}

# Capacity of the hash-keyed ecrecover sender cache consulted in VerifyEthSig.
# 0 = default (100000). Negative disables entirely.
mempool-tx-sender-cache-size = {{ .Cronos.MempoolTxSenderCacheSize }}
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

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

# Capacity of the sharded LRU tx-decode cache. Set to 0 to disable the cache
# entirely (raw decoder used). Default 10000.
tx-decode-cache-size = {{ .Cronos.TxDecodeCacheSize }}

# Per-entry raw payload byte cap for the tx-decode cache. Txs larger than this
# are decoded normally but not cached, bounding heap use against large txs.
# Worst-case raw-byte footprint is roughly tx-decode-cache-size * this value.
# Should not exceed mempool.max-tx-bytes. Default 65536 (64 KiB) covers >p99 of
# EVM tx sizes.
tx-decode-cache-max-tx-bytes = {{ .Cronos.TxDecodeCacheMaxTxBytes }}

# Re-gossip suppression window for mempool.type=app. A tx reaped for gossip is
# not re-broadcast until this elapses, stopping the AppReactor from flooding the
# whole pool to peers every reap_interval (~500ms). Default "15s".
mempool-gossip-ttl = "{{ .Cronos.MempoolGossipTTL }}"

# Max txs returned per gossip reap for mempool.type=app, spreading a large pool
# across reap ticks instead of one libp2p batch. Size it >= target tps *
# mempool.reap_interval (seconds), e.g. 10000 tps * 0.5s = 5000, or new txs
# backlog each tick. 0 disables the count cap (only mempool.reap_max_bytes /
# reap_max_gas apply). Default 5000.
mempool-gossip-max-per-reap = {{ .Cronos.MempoolGossipMaxPerReap }}

# Max candidate txs re-validated per Commit cycle for mempool.type=app. Bounds
# RunTx(ReCheck) time under deep pools; the O(pool) scan that selects candidates
# runs outside the admission mutex, so this only caps the ante re-runs. Size it to
# one block's tx count or stale (balance-drained / replaced) txs linger and get
# re-proposed as failures. 0 = unlimited. Default 5000.
mempool-recheck-batch-size = {{ .Cronos.MempoolRecheckBatchSize }}
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

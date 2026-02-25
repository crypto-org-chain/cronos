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
`

// DefaultRocksDBConfigTemplate defines the configuration template for rocksdb configuration
const DefaultRocksDBConfigTemplate = `
###############################################################################
###                             RocksDB Configuration                       ###
###############################################################################

[rocksdb]

# Defines the tuning profile for RocksDB based on the node's primary workload.
# Valid values:
# - ""          (default): standard configuration, safe for all nodes.
# - "validator" : optimizes for lowest latency point-lookups (state reads) during block execution.
# - "rpc"       : optimizes for highly concurrent read workloads (eth_calls, state queries) with lock-free caches.
# - "archive"   : optimizes for massive historical data scanning and sequential reads (eth_getLogs).
node_type = "{{ .RocksDB.NodeType }}"
`

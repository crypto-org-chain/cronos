# Cronos Attestation Layer Relayer

The Cronos Attestation Layer Relayer is a high-performance service that bridges the Cronos EVM chain with a Cosmos SDK-based attestation layer. It forwards block data for attestation and relays finality information back to the Cronos chain, with an embedded RPC API for monitoring and status queries.

## Overview

The relayer serves as the critical infrastructure component connecting Cronos (Layer 2) with an attestation chain (Layer 1), enabling:

- ✅ **Block Attestation** - Forward Cronos blocks to attestation layer for verification
- ✅ **Finality Relay** - Monitor and relay finality confirmations back to Cronos
- ✅ **Gap Detection** - Automatically detect and fill missing blocks
- ✅ **Crash Recovery** - Checkpoint-based state recovery
- ✅ **Batch Processing** - Efficient batch block forwarding
- ✅ **RPC Monitoring** - Embedded HTTP API for status and metrics

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    RELAYER SERVICE                          │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Source Chain Monitor (Cronos)                       │  │
│  │  - WebSocket subscription to new blocks              │  │
│  │  - Gap detection and filling                         │  │
│  └──────────────────────────────────────────────────────┘  │
│                           │                                 │
│                           ▼                                 │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Block Forwarder                                     │  │
│  │  - Batch block attestations                          │  │
│  │  - Cryptographic signatures                          │  │
│  │  - Transaction broadcasting (sync/async)             │  │
│  └──────────────────────────────────────────────────────┘  │
│                           │                                 │
│                           ▼                                 │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Attestation Chain (Layer 1)                         │  │
│  │  - Verify and store block attestations               │  │
│  │  - Emit finality events                              │  │
│  │  - Act as Data Availability layer                    │  │
│  └──────────────────────────────────────────────────────┘  │
│                           │                                 │
│                           ▼                                 │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Finality Monitor                                    │  │
│  │  - Subscribe to attestation events                   │  │
│  │  - Track pending attestations                        │  │
│  │  - Store finality information                        │  │
│  └──────────────────────────────────────────────────────┘  │
│                           │                                 │
│                           ▼                                 │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Finality Store (LevelDB/RocksDB/Memory)             │  │
│  │  - Persistent finality data                          │  │
│  │  - Query interface                                   │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Checkpoint Manager                                  │  │
│  │  - Auto-save recovery state                          │  │
│  │  - Pending attestations tracking                     │  │
│  │  - Last finality height                              │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  RPC Server (Optional, Embedded)                     │  │
│  │  - GET /health       - Health check                  │  │
│  │  - GET /status       - Relayer status                │  │
│  │  - GET /finality/... - Block finality info           │  │
│  │  - GET /checkpoint   - Checkpoint state              │  │
│  │  - GET /pending      - Pending attestations          │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Key Features

### Block Forwarding

- **Batch Processing**: Forward multiple blocks in a single transaction for efficiency
- **Signature Verification**: Cryptographically sign batch attestations
- **Gap Detection**: Automatically detect and query missing blocks
- **Broadcast Modes**: Support both sync (immediate finality) and async modes
- **Full Block Data**: Include transactions, evidence, and last commit for DA layer

### Finality Monitoring

- **Event Subscription**: Real-time monitoring of attestation events
- **Pending Tracking**: Track attestations until confirmed
- **Finality Storage**: Persist finality information for queries
- **Checkpoint Recovery**: Resume from last known state after crashes

### RPC API (Embedded)

- **Zero Configuration**: Automatically starts with relayer
- **Health Monitoring**: `/health` endpoint for load balancers
- **Status Queries**: Real-time relayer status and metrics
- **Finality Lookup**: Query finality status for any block
- **CORS Support**: Optional CORS for web dashboards

### Reliability

- **Crash Recovery**: Checkpoint-based state recovery
- **Gap Filling**: Automatic detection and filling of missing blocks
- **Retry Logic**: Configurable retry with exponential backoff
- **Graceful Shutdown**: Proper cleanup on termination
- **Error Isolation**: RPC failures don't affect core relayer

## Quick Start

### Prerequisites

- Go 1.21 or higher
- Access to Cronos RPC endpoint
- Access to Attestation Chain RPC endpoint
- Relayer account with sufficient funds

### Installation

```bash
# Clone the repository
git clone https://github.com/crypto-org-chain/cronos.git
cd cronos

# Build the relayer
make build-relayer

# Or build manually
go build -o relayerd ./cmd/relayerd
```

### Configuration

Create a `config.json` file:

```json
{
  "source_chain_id": "cronos_777-1",
  "source_rpc": "http://localhost:26657",
  "source_grpc": "localhost:9090",
  
  "attestation_chain_id": "attestation-1",
  "attestation_rpc": "http://localhost:36657",
  "attestation_grpc": "localhost:19090",
  
  "relayer_mnemonic": "word1 word2 ... word24",
  "relayer_address": "cronos1xxx...",
  
  "block_batch_size": 10,
  "max_retries": 3,
  "retry_delay": "5s",
  
  "gas_adjustment": 1.5,
  "gas_prices": "0.025stake",
  "broadcast_mode": "async",
  
  "finality_store_type": "leveldb",
  "finality_store_path": "./data/finality",
  "checkpoint_path": "./data/checkpoint.json",
  
  "rpc_enabled": true,
  "rpc_config": {
    "listen_addr": "0.0.0.0:8080",
    "read_timeout": "15s",
    "write_timeout": "15s",
    "enable_cors": true
  }
}
```

### Running the Relayer

```bash
# Start the relayer
./relayerd --config config.json

# With custom log level
./relayerd --config config.json --log-level debug

# Check status (if RPC enabled)
curl http://localhost:8080/health
curl http://localhost:8080/status | jq
```

## Configuration Reference

### Chain Configuration

| Field | Type | Description |
|-------|------|-------------|
| `source_chain_id` | string | Cronos chain ID |
| `source_rpc` | string | Cronos RPC endpoint |
| `source_grpc` | string | Cronos gRPC endpoint |
| `attestation_chain_id` | string | Attestation chain ID |
| `attestation_rpc` | string | Attestation RPC endpoint |
| `attestation_grpc` | string | Attestation gRPC endpoint |

### Relayer Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `relayer_mnemonic` | string | - | BIP39 mnemonic phrase |
| `relayer_address` | string | - | Relayer account address |
| `block_batch_size` | uint | 10 | Blocks per batch |
| `max_retries` | uint | 3 | Max retry attempts |
| `retry_delay` | duration | 5s | Delay between retries |

### Gas Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `gas_adjustment` | float64 | 1.5 | Gas estimation multiplier |
| `gas_prices` | string | - | Gas prices (e.g., "0.025stake") |
| `broadcast_mode` | string | async | "sync" or "async" |

### Storage Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `finality_store_type` | string | memory | "memory", "leveldb", "rocksdb" |
| `finality_store_path` | string | - | Path to store data |
| `checkpoint_path` | string | - | Path to checkpoint file |

### RPC Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `rpc_enabled` | bool | false | Enable RPC server |
| `rpc_config.listen_addr` | string | 0.0.0.0:8080 | Listen address |
| `rpc_config.read_timeout` | duration | 15s | HTTP read timeout |
| `rpc_config.write_timeout` | duration | 15s | HTTP write timeout |
| `rpc_config.enable_cors` | bool | false | Enable CORS headers |

## RPC API

When `rpc_enabled: true`, the relayer exposes an HTTP API:

### Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/status` | GET | Relayer status and metrics |
| `/finality/{chain_id}/{height}` | GET | Block finality information |
| `/checkpoint` | GET | Checkpoint state |
| `/pending` | GET | Pending attestations count |

### Examples

```bash
# Health check
curl http://localhost:8080/health

# Get relayer status
curl http://localhost:8080/status | jq

# Check block finality
curl http://localhost:8080/finality/cronos_777-1/1000 | jq

# Get checkpoint state
curl http://localhost:8080/checkpoint | jq

# Get pending attestations
curl http://localhost:8080/pending | jq
```

For complete API documentation, see [RPC_API.md](./RPC_API.md).

## Monitoring & Operations

### Health Checks

The relayer provides multiple ways to monitor health:

```bash
# RPC health endpoint
curl http://localhost:8080/health

# Check process
ps aux | grep relayerd

# Check logs
journalctl -u cronos-relayer -f
```

### Metrics

Query the status endpoint for real-time metrics:

```json
{
  "status": {
    "running": true,
    "source_chain_id": "cronos_777-1",
    "attestation_chain_id": "attestation-1",
    "last_block_forwarded": 1000,
    "last_finality_received": 950,
    "finalized_blocks_count": 900,
    "updated_at": "2024-11-05T17:30:00Z"
  }
}
```

### Prometheus Integration

Scrape the status endpoint for metrics:

```yaml
scrape_configs:
  - job_name: 'cronos-relayer'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/status'
    scrape_interval: 15s
```

### Systemd Service

```ini
[Unit]
Description=Cronos Relayer
After=network.target

[Service]
Type=simple
User=relayer
ExecStart=/usr/local/bin/relayerd --config /etc/relayer/config.json
Restart=on-failure
RestartSec=5s

# Health check
ExecStartPost=/bin/sh -c 'sleep 5 && curl -f http://localhost:8080/health || exit 1'

[Install]
WantedBy=multi-user.target
```

### Docker Deployment

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o relayerd ./cmd/relayerd

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/relayerd /usr/local/bin/
COPY config.json /etc/relayer/config.json

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

EXPOSE 8080
CMD ["relayerd", "--config", "/etc/relayer/config.json"]
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cronos-relayer
spec:
  replicas: 1
  selector:
    matchLabels:
      app: cronos-relayer
  template:
    metadata:
      labels:
        app: cronos-relayer
    spec:
      containers:
      - name: relayer
        image: cronos-relayer:latest
        ports:
        - containerPort: 8080
          name: rpc-api
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /status
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

## Development

### Building from Source

```bash
# Build the relayer
make build-relayer

# Or with specific flags
go build -mod=vendor -o relayerd ./cmd/relayerd

# Build with RocksDB support (requires rocksdb)
go build -tags rocksdb -o relayerd ./cmd/relayerd
```

## Troubleshooting

### Relayer Won't Start

```bash
# Check configuration
cat config.json | jq

# Verify connectivity
curl http://localhost:26657/status
curl http://localhost:36657/status

# Check account balance
cronosd query bank balances <relayer-address>
```

### Blocks Not Forwarding

```bash
# Check relayer status
curl http://localhost:8080/status | jq

# Check logs for errors
journalctl -u cronos-relayer -n 100

# Verify gas prices
cronosd query bank balances <relayer-address>
```

### RPC Server Not Responding

```bash
# Check if RPC is enabled
grep rpc_enabled config.json

# Check if port is in use
lsof -i :8080

# Check firewall
sudo iptables -L -n | grep 8080

# Test locally
curl http://127.0.0.1:8080/health
```

### High Memory Usage

```bash
# Check finality store size
du -sh ./data/finality

# Consider using different store type
# Memory: Fast but limited
# LevelDB: Good balance
# RocksDB: Best for high load

# Restart with different store type in config.json
```

### Gap in Block Forwarding

The relayer automatically detects and fills gaps, but you can verify:

```bash
# Check last forwarded block
curl http://localhost:8080/status | jq '.status.last_block_forwarded'

# Check checkpoint state
curl http://localhost:8080/checkpoint | jq

# Check pending attestations
curl http://localhost:8080/pending | jq
```

## Performance Tuning

### Batch Size

```json
{
  "block_batch_size": 10  // Increase for higher throughput
}
```

Larger batches = fewer transactions but higher gas per tx.

### Broadcast Mode

```json
{
  "broadcast_mode": "async"  // Faster, finality via events
  // or
  "broadcast_mode": "sync"   // Slower, immediate finality
}
```

### Store Backend

- **Memory**: Fastest, limited by RAM
- **LevelDB**: Good balance, moderate load
- **RocksDB**: Best for high throughput, requires build flag

### Gas Configuration

```json
{
  "gas_adjustment": 1.5,      // Increase if transactions fail
  "gas_prices": "0.025stake"  // Adjust based on network
}
```

## Documentation

- [RPC_API.md](./RPC_API.md) - Complete RPC API reference
- [RPC_INTEGRATION.md](./RPC_INTEGRATION.md) - Integration guide
- [config.example.json](./config.example.json) - Configuration example

## License

Apache 2.0 - See [LICENSE](../LICENSE) for details.

---

## Summary

**Get started in 3 steps:**
1. Configure `config.json`
2. Run `relayerd --config config.json`
3. Monitor via `curl http://localhost:8080/status`

For detailed information, see the documentation files in this directory.

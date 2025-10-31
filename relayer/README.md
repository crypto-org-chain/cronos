# Cronos Attestation Layer Relayer

A relayer system connecting Cronos EVM network with a Cosmos SDK-based attestation Layer 1 chain using ABCI data and Cosmos SDK messages.

## Features

### 1. ABCI Block Forwarding
Forwards `RequestFinalizeBlock` and `ResponseFinalizeBlock` from Cronos to the attestation layer chain.

```go
type ABCIBlockData struct {
    ChainID               string
    BlockHeight           uint64
    RequestFinalizeBlock  *abci.RequestFinalizeBlock
    ResponseFinalizeBlock *abci.ResponseFinalizeBlock
    Timestamp             int64
    Signature             []byte
}
```

**Flow**: Cronos ABCI → Monitor → ForwardBlock() → `MsgSubmitBlockAttestation` → Attestation Chain → `EventBlockAttested`

### 2. Finality Monitoring & Storage
Monitors `EventBlockFinalized` events from attestation chain and stores finality information locally.

```go
type FinalityInfoCosmos struct {
    AttestationID     uint64
    ChainID           string
    BlockHeight       uint64
    Finalized         bool
    FinalizedAt       int64
    FinalityProof     []byte
    ValidatorCount    uint32
}
```

**Storage**: LevelDB/RocksDB backend with in-memory caching for fast lookups.

### 3. Forced Transaction Handling
Monitors and executes forced transactions with priority-based execution.

```go
type ForcedTxCosmos struct {
    ForcedTxID      uint64
    TargetChainID   string
    TargetChainType string  // "evm" or "cosmos"
    Priority        uint32  // 0-255
    TxType          ForcedTxType
    TransactionData []byte
    Deadline        uint64
}
```

**Flow**: `MsgSubmitForcedTransaction` → Attestation Chain State → `EventForcedTxSubmitted` → Monitor → Execute on Target Chain

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│         Attestation Layer (Cosmos SDK Chain)              │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │ Attestation │  │  Finality    │  │  Forced TX   │   │
│  │   Module    │  │  Events      │  │  Queue       │   │
│  └──────▲──────┘  └──────┬───────┘  └──────┬───────┘   │
│         │ Submit          │ Emit            │ Store      │
└─────────┼─────────────────┼─────────────────┼───────────┘
          │                 │                 │
    ┌─────┴─────────────────┴─────────────────┴──────┐
    │            Cronos Relayer Service               │
    │  ┌──────────┐  ┌────────────┐  ┌────────────┐ │
    │  │  ABCI    │  │ Finality   │  │  Forced    │ │
    │  │Forwarder │  │  Monitor   │  │TX Executor │ │
    │  └────┬─────┘  └─────▲──────┘  └─────▲──────┘ │
    └───────┼──────────────┼────────────────┼────────┘
            │              │                │
┌───────────┴──────────────┴────────────────┴────────┐
│            Cronos Chain (Source EVM)                │
│  ┌──────────────┐           ┌──────────────┐       │
│  │ ABCI Data    │           │ EVM Execute  │       │
│  └──────────────┘           └──────────────┘       │
└─────────────────────────────────────────────────────┘
```

## Configuration

```json
{
  "source_chain_id": "cronos_25-1",
  "source_rpc": "http://localhost:26657",
  "source_grpc": "localhost:9090",
  
  "attestation_chain_id": "attestation-1",
  "attestation_rpc": "http://localhost:36657",
  "attestation_grpc": "localhost:19090",
  
  "relayer_mnemonic": "word1 word2 ... word24",
  "relayer_address": "cronos1...",
  
  "block_batch_size": 10,
  "max_retries": 3,
  "retry_delay": "5s",
  
  "block_poll_interval": "2s",
  "finality_poll_interval": "5s",
  "forced_tx_poll_interval": "3s",
  
  "gas_adjustment": 1.5,
  "gas_prices": "0.025stake",
  
  "finality_store_type": "leveldb",
  "finality_store_path": "./data/finality"
}
```

## Components

### CosmosChainMonitor
Monitors Cosmos SDK chains and extracts ABCI data.

```go
monitor, _ := NewCosmosChainMonitor(rpcEndpoint, config, logger, chainID, chainName)
monitor.Start(ctx)
blockCh, _ := monitor.SubscribeNewBlocks(ctx)
```

### BlockForwarder
Forwards ABCI block data to attestation chain via Cosmos SDK messages.

```go
forwarder, _ := NewBlockForwarderCosmos(clientCtx, config, logger)
attestationID, _ := forwarder.ForwardBlock(ctx, blockData)
```

### FinalityMonitor & Store
Monitors finality events and persists state locally.

```go
store, _ := NewFinalityStoreFromConfig(config, logger)
monitor, _ := NewFinalityMonitorCosmos(client, config, logger, store)
finalityCh, _ := monitor.SubscribeFinality(ctx)
```

### ForcedTxMonitor & Executor
Monitors and executes forced transactions with priority.

```go
monitor, _ := NewForcedTxMonitorCosmos(client, clientCtx, config, logger)
executor, _ := NewForcedTxExecutorCosmos(clientCtx, config, logger)
forcedTxCh, _ := monitor.SubscribeForcedTx(ctx)
executor.ExecuteForcedTx(ctx, tx)
```

## Protobuf Messages

### Block Attestation
```protobuf
message MsgSubmitBlockAttestation {
    string relayer = 1;
    string chain_id = 2;
    uint64 block_height = 3;
    cometbft.abci.RequestFinalizeBlock request_finalize_block = 4;
    cometbft.abci.ResponseFinalizeBlock response_finalize_block = 5;
    int64 timestamp = 6;
    bytes signature = 7;
}
```

### Forced Transaction
```protobuf
message MsgSubmitForcedTransaction {
    string submitter = 1;
    string target_chain_id = 2;
    string target_chain_type = 3;  // "evm" or "cosmos"
    uint32 priority = 4;
    ForcedTxType tx_type = 5;
    bytes transaction_data = 6;
    uint64 deadline = 7;
}
```

### Events
```protobuf
message EventBlockFinalized {
    string chain_id = 1;
    uint64 block_height = 2;
    int64 finalized_at = 3;
    uint32 validator_count = 4;
    bytes finality_signature = 5;
}

message EventForcedTxSubmitted {
    uint64 forced_tx_id = 1;
    string target_chain_id = 3;
    uint32 priority = 5;
    ForcedTxType tx_type = 6;
}
```

## Usage

### Using RelayerService (Recommended)

The `RelayerService` provides a unified interface for all relayer functionality:

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    
    "github.com/crypto-org-chain/cronos/v2/relayer"
    "cosmossdk.io/log"
)

func main() {
    // Load configuration
    config := relayer.LoadCosmosRelayerConfig("config.json")
    logger := log.NewLogger(os.Stdout)
    
    // Setup client contexts (see example_main.go for full implementation)
    sourceClientCtx := createClientContext(config.SourceRPC, config.SourceGRPC, config.RelayerMnemonic)
    attestationClientCtx := createClientContext(config.AttestationRPC, config.AttestationGRPC, config.RelayerMnemonic)
    
    // Create relayer service (all components managed internally)
    service, err := relayer.NewRelayerService(
        config,
        logger,
        sourceClientCtx,
        attestationClientCtx,
    )
    if err != nil {
        logger.Error("Failed to create relayer service", "error", err)
        os.Exit(1)
    }
    
    // Start relayer (starts all workers automatically)
    ctx := context.Background()
    if err := service.Start(ctx); err != nil {
        logger.Error("Failed to start relayer", "error", err)
        os.Exit(1)
    }
    
    logger.Info("Relayer started successfully")
    
    // Monitor status
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        for range ticker.C {
            status := service.GetStatus()
            logger.Info("Status",
                "last_block", status.LastBlockForwarded,
                "last_finality", status.LastFinalityReceived,
                "finalized_count", status.FinalizedBlocksCount,
            )
        }
    }()
    
    // Wait for interrupt
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
    
    // Stop relayer
    if err := service.Stop(); err != nil {
        logger.Error("Failed to stop relayer", "error", err)
    }
}
```

See `example_main.go` for a complete working example.

## Protobuf Generation

Generate Go types from protobuf definitions:

```bash
cd cronos
make proto-gen
# or
./scripts/protocgen.sh
```

This generates Go types from `proto/attestation/v1/*.proto` files into `relayer/types/`.

## Next Steps

1. **Implement Attestation Module**: Create Cosmos SDK module on attestation chain
   - Message handlers (`MsgSubmitBlockAttestation`, `MsgSubmitForcedTransaction`)
   - Finality logic (validator signatures, timeouts)
   - Forced TX queue management
   - Event emission

2. **Integration**: Replace placeholder code with generated protobuf types

3. **Testing**: Unit and integration tests with test chains

4. **Deployment**: Deploy attestation chain and configure relayer

## Performance

| Metric | Target |
|--------|--------|
| Block attestation latency | < 3s |
| Finality detection | < 5s |
| Forced TX execution | < 30s |
| Throughput | 50+ blocks/min |
| Finality lookups | < 1ms (cached) |

## Security

- **Signature Verification**: Relayer signs block attestations
- **Forced TX Validation**: Deadline and authority checks
- **Event Verification**: Parse and validate all events
- **Gas Management**: Monitor relayer balance

## File Structure

```
relayer/
├── relayer_service.go           # Main service (all-in-one)
├── types_cosmos.go              # Types and interfaces
├── cosmos_monitor.go            # Chain monitoring
├── cosmos_block_forwarder.go    # Block forwarding
├── cosmos_finality_monitor.go   # Finality monitoring
├── cosmos_forced_tx.go          # Forced TX handling
├── finality_store.go            # Data persistence
├── types/                       # Generated protobuf types
│   ├── tx.pb.go
│   ├── events.pb.go
│   └── query.pb.go
├── config.cosmos.example.json   # Configuration
├── example_main.go              # Usage example
└── README.md                    # This file

proto/attestation/v1/
├── tx.proto                     # Message definitions
├── events.proto                 # Event definitions
└── query.proto                  # Query definitions
```


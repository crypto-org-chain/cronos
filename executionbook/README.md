# ExecutionBook Package

The `executionbook` package implements a priority transaction system with preconfirmation capabilities for the Cronos blockchain. It allows validators to provide early confirmations for high-priority transactions before they are included in a block.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Components](#components)
- [Configuration](#configuration)
- [Usage](#usage)
- [API Reference](#api-reference)
- [Testing](#testing)
- [Implementation Details](#implementation-details)

---

## Overview

The preconfer system provides:

1. **Priority Transaction Handling**: Transactions with `PRIORITY:` prefix in memo receive priority boost
2. **Preconfirmations**: Early, non-binding confirmations from validators
3. **Whitelist Support**: Optional access control for priority boosting
4. **gRPC API**: Service for submitting and querying priority transactions
5. **CLI Commands**: Command-line interface for preconfer operations

### Key Features

- **Priority Boost**: Transactions marked with priority receive a boost of `1,000,000,000` to their priority
- **Preconfirmation Timeout**: Configurable timeout (default 30s) for preconfirmation expiry
- **Position Tracking**: Real-time tracking of transaction position in mempool
- **Status Management**: Track transaction lifecycle from pending to included
- **Ethereum Support**: Full support for Ethereum-style transactions

---

## Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Application Layer                        │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              PriorityTxService                        │   │
│  │  - Submit priority transactions                       │   │
│  │  - Create preconfirmations                            │   │
│  │  - Track transaction status                           │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                      gRPC Layer                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │         PriorityTxGRPCServer                          │   │
│  │  - SubmitPriorityTx                                   │   │
│  │  - GetPriorityTxStatus                                │   │
│  │  - GetMempoolStats                                    │   │
│  │  - ListPriorityTxs                                    │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                   Mempool Layer                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │         Preconfer ExecutionBook                        │   │
│  │  - Priority boost application                         │   │
│  │  - Whitelist enforcement                              │   │
│  │  - Transaction selection                              │   │
│  └──────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────┐   │
│  │         PriorityTxSelector                            │   │
│  │  - Priority transaction ordering                      │   │
│  │  - Fast proposal selection                            │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Transaction Lifecycle

```
1. Submit
   ↓
2. Validate Priority Level (must be 1)
   ↓
3. Check Whitelist (if enabled)
   ↓
4. Apply Priority Boost (+1,000,000,000)
   ↓
5. Create Preconfirmation
   ↓
6. Insert into Mempool
   ↓
7. Track Status
   ↓
8. Include in Block
```

---

## Components

### 1. Priority Transaction Service (`priority_tx_service.go`)

Core service for managing priority transactions and preconfirmations.

**Key Types:**

```go
type PriorityTxService struct {
    app               *baseapp.BaseApp
    mempool           mempool.Mempool
    txDecoder         sdk.TxDecoder
    logger            log.Logger
    validatorAddress  string
    preconfirmTimeout time.Duration
}

type TxStatusType int
const (
    TxStatusUnknown TxStatusType = iota
    TxStatusPending
    TxStatusPreconfirmed
    TxStatusIncluded
    TxStatusRejected
    TxStatusExpired
)
```

**Key Methods:**

- `SubmitPriorityTx(ctx, txBytes, priorityLevel)` - Submit priority transaction
- `GetTxStatus(txHash)` - Get transaction status
- `GetMempoolStats()` - Get mempool statistics
- `ListPriorityTxs(limit)` - List priority transactions

### 2. Preconfer ExecutionBook (`execution_book.go`)

Enhanced mempool wrapper that applies priority boosting.

**Key Features:**

- Wraps any standard Cosmos SDK mempool
- Applies configurable priority boost (default: 1,000,000,000)
- Enforces whitelist for priority boosting (optional)
- Preserves base mempool functionality

**Configuration:**

```go
type ExecutionBookConfig struct {
    BaseMempool        mempool.Mempool
    TxDecoder          sdk.TxDecoder
    PriorityBoost      int64
    Logger             log.Logger
    WhitelistAddresses []string
    SignerExtractor    mempool.SignerExtractionAdapter
}
```

### 3. Priority Transaction Selector (`priority_tx_selector.go`)

Custom proposal handler for priority transaction ordering.

**Capabilities:**

- Ensures priority transactions are selected first
- Handles both standard and fast proposal selection
- Maintains gas limits and block constraints

### 4. gRPC Server (`priority_tx_grpc.go`)

gRPC service implementation for external access.

**Endpoints:**

- `SubmitPriorityTx` - Submit a priority transaction
- `GetPriorityTxStatus` - Query transaction status
- `GetMempoolStats` - Get mempool statistics
- `ListPriorityTxs` - List pending priority transactions

### 5. Whitelist Management (`whitelist_grpc.go`)

Manage whitelist for priority boosting access control.

**Features:**

- Add/remove addresses from whitelist
- Set entire whitelist
- Query whitelist status
- Query if address is whitelisted

### 6. CLI Commands (`cli.go`)

Command-line interface for preconfer operations.

**Available Commands:**

```bash
cronosd preconfer submit-priority-tx <tx-file> [priority-level]
cronosd preconfer get-status <tx-hash>
cronosd preconfer list-priority-txs [limit]
cronosd preconfer mempool-stats
cronosd preconfer whitelist add <address>
cronosd preconfer whitelist remove <address>
cronosd preconfer whitelist list
```

---

## Configuration

### App Configuration (`app.toml`)

```toml
[preconfer]
# Enable the priority transaction selector
enable = true

# Validator address for signing preconfirmations (optional)
# Format: Bech32 validator address (e.g., cronosvaloper1...)
validator_address = ""

# Preconfirmation timeout duration (default: "30s")
# Time before a preconfirmation expires
# Accepts Go duration format: "10s", "1m", "90s", "1m30s", etc.
preconfirm_timeout = "30s"

# Whitelist for priority boosting (optional)
# If empty, all addresses can use priority boosting
# If non-empty, only listed addresses can boost priority
whitelist = []
```

### Programmatic Configuration

```go
// Create execution book
mempoolConfig := executionbook.ExecutionBookConfig{
    BaseMempool:        baseMempool,
    TxDecoder:          txDecoder,
    PriorityBoost:      1_000_000_000,
    Logger:             logger,
    WhitelistAddresses: []string{"0x1234...", "0x5678..."},
}
preconferMempool := executionbook.NewExecutionBook(mempoolConfig)

// Load validator private key for signing preconfirmations (optional)
// Supports Ed25519 (default) and secp256k1
privKey, err := executionbook.LoadPrivKeyFromHex("your_private_key_hex", "ed25519")
if err != nil {
    log.Fatal(err)
}

// Create priority tx service
serviceConfig := executionbook.PriorityTxServiceConfig{
    App:               app.BaseApp,
    Mempool:           preconferMempool,
    TxDecoder:         txDecoder,
    Logger:            logger,
    ValidatorAddress:  "cronosvaloper1...",
    ValidatorPrivKey:  privKey, // Optional: for cryptographic signing
    PreconfirmTimeout: 30 * time.Second,
}
service := executionbook.NewPriorityTxService(serviceConfig)

// Register gRPC service
grpcServer := executionbook.NewPriorityTxGRPCServer(service)
executionbook.RegisterPriorityTxServiceServer(app.GRPCQueryRouter(), grpcServer)
```

---

## Usage

### Submitting Priority Transactions

#### Via Transaction Memo

Add `PRIORITY:` prefix to transaction memo:

```go
// Cosmos SDK transaction
tx := txBuilder.GetTx()
txBuilder.SetMemo("PRIORITY: urgent transaction")

// Ethereum transaction (using ExtensionOptionsDynamicFeeTx)
dynamicFeeTx := &types.ExtensionOptionsDynamicFeeTx{
    Memo: "PRIORITY: urgent eth tx",
}
```

**Note**: All priority transactions receive the same boost. There is no distinction between different priority levels when using the memo prefix.

#### Via gRPC API

```go
client := executionbook.NewPriorityTxServiceClient(conn)

req := &executionbook.SubmitPriorityTxRequest{
    TxBytes:       txBytes,
    PriorityLevel: 1,  // Must be 1 (only priority level currently supported)
}

resp, err := client.SubmitPriorityTx(ctx, req)
if err != nil {
    return err
}

fmt.Printf("Transaction submitted: %s\n", resp.TxHash)
fmt.Printf("Mempool position: %d\n", resp.MempoolPosition)
if resp.Preconfirmation != nil {
    fmt.Printf("Preconfirmed by: %s\n", resp.Preconfirmation.Validator)
    fmt.Printf("Expires at: %v\n", time.Unix(resp.Preconfirmation.ExpiresAt, 0))
}
```

#### Via CLI

```bash
# Submit priority transaction (priority level must be 1)
cronosd preconfer submit-priority-tx tx.json 1

# Check status
cronosd preconfer get-status ABC123...

# List pending priority transactions
cronosd preconfer list-priority-txs 10

# Check mempool statistics
cronosd preconfer mempool-stats
```

### Querying Transaction Status

```go
statusResp, err := client.GetPriorityTxStatus(ctx, &executionbook.GetPriorityTxStatusRequest{
    TxHash: "ABC123...",
})

fmt.Printf("Status: %s\n", statusResp.Status) // "pending", "preconfirmed", "included", etc.
fmt.Printf("In mempool: %v\n", statusResp.InMempool)
fmt.Printf("Block height: %d\n", statusResp.BlockHeight)
```

### Managing Whitelist

```bash
# Add address to whitelist
cronosd preconfer whitelist add 0x1234567890123456789012345678901234567890

# Remove address from whitelist
cronosd preconfer whitelist remove 0x1234567890123456789012345678901234567890

# List whitelisted addresses
cronosd preconfer whitelist list

# Check if address is whitelisted
cronosd preconfer whitelist is-whitelisted 0x1234567890123456789012345678901234567890
```

---

## API Reference

### SubmitPriorityTxRequest

```go
type SubmitPriorityTxRequest struct {
    TxBytes          []byte
    PriorityLevel    uint32  // Must be 1
    WaitForInclusion bool    // Wait for block inclusion
}
```

### SubmitPriorityTxResponse

```go
type SubmitPriorityTxResponse struct {
    TxHash          string
    Accepted        bool
    Reason          string
    Preconfirmation *Preconfirmation
    MempoolPosition uint32
}
```

### GetPriorityTxStatusResponse

```go
type GetPriorityTxStatusResponse struct {
    Status          string  // "pending", "preconfirmed", "included", etc.
    InMempool       bool
    BlockHeight     int64
    MempoolPosition uint32
    Preconfirmation *Preconfirmation
    Timestamp       int64
}
```

### Preconfirmation

```go
type Preconfirmation struct {
    TxHash        string
    Timestamp     int64
    Validator     string
    PriorityLevel uint32  // Always 1 (only level currently supported)
    Signature     []byte
    ExpiresAt     int64
}
```

### GetMempoolStatsResponse

```go
type GetMempoolStatsResponse struct {
    TotalTxs         uint32
    PriorityTxs      uint32
    PreconfirmedTxs  uint32
    AvgWaitTime      uint32
    OldestTxTime     int64
}
```

---

## Testing

### Running Tests

```bash
# Run all preconfer tests
go test -v ./preconfer/...

# Run specific test
go test -v ./preconfer/... -run TestPriorityTxService_SubmitPriorityTx

# Run with coverage
go test -v -coverprofile=coverage.out ./preconfer/...
go tool cover -html=coverage.out
```

### Test Coverage

The package includes comprehensive tests:

- **Unit Tests**: All components have unit tests
- **Integration Tests**: End-to-end workflow tests
- **Mempool Tests**: Priority boosting and whitelist tests
- **gRPC Tests**: API endpoint tests
- **Selector Tests**: Transaction selection tests

**Current Coverage**: ~85% code coverage

### Key Test Files

- `priority_tx_service_test.go` - Service layer tests
- `execution_book_test.go` - ExecutionBook tests
- `execution_book_whitelist_test.go` - Whitelist tests
- `priority_tx_selector_test.go` - Selector tests
- `priority_helpers_test.go` - Helper function tests
- `ethereum_priority_test.go` - Ethereum support tests

---

## Implementation Details

### Priority Levels

**Current Implementation**: Single priority level (1)

All priority transactions receive the same boost. Future versions may support multiple priority levels with different boost values.

```go
const (
    DefaultPriorityBoost int64 = 1_000_000_000
)
```

### Transaction Marking

Transactions are marked as priority in two ways:

1. **Memo Prefix**: Add `PRIORITY:` to transaction memo
2. **Ethereum Extension**: Use `ExtensionOptionsDynamicFeeTx` memo field

**Detection Logic:**

```go
func IsMarkedPriorityTx(tx sdk.Tx) bool {
    // Check standard memo
    if txWithMemo, ok := tx.(sdk.TxWithMemo); ok {
        if strings.HasPrefix(txWithMemo.GetMemo(), "PRIORITY:") {
            return true
        }
    }
    
    // Check Ethereum extension
    for _, msg := range tx.GetMsgs() {
        if ethTx, ok := msg.(*evmtypes.MsgEthereumTx); ok {
            if ext := ethTx.GetExtensionOptions(); len(ext) > 0 {
                // Check extension for priority marker
                if hasPriorityInExtension(ext) {
                    return true
                }
            }
        }
    }
    
    return false
}
```

### Priority Boost Mechanism

When a priority transaction is inserted into the mempool:

1. **Validation**: Check if transaction is marked as priority
2. **Whitelist Check**: Verify sender is authorized (if whitelist enabled)
3. **Priority Boost**: Add `DefaultPriorityBoost` to transaction's priority
4. **Mempool Insert**: Insert with boosted priority into base mempool

```go
func (m *ExecutionBook) Insert(ctx context.Context, tx sdk.Tx) error {
    if IsMarkedPriorityTx(tx) && m.isAddressWhitelisted(tx) {
        sdkCtx := sdk.UnwrapSDKContext(ctx)
        boostedPriority := sdkCtx.Priority() + m.priorityBoost
        ctx = sdkCtx.WithPriority(boostedPriority)
    }
    return m.Mempool.Insert(ctx, tx)
}
```

### Preconfirmation Flow

1. **Transaction Submission**: Client submits priority transaction
2. **Validation**: Service validates priority level and format
3. **Mempool Insert**: Transaction inserted with priority boost
4. **Preconfirmation Creation**:
   - Generate preconfirmation with timestamp
   - Set expiration time (current + timeout)
   - Sign with validator key (if configured)
5. **Response**: Return preconfirmation to client
6. **Tracking**: Service tracks transaction status
7. **Expiration**: Cleanup expired preconfirmations

### Cryptographic Signing

Preconfirmations can be cryptographically signed to ensure authenticity and prevent tampering.

**Supported Key Types:**
- **Ed25519** (default) - Standard for Cosmos SDK validators
- **secp256k1** - Alternative signing algorithm

**Message Format:**

```
PRECONFIRM | txHash | priorityLevel (4 bytes) | validatorAddress
```

The message is hashed with SHA-256 before signing.

**Loading Private Keys:**

```go
// Load Ed25519 key from 32-byte hex string (will derive public key)
privKey, err := executionbook.LoadPrivKeyFromHex("your_private_key_hex", "ed25519")

// Load secp256k1 key
privKey, err := executionbook.LoadPrivKeyFromHex("your_private_key_hex", "secp256k1")

// Load from raw bytes (32 bytes for Ed25519, 32 bytes for secp256k1)
privKey, err := executionbook.LoadPrivKeyFromBytes(keyBytes, "ed25519")
```

**Verifying Signatures:**

```go
// Get validator's public key
pubKey := service.GetPublicKey()

// Verify signature
isValid := service.VerifyPreconfirmationSignature(
    txHash, 
    priorityLevel, 
    signature, 
    pubKey,
)
```

**Security Considerations:**

- Private keys should be stored securely (HSM, key management service, etc.)
- If no private key is configured, preconfirmations will be unsigned
- Signatures are **non-binding** - they provide authenticity but not consensus guarantees
- Clients should verify signatures against the validator's public key
- The service logs whether signing is enabled on initialization

### Position Calculation

Position in mempool is calculated by counting priority transactions:

```go
func (s *PriorityTxService) countPriorityTxsInMempool() uint32 {
    var count uint32
    for _, info := range s.txTracker {
        if info.InMempool && info.Preconfirmation != nil {
            count++
        }
    }
    return count
}
```

Position is calculated **before** adding the current transaction:
```go
position := s.countPriorityTxsInMempool() + 1
```

### Status Transitions

```
Unknown → Pending → Preconfirmed → Included
                                 ↘ Rejected
                                 ↘ Expired
```

**Status Strings:**
- `"unknown"` - Initial/unknown status
- `"pending"` - Submitted to mempool
- `"preconfirmed"` - Preconfirmation issued
- `"included"` - Included in block
- `"rejected"` - Transaction rejected
- `"expired"` - Preconfirmation expired

### Whitelist Implementation

The whitelist uses Ethereum addresses (0x...) for access control:

```go
type ExecutionBook struct {
    whitelistMu sync.RWMutex
    whitelist   map[string]bool  // address -> enabled
}

func (m *ExecutionBook) isAddressWhitelisted(tx sdk.Tx) bool {
    // If whitelist is empty, all addresses allowed
    if len(m.whitelist) == 0 {
        return true
    }
    
    // Extract signer address and check whitelist
    signers := m.signerExtractor.GetSigners(tx)
    for _, signer := range signers {
        addr := common.BytesToAddress(signer).Hex()
        if m.whitelist[strings.ToLower(addr)] {
            return true
        }
    }
    
    return false
}
```

### Ethereum Transaction Support

Full support for Ethereum-style transactions:

- **MsgEthereumTx**: Native Ethereum transaction type
- **Dynamic Fee Transactions**: EIP-1559 support
- **Extension Options**: Priority marker in extensions
- **Memo Support**: Use extension memo for priority marking

**Ethereum Transaction Info:**

```go
type EthereumTxInfo struct {
    HasEthereumTx   bool
    EthereumTxCount int
    CosmosMessages  int
    TotalMessages   int
}
```

### Transaction Type Detection

```go
func GetTransactionType(tx sdk.Tx) string {
    // Returns: "ethereum", "cosmos", "mixed", "empty", or "unknown"
    
    hasEthTx := false
    hasOtherTx := false
    
    for _, msg := range tx.GetMsgs() {
        if _, ok := msg.(*evmtypes.MsgEthereumTx); ok {
            hasEthTx = true
        } else {
            hasOtherTx = true
        }
    }
    
    if hasEthTx && !hasOtherTx {
        return "ethereum"
    } else if !hasEthTx && hasOtherTx {
        return "cosmos"
    } else if hasEthTx && hasOtherTx {
        return "mixed"
    }
    
    return StatusUnknown
}
```

### Cleanup and Maintenance

**Expired Preconfirmations:**

The service runs a background goroutine to clean up expired preconfirmations:

```go
func (s *PriorityTxService) cleanupExpiredPreconfirmations() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        now := time.Now()
        s.preconfirmationMutex.Lock()
        
        for txHash, preconf := range s.preconfirmations {
            if now.After(preconf.ExpiresAt) {
                delete(s.preconfirmations, txHash)
                // Update status to expired
            }
        }
        
        s.preconfirmationMutex.Unlock()
    }
}
```

---

## Constants

```go
const (
    // DefaultPriorityBoost is the priority increase for marked transactions
    DefaultPriorityBoost int64 = 1_000_000_000
    
    // StatusUnknown is the string representation for unknown status
    StatusUnknown = "unknown"
    
    // Default preconfirmation timeout
    DefaultPreconfirmTimeout = 30 * time.Second
)
```

---

## Error Handling

The package uses standard Go error patterns:

```go
// Service errors
var (
    ErrInvalidPriorityLevel = errors.New("invalid priority level: must be 1")
    ErrTxNotFound          = errors.New("transaction not found")
    ErrNotWhitelisted      = errors.New("address not whitelisted for priority boosting")
)

// gRPC errors
status.Error(codes.InvalidArgument, "tx hash is required")
status.Error(codes.Internal, "failed to get tx status")
status.Error(codes.NotFound, "transaction not found")
```

---

## Performance Considerations

### Mempool Performance

- **Priority Boost**: O(1) operation, no performance impact
- **Whitelist Check**: O(1) map lookup
- **Transaction Selection**: Handled by base mempool's ordering

### Service Performance

- **Tracking**: In-memory map with RWMutex for concurrent access
- **Cleanup**: Background goroutine runs every 10 seconds
- **Position Calculation**: O(n) where n is number of tracked transactions

### Optimization Tips

1. **Whitelist Size**: Keep whitelist reasonably sized (<1000 addresses)
2. **Cleanup Frequency**: Adjust cleanup interval based on load
3. **Tracking Limit**: Consider adding limits on tracked transactions
4. **Memory Usage**: Monitor memory with many pending transactions

---

## Security Considerations

### Preconfirmation Security

⚠️ **Important**: Preconfirmations are **non-binding**:

- Not consensus-guaranteed
- Validator may go offline
- Block may be reorganized
- Should be used for UX, not security

### Whitelist Security

- Uses Ethereum address format (0x...)
- Case-insensitive comparison
- Thread-safe with RWMutex
- Empty whitelist = all allowed (default)

### DoS Prevention

- Single priority level limits abuse
- Whitelist can restrict access
- Standard mempool limits apply
- No guaranteed inclusion

---

## Future Enhancements

Possible improvements for future versions:

1. **Multiple Priority Levels**: Support different priority tiers
2. **Dynamic Pricing**: Fee-based priority levels
3. **Preconfirmation Aggregation**: Multi-validator preconfirmations
4. **Metrics**: Prometheus metrics for monitoring
5. **Rate Limiting**: Per-address rate limits
6. **Priority Decay**: Time-based priority reduction
7. **Guaranteed Inclusion**: Consensus-backed guarantees

---

## Troubleshooting

### Common Issues

**Transaction not getting priority boost:**
- Check memo has `PRIORITY:` prefix
- Verify address is whitelisted (if enabled)
- Confirm preconfer is enabled in config

**Preconfirmation not created:**
- Check validator address is configured
- Verify transaction is valid
- Ensure priority level is 1

**Whitelist not working:**
- Use Ethereum address format (0x...)
- Check address is in whitelist
- Verify whitelist is not empty

### Debug Logging

Enable debug logging to troubleshoot:

```toml
[log]
level = "debug"
```

Look for log messages:
- `"inserting priority transaction"` - Priority boost applied
- `"priority transaction rejected"` - Whitelist rejection
- `"counted priority transactions in mempool"` - Position calculation
- `"priority transaction accepted"` - Successful submission

**Last Updated**: October 15, 2025


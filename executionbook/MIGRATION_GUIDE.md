# Migration Guide: Priority Transaction System → Sequencer-Based ExecutionBook

## Overview

This guide explains how to migrate from the old priority transaction system to the new sequencer-based ExecutionBook system.

## Breaking Changes

### 1. ExecutionBook is No Longer a Mempool Wrapper

**Old System:**
```go
// ExecutionBook wrapped a base mempool
type ExecutionBook struct {
    mempool.Mempool  // Embedded base mempool
    // ... priority boost fields
}

// Used as a drop-in mempool replacement
mpool = executionbook.NewExecutionBook(executionbook.ExecutionBookConfig{
    BaseMempool:     baseMpool,
    TxDecoder:       txDecoder,
    PriorityBoost:   1000000,
    // ...
})
```

**New System:**
```go
// ExecutionBook is a standalone transaction book
type ExecutionBook struct {
    transactions     map[string]*SequencerTransaction
    sequencerPubKeys map[string]cryptotypes.PubKey
    // ... no embedded mempool
}

// Separate from mempool - stores sequencer transactions
book := executionbook.NewExecutionBook(executionbook.ExecutionBookConfig{
    Logger:           logger,
    SequencerPubKeys: sequencerKeys,  // No BaseMempool!
})
```

### 2. No More Priority Transaction Prefix

**Old System:**
- Users sent transactions with `PRIORITY:` prefix
- Mempool boosted priority for whitelisted addresses

**New System:**
- Sequencers pre-execute transactions off-chain
- Relayers submit (txHash + signature + sequence) to ExecutionBook
- No user-facing prefix needed

### 3. Block Building Changed

**Old System:**
- Validators selected transactions from mempool
- Priority-boosted transactions were selected first

**New System:**
- ProposalHandler builds blocks ONLY from ExecutionBook
- Transactions must come through sequencers
- No mempool transaction selection

## Required Changes in app/app.go

### Lines to Remove/Update

#### 1. Remove Field (Line 322)
```go
// OLD - REMOVE
preconferMempool *executionbook.ExecutionBook
```

#### 2. Update Field Declaration (Line 325)
```go
// OLD - REMOVE
priorityTxService *executionbook.PriorityTxService

// NEW - ADD
executionBook     *executionbook.ExecutionBook
proposalHandler   *executionbook.ProposalHandler  
sequencerGRPC     *executionbook.SequencerGRPCServer
```

#### 3. Remove Old Mempool Integration (Lines 389-431)
```go
// OLD - REMOVE ALL THIS CODE
preconferEnabled := cast.ToBool(appOpts.Get("preconfer.enable"))
preconferWhitelist := cast.ToStringSlice(appOpts.Get("preconfer.whitelist"))

var mpool mempool.Mempool
var preconferMempoolRef *executionbook.ExecutionBook
if maxTxs := cast.ToInt(appOpts.Get(server.FlagMempoolMaxTxs)); maxTxs >= 0 {
    // ... old mempool wrapping code ...
    if preconferEnabled {
        preconferMpool := executionbook.NewExecutionBook(executionbook.ExecutionBookConfig{
            BaseMempool: baseMpool,  // ← This no longer exists
            // ...
        })
        mpool = preconferMpool
        preconferMempoolRef = preconferMpool
    }
}
```

**NEW - REPLACE WITH:**
```go
// Initialize sequencer-based ExecutionBook
var executionBook *executionbook.ExecutionBook
var proposalHandler *executionbook.ProposalHandler
var sequencerGRPC *executionbook.SequencerGRPCServer

executionBookEnabled := cast.ToBool(appOpts.Get("executionbook.enabled"))
if executionBookEnabled {
    // Load sequencer public keys from configuration
    sequencerKeys := loadSequencerPubKeys(appOpts)
    
    // Create ExecutionBook
    executionBook = executionbook.NewExecutionBook(executionbook.ExecutionBookConfig{
        Logger:           logger.With("module", "executionbook"),
        SequencerPubKeys: sequencerKeys,
    })
    
    // Create ProposalHandler
    proposalHandler = executionbook.NewProposalHandler(executionbook.ProposalHandlerConfig{
        Book:      executionBook,
        TxDecoder: txDecoder,
        Logger:    logger.With("module", "proposal_handler"),
    })
    
    // Create gRPC server
    sequencerGRPC = executionbook.NewSequencerGRPCServer(executionBook)
    
    logger.Info("ExecutionBook initialized",
        "sequencer_count", len(sequencerKeys),
        "enabled", true)
}

// Keep normal mempool for non-sequencer mode or fallback
var mpool mempool.Mempool
if maxTxs := cast.ToInt(appOpts.Get(server.FlagMempoolMaxTxs)); maxTxs >= 0 {
    mpool = mempool.NewPriorityMempool(mempool.PriorityNonceMempoolConfig[int64]{
        TxPriority:      mempool.NewDefaultTxPriority(),
        SignerExtractor: evmapp.NewEthSignerExtractionAdapter(mempool.NewDefaultSignerExtractionAdapter()),
        MaxTx:           maxTxs,
    })
} else {
    mpool = mempool.NoOpMempool{}
}
```

#### 4. Remove Old Priority Service (Lines 497-539)
```go
// OLD - REMOVE
if preconferMempoolRef != nil {
    // ... old priority tx service initialization ...
}

// NEW - ADD
app.executionBook = executionBook
app.proposalHandler = proposalHandler
app.sequencerGRPC = sequencerGRPC

// Set ABCI handlers if ExecutionBook is enabled
if proposalHandler != nil {
    bApp.SetPrepareProposal(proposalHandler.PrepareProposalHandler())
    bApp.SetProcessProposal(proposalHandler.ProcessProposalHandler())
}
```

#### 5. Update gRPC Registration (Lines 1048-1049)
```go
// OLD - REMOVE
if app.preconferMempool != nil {
    whitelistServer := executionbook.NewWhitelistGRPCServer(app.preconferMempool)
    // ... registration ...
}

// NEW - REPLACE
if app.sequencerGRPC != nil {
    // TODO: Register sequencer gRPC services
    // This requires protobuf definitions to be added
    // For now, the gRPC server is ready but not registered
}
```

### Helper Function to Add

Add this function to load sequencer keys from configuration:

```go
// loadSequencerPubKeys loads sequencer public keys from app configuration
func loadSequencerPubKeys(appOpts servertypes.AppOptions) map[string]cryptotypes.PubKey {
    sequencerKeys := make(map[string]cryptotypes.PubKey)
    
    // Load from app.toml
    // Expected format in app.toml:
    // [executionbook]
    // sequencers = [
    //   {id = "seq1", pubkey = "base64_encoded_pubkey", type = "ed25519"},
    // ]
    
    // Example implementation:
    sequencersConfig := cast.ToString(appOpts.Get("executionbook.sequencers"))
    if sequencersConfig != "" {
        // Parse JSON or TOML format
        // This is application-specific
        // For now, return empty map or load from environment
    }
    
    return sequencerKeys
}
```

## Configuration Changes

### Old app.toml
```toml
[preconfer]
enable = true
whitelist = ["0x1234..."]
```

### New app.toml
```toml
[executionbook]
enabled = true
sequencers = [
    {id = "sequencer_1", pubkey = "A1B2C3D4...", type = "ed25519"},
    {id = "sequencer_2", pubkey = "F6E5D4C3...", type = "ed25519"},
]
max_txs_per_block = 1000
```

## Runtime Behavior Changes

### Old System
1. User sends `PRIORITY:0xabc123...` transaction
2. Mempool checks whitelist
3. If whitelisted, boost priority by 1,000,000
4. Validator selects high-priority transactions for block

### New System
1. User sends transaction to sequencer (off-chain)
2. Sequencer executes and assigns sequence number
3. Relayer submits (txHash, sequence, signature) to ExecutionBook
4. ProposalHandler builds block using only ExecutionBook transactions
5. All transactions included in sequence order

## CLI Command Changes

### Old Commands (REMOVED)
```bash
cronosd preconfer whitelist add 0x1234...
cronosd preconfer whitelist remove 0x1234...
cronosd preconfer whitelist list
cronosd preconfer whitelist clear
cronosd preconfer whitelist set 0x1234... 0x5678...
```

### New Commands
```bash
cronosd executionbook sequencer list
cronosd executionbook stats
```

## Testing Migration

### 1. Remove Old Tests
Delete any tests that relied on priority transaction prefixes or whitelist functionality.

### 2. Add New Tests
Test the sequencer transaction submission flow:

```go
func TestSequencerIntegration(t *testing.T) {
    // 1. Generate sequencer keys
    seqPrivKey := ed25519.GenPrivKey()
    seqPubKey := seqPrivKey.PubKey()
    
    // 2. Initialize app with ExecutionBook
    app := NewApp(/* ... */)
    
    // 3. Submit sequencer transaction
    txHash := "0xabc123"
    signature, _ := executionbook.CreateSequencerSignature(txHash, 0, seqPrivKey)
    
    req := &executionbook.SubmitSequencerTxRequest{
        TxHash:         txHash,
        SequenceNumber: 0,
        Signature:      signature,
        SequencerID:    "seq1",
    }
    
    resp, err := app.sequencerGRPC.SubmitSequencerTx(ctx, req)
    require.NoError(t, err)
    require.True(t, resp.Success)
    
    // 4. Verify transaction in book
    stats := app.executionBook.GetStats()
    require.Equal(t, 1, stats.PendingTransactions)
}
```

## Rollout Strategy

### Phase 1: Parallel Operation (Recommended)
1. Keep old mempool system operational
2. Add new ExecutionBook alongside
3. Both systems can coexist initially
4. Gradually migrate traffic to sequencers

```go
// Run both systems
if executionBookEnabled {
    // Use ExecutionBook for sequencer transactions
    bApp.SetPrepareProposal(hybridProposalHandler)
} else {
    // Fall back to mempool
    // Keep existing behavior
}
```

### Phase 2: Full Migration
1. Disable priority transaction system
2. Remove old mempool wrapping code
3. Use only ExecutionBook + ProposalHandler
4. Remove old configuration

### Phase 3: Cleanup
1. Delete old code files
2. Update all documentation
3. Remove old dependencies
4. Archive migration guides

## Troubleshooting

### Error: "BaseMempool field not found"
**Cause:** Using old ExecutionBook API  
**Fix:** Update to new `ExecutionBookConfig` without `BaseMempool`

### Error: "ExecutionBook does not implement Mempool"
**Cause:** Trying to use ExecutionBook as a mempool  
**Fix:** ExecutionBook is no longer a mempool - use separate mempool and ExecutionBook

### Error: "PriorityTxService undefined"
**Cause:** Old priority transaction service removed  
**Fix:** Use `SequencerGRPCServer` instead

### Error: "WhitelistGRPCServer undefined"  
**Cause:** Whitelist system removed  
**Fix:** Use sequencer authentication instead

## Backward Compatibility

❌ **NOT backward compatible** - This is a breaking change

The systems are fundamentally different:
- Old: User-driven priority transactions
- New: Sequencer-driven ordered transactions

**Migration is required** - there is no compatibility layer.

## Support

For questions during migration:
- Review `README.md` for new API reference
- See `INTEGRATION.md` for step-by-step integration
- Check `IMPLEMENTATION_SUMMARY.md` for architecture changes
- Run tests: `go test ./executionbook/...`

## Checklist

- [ ] Remove `preconferMempool` field from App struct
- [ ] Add `executionBook`, `proposalHandler`, `sequencerGRPC` fields
- [ ] Update app initialization code
- [ ] Remove old mempool wrapping code
- [ ] Add sequencer key loading logic
- [ ] Set ABCI handlers (PrepareProposal, ProcessProposal)
- [ ] Update gRPC service registration
- [ ] Update app.toml configuration
- [ ] Remove old priority transaction tests
- [ ] Add sequencer transaction tests
- [ ] Update deployment documentation
- [ ] Train team on new architecture
- [ ] Update monitoring and alerting
- [ ] Test with sequencer infrastructure


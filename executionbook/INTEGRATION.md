# ExecutionBook Integration Guide

This guide shows how to integrate the ExecutionBook into a Cronos application.

## Step 1: Initialize ExecutionBook in App

Add to your `app/app.go`:

```go
import (
    "github.com/crypto-org-chain/cronos/v2/executionbook"
    "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
    cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

type App struct {
    *baseapp.BaseApp
    
    // Add ExecutionBook
    executionBook    *executionbook.ExecutionBook
    proposalHandler  *executionbook.ProposalHandler
    sequencerGRPC    *executionbook.SequencerGRPCServer
    
    // ... other fields
}

func NewApp(...) *App {
    // ... existing initialization ...
    
    // Initialize ExecutionBook
    sequencerPubKeys := loadSequencerPubKeys(appOpts) // Load from config
    
    executionBook := executionbook.NewExecutionBook(executionbook.ExecutionBookConfig{
        Logger:           logger.With("module", "executionbook"),
        SequencerPubKeys: sequencerPubKeys,
    })
    
    // Initialize ProposalHandler
    proposalHandler := executionbook.NewProposalHandler(executionbook.ProposalHandlerConfig{
        Book:      executionBook,
        TxDecoder: app.txConfig.TxDecoder(),
        Logger:    logger.With("module", "proposal_handler"),
    })
    
    // Initialize gRPC Server
    sequencerGRPC := executionbook.NewSequencerGRPCServer(executionBook)
    
    app.executionBook = executionBook
    app.proposalHandler = proposalHandler
    app.sequencerGRPC = sequencerGRPC
    
    // Set ABCI handlers
    app.SetPrepareProposal(proposalHandler.PrepareProposalHandler())
    app.SetProcessProposal(proposalHandler.ProcessProposalHandler())
    
    return app
}
```

## Step 2: Load Sequencer Keys from Configuration

Add sequencer configuration to your app config:

```go
// In app/app.go or config package
func loadSequencerPubKeys(appOpts servertypes.AppOptions) map[string]cryptotypes.PubKey {
    sequencerKeys := make(map[string]cryptotypes.PubKey)
    
    // Load from app.toml or environment variables
    // Example format:
    // [executionbook]
    // sequencers = [
    //   {id = "seq1", pubkey = "base64_encoded_pubkey"},
    //   {id = "seq2", pubkey = "base64_encoded_pubkey"},
    // ]
    
    keysJSON := cast.ToString(appOpts.Get("executionbook.sequencers"))
    if keysJSON != "" {
        // Parse and load keys
        // Implementation depends on your config format
    }
    
    return sequencerKeys
}
```

## Step 3: Register gRPC Services

Add to your gRPC server registration:

```go
// In app/app.go
func (app *App) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {
    // ... existing routes ...
    
    // Register ExecutionBook gRPC services
    // This would require proper protobuf definitions
    // For now, this is a placeholder for future implementation
}
```

## Step 4: Hook Block Commit Events

Add cleanup after block commits:

```go
// In app/app.go, override Commit
func (app *App) Commit() (res abci.ResponseCommit) {
    res = app.BaseApp.Commit()
    
    // Extract transaction hashes from the committed block
    block := app.BaseApp.GetBlockStore().LoadBlock(res.Height)
    txHashes := make([]string, 0, len(block.Txs))
    for _, tx := range block.Txs {
        txHashes = append(txHashes, executionbook.CalculateTxHash(tx))
    }
    
    // Notify proposal handler
    if err := app.proposalHandler.OnBlockCommit(res.Height, txHashes); err != nil {
        app.Logger().Error("Failed to process block commit", "error", err)
    }
    
    return res
}
```

## Step 5: Configuration File

Add to your `app.toml`:

```toml
###############################################################################
###                           ExecutionBook Configuration                  ###
###############################################################################

[executionbook]
# Enable execution book for sequencer-based ordering
enabled = true

# Sequencer public keys for signature verification
# Format: [{id = "sequencer_id", pubkey = "base64_encoded_ed25519_or_secp256k1_pubkey"}]
sequencers = [
    {id = "sequencer_1", pubkey = "A1B2C3D4E5F6..."},
    {id = "sequencer_2", pubkey = "F6E5D4C3B2A1..."},
]

# Maximum transactions per block from execution book (0 = unlimited)
max_txs_per_block = 1000

# Reject blocks with non-sequencer transactions
strict_mode = true
```

## Step 6: CLI Integration

The CLI commands are automatically available:

```bash
# List sequencers
cronosd executionbook sequencer list

# Get execution book statistics  
cronosd executionbook stats
```

## Step 7: Relayer Integration

Relayers submit transactions via gRPC:

```go
// Example relayer code
import (
    "context"
    "github.com/crypto-org-chain/cronos/v2/executionbook"
)

func relayTransaction(
    client *executionbook.SequencerGRPCServer,
    txHash string,
    seq uint64,
    signature []byte,
    seqID string,
) error {
    req := &executionbook.SubmitSequencerTxRequest{
        TxHash:         txHash,
        SequenceNumber: seq,
        Signature:      signature,
        SequencerID:    seqID,
    }
    
    resp, err := client.SubmitSequencerTx(context.Background(), req)
    if err != nil {
        return err
    }
    
    if !resp.Success {
        return fmt.Errorf("submission failed: %s", resp.Message)
    }
    
    return nil
}
```

## Step 8: Sequencer Key Generation

Generate sequencer keys:

```go
import (
    "encoding/base64"
    "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
)

// Generate new sequencer key pair
privKey := ed25519.GenPrivKey()
pubKey := privKey.PubKey()

// Export for configuration
pubKeyBase64 := base64.StdEncoding.EncodeToString(pubKey.Bytes())
privKeyBase64 := base64.StdEncoding.EncodeToString(privKey.Bytes())

fmt.Printf("Sequencer Public Key: %s\n", pubKeyBase64)
fmt.Printf("Sequencer Private Key (keep secret!): %s\n", privKeyBase64)
```

## Testing Integration

Test the integration:

```bash
# 1. Start the node with ExecutionBook enabled
cronosd start --executionbook.enabled=true

# 2. In another terminal, check stats
cronosd executionbook stats

# 3. Submit a test transaction (requires relayer)
# See relayer documentation for submission

# 4. Verify transaction inclusion
cronosd query tx <tx_hash>
```

## Monitoring

Monitor ExecutionBook metrics:

```go
// Add Prometheus metrics
import "github.com/prometheus/client_golang/prometheus"

var (
    executionBookTxs = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "execution_book_transactions",
            Help: "Number of transactions in execution book",
        },
        []string{"status"}, // pending, included
    )
)

// Update metrics periodically
stats := app.executionBook.GetStats()
executionBookTxs.WithLabelValues("pending").Set(float64(stats.PendingTransactions))
executionBookTxs.WithLabelValues("included").Set(float64(stats.IncludedTransactions))
```

## Troubleshooting

### Common Issues

1. **"unknown sequencer" error**
   - Verify sequencer ID matches configuration
   - Check sequencer public key is correctly loaded

2. **"sequence number mismatch" error**
   - Sequence may have advanced
   - Call `GetStats()` to check current sequence
   - Ensure no gaps in submission

3. **"invalid sequencer signature" error**
   - Verify signature is created correctly
   - Check sequencer private key matches configured public key
   - Ensure message format matches specification

4. **Blocks not including transactions**
   - Check `PrepareProposal` is called
   - Verify transaction bytes are available
   - Check logs for proposal handler errors

## Security Checklist

- [ ] Sequencer private keys stored securely
- [ ] Only trusted relayers have access to gRPC endpoints
- [ ] Sequencer key rotation procedure documented
- [ ] Monitoring alerts for unexpected sequence gaps
- [ ] Regular audits of included transactions
- [ ] Backup sequencers configured for redundancy

## Further Reading

- [README.md](./README.md) - Package overview and API reference
- [Cosmos SDK ABCI++](https://docs.cosmos.network/main/build/abci) - Understanding PrepareProposal/ProcessProposal
- [CometBFT Specification](https://github.com/cometbft/cometbft/tree/main/spec) - Block proposal specification


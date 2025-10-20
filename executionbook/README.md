# ExecutionBook Package

## Overview

The `executionbook` package implements a **sequencer-based transaction ordering system** for Cronos. Unlike traditional blockchain mempools where validators select and order transactions, this system allows **off-chain sequencers** to pre-execute transactions and guarantee their inclusion in blocks in a specific order.

## Architecture

```
┌─────────────┐
│  Sequencer  │  (Off-chain: Executes transactions with sequence numbers)
│  (Off-chain)│
└──────┬──────┘
       │
       ↓ (Forwards pre-executed transactions)
┌──────────────┐
│   Relayer    │
└──────┬───────┘
       │
       ↓ (Submits to execution book via gRPC)
┌─────────────────┐
│ Execution Book  │  (Stores tx hashes + sequence numbers)
│                 │  (Validates sequencer signatures)
└────────┬────────┘
         │
         ↓ (Provides transactions in order)
┌──────────────────┐
│ Proposal Handler │  (Builds blocks ONLY from execution book)
└────────┬─────────┘
         │
         ↓
┌────────────────┐
│ Block Inclusion│
└────────────────┘
```

## Key Components

### 1. ExecutionBook

The core storage and validation system for sequencer transactions.

**Features:**
- Stores transaction hashes with sequence numbers
- Validates sequencer signatures using public keys
- Enforces strict sequence ordering (no gaps allowed)
- Maintains per-block sequence numbering (resets each block)
- Tracks transaction inclusion status

**Key Methods:**
```go
// Submit a sequencer transaction
func (eb *ExecutionBook) SubmitSequencerTx(
    txHash string,
    sequenceNumber uint64,
    signature []byte,
    sequencerID string,
) error

// Get transactions in sequence order
func (eb *ExecutionBook) GetOrderedTransactions() []*SequencerTransaction

// Mark transactions as included in a block
func (eb *ExecutionBook) MarkIncluded(txHashes []string, blockHeight int64)

// Clean up included transactions
func (eb *ExecutionBook) CleanupIncludedTransactions() int

// Reset sequence for new block
func (eb *ExecutionBook) ResetSequence(blockHeight int64)
```

### 2. Sequencer Authentication

Sequencers are identified by their public keys and must sign each transaction submission.

**Signature Format:**
```
Message = SHA256("SEQUENCER_TX|" + txHash + "|" + sequenceNumber)
Signature = Sign(Message, sequencerPrivateKey)
```

**Supported Key Types:**
- Ed25519
- secp256k1

**Managing Sequencers:**
```go
// Add a new sequencer
book.AddSequencer("sequencer_id", publicKey)

// Remove a sequencer
book.RemoveSequencer("sequencer_id")
```

### 3. Sequence Number Management

Sequence numbers enforce strict transaction ordering:

- **Per-block sequence**: Numbers reset to 0 at each block height
- **No gaps allowed**: Each transaction must have exactly `nextSequence`
- **Strict ordering**: Transactions are processed in sequence order
- **Rejection**: Out-of-order or duplicate submissions are rejected

**Example:**
```
Block N:
  - Submit tx1 with sequence 0 ✓
  - Submit tx2 with sequence 1 ✓
  - Submit tx3 with sequence 5 ✗ (gap from 1 to 5)
  - Submit tx4 with sequence 2 ✓

Block N+1:
  - Sequence resets to 0
  - Submit tx5 with sequence 0 ✓
```

### 4. ProposalHandler

Custom block building logic that creates blocks **ONLY** from execution book transactions.

**Features:**
- Replaces default mempool-based block building
- Uses `PrepareProposal` to build blocks from execution book
- Uses `ProcessProposal` to validate proposed blocks
- Automatically cleans up included transactions after block commit

**Integration:**
```go
handler := executionbook.NewProposalHandler(executionbook.ProposalHandlerConfig{
    Book:      book,
    TxDecoder: txDecoder,
    Logger:    logger,
})

// Set ABCI handlers
app.SetPrepareProposal(handler.PrepareProposalHandler())
app.SetProcessProposal(handler.ProcessProposalHandler())

// Hook into block commit
app.SetPostBlockCommit(func(blockHeight int64, txHashes []string) {
    handler.OnBlockCommit(blockHeight, txHashes)
})
```

### 5. gRPC API

REST/gRPC interface for relayers to submit sequencer transactions.

**Endpoints:**

#### Submit Sequencer Transaction
```go
message SubmitSequencerTxRequest {
    string tx_hash = 1;
    uint64 sequence_number = 2;
    bytes signature = 3;
    string sequencer_id = 4;
}

message SubmitSequencerTxResponse {
    bool success = 1;
    string message = 2;
}
```

#### Get Statistics
```go
message GetStatsRequest {}

message GetStatsResponse {
    int32 total_transactions = 1;
    int32 pending_transactions = 2;
    int32 included_transactions = 3;
    uint64 next_sequence = 4;
    int64 current_block_height = 5;
    int32 sequencer_count = 6;
}
```

## Usage

### Creating an ExecutionBook

```go
import (
    "github.com/crypto-org-chain/cronos/v2/executionbook"
    "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
)

// Generate sequencer keys
seq1PrivKey := ed25519.GenPrivKey()
seq1PubKey := seq1PrivKey.PubKey()

// Create execution book
book := executionbook.NewExecutionBook(executionbook.ExecutionBookConfig{
    Logger: logger,
    SequencerPubKeys: map[string]cryptotypes.PubKey{
        "sequencer_1": seq1PubKey,
        "sequencer_2": seq2PubKey,
    },
})
```

### Submitting Transactions (Relayer)

```go
// Transaction hash from sequencer
txHash := "0xabcd1234..."
sequenceNumber := uint64(0)
sequencerID := "sequencer_1"

// Create signature (done by sequencer)
signature, err := executionbook.CreateSequencerSignature(
    txHash, 
    sequenceNumber, 
    sequencerPrivKey,
)

// Submit to execution book
err = book.SubmitSequencerTx(txHash, sequenceNumber, signature, sequencerID)
if err != nil {
    // Handle error (invalid signature, wrong sequence, etc.)
}
```

### Building Blocks (Validator)

```go
// Get ordered transactions for block proposal
sequencerTxs := book.GetOrderedTransactions()

for _, tx := range sequencerTxs {
    fmt.Printf("Include tx %s (sequence %d) from sequencer %s\n",
        tx.TxHash, tx.SequenceNumber, tx.SequencerID)
}
```

### After Block Commit

```go
// Mark transactions as included
txHashes := []string{"hash1", "hash2", "hash3"}
book.MarkIncluded(txHashes, blockHeight)

// Clean up included transactions
cleaned := book.CleanupIncludedTransactions()
fmt.Printf("Cleaned up %d transactions\n", cleaned)

// Reset sequence for next block
book.ResetSequence(nextBlockHeight)
```

## CLI Commands

The package provides CLI commands for managing the execution book:

```bash
# List registered sequencers
cronosd executionbook sequencer list

# Get execution book statistics
cronosd executionbook stats
```

## Transaction Lifecycle

1. **Sequencer Execution**: Off-chain sequencer executes transaction and assigns sequence number
2. **Relayer Submission**: Relayer submits (txHash, sequenceNumber, signature) to execution book
3. **Validation**: ExecutionBook validates signature and sequence order
4. **Storage**: Transaction stored in execution book if valid
5. **Block Building**: Validator uses ProposalHandler to build block from execution book
6. **Inclusion**: Transactions included in block in sequence order
7. **Cleanup**: After block commit, included transactions are removed from book

## Sequence Number Reset

Sequence numbers reset to 0 at the start of each block:

```go
// At block height 100
book.SubmitSequencerTx("tx1", 0, sig, "seq1") // ✓
book.SubmitSequencerTx("tx2", 1, sig, "seq1") // ✓

// Block committed, reset for block 101
book.ResetSequence(101)

// Now sequence starts from 0 again
book.SubmitSequencerTx("tx3", 0, sig, "seq1") // ✓
```

## Error Handling

The execution book performs extensive validation:

- **Unknown Sequencer**: Submission from unregistered sequencer is rejected
- **Invalid Signature**: Cryptographic signature verification failure
- **Sequence Gap**: Submitted sequence number != expected next sequence
- **Duplicate Transaction**: Same transaction hash already submitted
- **Already Included**: Transaction already included in a block

Example error handling:

```go
err := book.SubmitSequencerTx(txHash, seq, sig, seqID)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "unknown sequencer"):
        // Sequencer not registered
    case strings.Contains(err.Error(), "invalid sequencer signature"):
        // Signature verification failed
    case strings.Contains(err.Error(), "sequence number mismatch"):
        // Wrong sequence number (gap detected)
    case strings.Contains(err.Error(), "already submitted"):
        // Duplicate submission
    }
}
```

## Statistics and Monitoring

Get real-time statistics:

```go
stats := book.GetStats()
fmt.Printf("Total transactions: %d\n", stats.TotalTransactions)
fmt.Printf("Pending transactions: %d\n", stats.PendingTransactions)
fmt.Printf("Included transactions: %d\n", stats.IncludedTransactions)
fmt.Printf("Next sequence number: %d\n", stats.NextSequence)
fmt.Printf("Current block height: %d\n", stats.CurrentBlockHeight)
fmt.Printf("Registered sequencers: %d\n", stats.SequencerCount)
```

## Security Considerations

### Sequencer Key Management

- **Keep private keys secure**: Sequencer private keys control transaction ordering
- **Key rotation**: Support for adding/removing sequencers dynamically
- **Multi-sequencer**: Can register multiple sequencers for redundancy

### Signature Verification

- All transactions require valid sequencer signatures
- Signatures are verified before accepting transactions
- Message format is deterministic to prevent replay attacks

### Sequence Enforcement

- Strict ordering prevents transaction reordering attacks
- No gaps allowed ensures complete transaction sets
- Per-block reset prevents long-running sequence issues

## Testing

The package includes comprehensive tests:

```bash
# Run all executionbook tests
go test ./executionbook/...

# Run specific test suites
go test ./executionbook/... -run TestExecutionBook
go test ./executionbook/... -run TestProposalHandler
go test ./executionbook/... -run TestSequencerGRPC
```

## Future Enhancements

Potential improvements for production:

1. **Transaction Pool Integration**: Store actual transaction bytes, not just hashes
2. **Gas Limit Handling**: Respect block gas limits during proposal preparation
3. **MEV Protection**: Additional sequencer-level MEV protection mechanisms
4. **Sequencer Rotation**: Automatic sequencer rotation based on governance
5. **Cross-Chain Integration**: Bridge support for cross-chain sequencing
6. **Monitoring**: Prometheus metrics for execution book performance
7. **Persistence**: Persistent storage for transaction history

## Migration from Priority Transaction System

This package **completely replaces** the old priority transaction system:

- ❌ No more priority transaction prefixes
- ❌ No more mempool-based transaction selection
- ❌ No more whitelist management
- ✅ All blocks built from sequencer transactions
- ✅ Guaranteed ordering and inclusion
- ✅ Off-chain execution before on-chain finalization

## License

This package is part of Cronos and follows the project's license.

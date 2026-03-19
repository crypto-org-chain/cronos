# Fix Unlucky Tx

## Problem

Certain blocks on the Cronos mainnet contain transactions whose gas consumption exceeded the block gas limit. When this happens, the Cosmos SDK's block gas meter marks the transaction as failed (error code 11), but the `ethereum_tx` events are never emitted. This causes:

- `eth_getBlockReceipts` to crash for the affected block
- `eth_getTransactionByHash` / `eth_getTransactionReceipt` to return the wrong block number
- The transaction to be missing from the CometBFT tx index

### Known affected blocks

| Block | Gas Used | Gas Limit | Usage |
|-------|----------|-----------|-------|
| [6541](https://explorer.cronos.org/block/6541) | 10,352,607 | 10,000,000 | 103.53% |
| Blocks between v0.7.0 (2,693,800) and v0.7.1 | varies | varies | > 100% |

## Solution

The `fix-unlucky-tx` command patches the CometBFT database offline to:

1. Add the missing `ethereum_tx` events (with correct `ethereumTxHash` and `txIndex`) to the ABCI response
2. Re-index the transaction in the CometBFT tx indexer

## Prerequisites

- **The node must be stopped** before running this command. It directly modifies the CometBFT database files (blockstore, state, tx_index).
- The node must have `discard_abci_responses = false` in CometBFT config (the default), so that ABCI responses are stored.

## Usage

```
cronosd database fix-unlucky-tx [blocks-file] [flags]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `blocks-file` | Path to a text file with one block height per line. Use `-` to read from stdin. |

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--home` | `~/.cronos` | Node home directory |
| `--min-block-height` | `2693800` | Reject block heights below this value. Block 6541 is always allowed as a known exception. |
| `--chain-id` | `cronosmainnet_25-1` | Chain ID (only used for psql tx indexer backend) |

## Examples

### Patch block 6541

```bash
# Stop the node
sudo systemctl stop cronosd

# Patch
echo "6541" | cronosd database fix-unlucky-tx - --home ~/.cronos

# Start the node
sudo systemctl start cronosd
```

### Patch multiple blocks from a file

```bash
# Create a file with one block height per line
cat > blocks.txt << 'EOF'
6541
2693850
2693900
EOF

# Stop the node
sudo systemctl stop cronosd

# Patch all blocks
cronosd database fix-unlucky-tx blocks.txt --home ~/.cronos

# Start the node
sudo systemctl start cronosd
```

### Find affected blocks using the helper script

The repository includes `scripts/find-unlucky-txs.py` that scans block results for unlucky transactions. To find all affected blocks in a range:

```bash
for height in $(seq 2693800 2700000); do
  curl -s "http://localhost:26657/block_results?height=$height" | python3 scripts/find-unlucky-txs.py
done > blocks.txt
```

Then patch them:

```bash
cronosd database fix-unlucky-tx blocks.txt --home ~/.cronos
```

## Verification

After patching and restarting the node, verify the fix:

```bash
# Check that the transaction can be found via tx_search
curl -s "http://localhost:26657/tx_search?query=\"ethereum_tx.ethereumTxHash='0x...'\"" | jq '.result.total_count'

# Check that eth_getBlockReceipts no longer crashes
curl -s -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_getBlockReceipts","params":["0x198d"],"id":1}' | jq
```

## Idempotency

The command is idempotent. If a block has already been patched (the last event is already `ethereum_tx`), it will be silently skipped. It is safe to run the command multiple times on the same block.

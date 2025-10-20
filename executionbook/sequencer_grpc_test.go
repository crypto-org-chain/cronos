package executionbook

import (
	"context"
	"crypto/sha256"
	"testing"

	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/stretchr/testify/require"
)

// testTxHashGRPC creates a []byte hash from a string for testing
func testTxHashGRPC(s string) []byte {
	hash := sha256.Sum256([]byte(s))
	return hash[:]
}

func TestNewSequencerGRPCServer(t *testing.T) {
	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
	})

	t.Run("Create server with valid book", func(t *testing.T) {
		server := NewSequencerGRPCServer(book)
		require.NotNil(t, server)
		require.Equal(t, book, server.book)
	})

	t.Run("Panic on nil book", func(t *testing.T) {
		require.Panics(t, func() {
			NewSequencerGRPCServer(nil)
		})
	})
}

func TestSequencerGRPCServer_SubmitSequencerTx(t *testing.T) {
	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"seq1": seqPubKey,
		},
	})

	server := NewSequencerGRPCServer(book)
	ctx := context.Background()

	t.Run("Submit valid transaction", func(t *testing.T) {
		txHash := testTxHashGRPC("tx0")
		signature, err := CreateSequencerSignature(txHash, 0, seqPrivKey)
		require.NoError(t, err)

		req := &SubmitSequencerTxRequest{
			TxHash:         txHash,
			SequenceNumber: 0,
			Signature:      signature,
			SequencerID:    "seq1",
		}

		resp, err := server.SubmitSequencerTx(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.Success)
		require.Contains(t, resp.Message, "successfully")
	})

	t.Run("Reject nil request", func(t *testing.T) {
		resp, err := server.SubmitSequencerTx(ctx, nil)
		require.Error(t, err)
		require.Nil(t, resp)
	})

	t.Run("Reject empty tx hash", func(t *testing.T) {
		req := &SubmitSequencerTxRequest{
			TxHash:         []byte{},
			SequenceNumber: 1,
			Signature:      []byte("sig"),
			SequencerID:    "seq1",
		}

		resp, err := server.SubmitSequencerTx(ctx, req)
		require.NoError(t, err) // gRPC succeeds but returns failure
		require.NotNil(t, resp)
		require.False(t, resp.Success)
		require.Contains(t, resp.Message, "invalid tx hash length")
	})

	t.Run("Reject empty signature", func(t *testing.T) {
		req := &SubmitSequencerTxRequest{
			TxHash:         testTxHashGRPC("tx1"),
			SequenceNumber: 1,
			Signature:      []byte{},
			SequencerID:    "seq1",
		}

		resp, err := server.SubmitSequencerTx(ctx, req)
		require.NoError(t, err) // gRPC succeeds but returns failure
		require.NotNil(t, resp)
		require.False(t, resp.Success)
		require.Contains(t, resp.Message, "signature cannot be empty")
	})

	t.Run("Reject empty sequencer ID", func(t *testing.T) {
		req := &SubmitSequencerTxRequest{
			TxHash:         testTxHashGRPC("tx2"),
			SequenceNumber: 1,
			Signature:      []byte("sig"),
			SequencerID:    "",
		}

		resp, err := server.SubmitSequencerTx(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "sequencer_id cannot be empty")
	})

	t.Run("Return error response for invalid signature", func(t *testing.T) {
		req := &SubmitSequencerTxRequest{
			TxHash:         testTxHashGRPC("tx3"),
			SequenceNumber: 1,
			Signature:      []byte("invalid_signature"),
			SequencerID:    "seq1",
		}

		resp, err := server.SubmitSequencerTx(ctx, req)
		require.NoError(t, err) // gRPC call succeeds
		require.NotNil(t, resp)
		require.False(t, resp.Success)
		require.Contains(t, resp.Message, "failed to submit")
	})
}

func TestSequencerGRPCServer_GetStats(t *testing.T) {
	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"seq1": seqPubKey,
			"seq2": seqPubKey,
		},
	})

	server := NewSequencerGRPCServer(book)
	ctx := context.Background()

	// Submit some transactions
	txHashes := [][]byte{}
	for i := 0; i < 5; i++ {
		txHash := testTxHashGRPC(string(rune('a' + i)))
		txHashes = append(txHashes, txHash)
		signature, err := CreateSequencerSignature(txHash, uint64(i), seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash, uint64(i), signature, "seq1")
		require.NoError(t, err)
	}

	// Mark some as included
	book.MarkIncluded([][]byte{txHashes[0], txHashes[1]}, 100)

	t.Run("Get stats", func(t *testing.T) {
		req := &GetStatsRequest{}
		resp, err := server.GetStats(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, int32(5), resp.TotalTransactions)
		require.Equal(t, int32(3), resp.PendingTransactions)
		require.Equal(t, int32(2), resp.IncludedTransactions)
		require.Equal(t, uint64(5), resp.NextSequence)
		require.Equal(t, int32(2), resp.SequencerCount)
	})
}

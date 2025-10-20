package executionbook

import (
	"crypto/sha256"
	"testing"

	"cosmossdk.io/log"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/stretchr/testify/require"
)

// testTxHashPH creates a []byte hash from a string for testing
func testTxHashPH(s string) []byte {
	hash := sha256.Sum256([]byte(s))
	return hash[:]
}

func mockTxDecoder(txBytes []byte) (interface{}, error) {
	return txBytes, nil
}

func TestNewProposalHandler(t *testing.T) {
	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
	})

	t.Run("Create handler with valid config", func(t *testing.T) {
		handler := NewProposalHandler(ProposalHandlerConfig{
			Book:      book,
			TxDecoder: mockTxDecoder,
			Logger:    log.NewNopLogger(),
		})
		require.NotNil(t, handler)
	})

	t.Run("Panic on nil book", func(t *testing.T) {
		require.Panics(t, func() {
			NewProposalHandler(ProposalHandlerConfig{
				Book:      nil,
				TxDecoder: mockTxDecoder,
				Logger:    log.NewNopLogger(),
			})
		})
	})

	t.Run("Panic on nil tx decoder", func(t *testing.T) {
		require.Panics(t, func() {
			NewProposalHandler(ProposalHandlerConfig{
				Book:      book,
				TxDecoder: nil,
				Logger:    log.NewNopLogger(),
			})
		})
	})
}

func TestProposalHandler_PrepareProposal(t *testing.T) {
	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"seq1": seqPubKey,
		},
	})

	handler := NewProposalHandler(ProposalHandlerConfig{
		Book:      book,
		TxDecoder: mockTxDecoder,
		Logger:    log.NewNopLogger(),
	})

	t.Run("Prepare proposal with no transactions", func(t *testing.T) {
		prepareHandler := handler.PrepareProposalHandler()

		req := &abci.RequestPrepareProposal{
			Height:     100,
			MaxTxBytes: 1000000,
		}

		resp, err := prepareHandler(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Empty(t, resp.Txs)
	})

	t.Run("Prepare proposal with sequencer transactions", func(t *testing.T) {
		// Submit some sequencer transactions
		for i := 0; i < 3; i++ {
			txHash := testTxHashPH(string(rune('a' + i)))
			signature, err := CreateSequencerSignature(txHash, uint64(i), seqPrivKey)
			require.NoError(t, err)

			err = book.SubmitSequencerTx(txHash, uint64(i), signature, "seq1")
			require.NoError(t, err)
		}

		prepareHandler := handler.PrepareProposalHandler()

		req := &abci.RequestPrepareProposal{
			Height:     101,
			MaxTxBytes: 1000000,
		}

		resp, err := prepareHandler(req)
		require.NoError(t, err)
		require.NotNil(t, resp)

		// Note: Currently returns empty because transaction bytes fetching is not implemented
		// In a real implementation, this would return the actual transaction bytes
		require.Empty(t, resp.Txs)
	})
}

func TestProposalHandler_ProcessProposal(t *testing.T) {
	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"seq1": seqPubKey,
		},
	})

	handler := NewProposalHandler(ProposalHandlerConfig{
		Book:      book,
		TxDecoder: mockTxDecoder,
		Logger:    log.NewNopLogger(),
	})

	processHandler := handler.ProcessProposalHandler()

	t.Run("Accept proposal with no transactions", func(t *testing.T) {
		req := &abci.RequestProcessProposal{
			Height: 100,
			Txs:    [][]byte{},
		}

		resp, err := processHandler(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, abci.ResponseProcessProposal_ACCEPT, resp.Status)
	})

	t.Run("Reject proposal with transaction not in execution book", func(t *testing.T) {
		txBytes := []byte("unknown transaction")

		req := &abci.RequestProcessProposal{
			Height: 100,
			Txs:    [][]byte{txBytes},
		}

		resp, err := processHandler(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, abci.ResponseProcessProposal_REJECT, resp.Status)
	})

	t.Run("Accept proposal with valid sequencer transaction", func(t *testing.T) {
		// Create and submit a sequencer transaction
		txBytes := []byte("valid transaction")
		txHash := CalculateTxHash(txBytes)

		signature, err := CreateSequencerSignature(txHash, 0, seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash, 0, signature, "seq1")
		require.NoError(t, err)

		req := &abci.RequestProcessProposal{
			Height: 100,
			Txs:    [][]byte{txBytes},
		}

		resp, err := processHandler(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, abci.ResponseProcessProposal_ACCEPT, resp.Status)
	})

	t.Run("Reject proposal with already included transaction", func(t *testing.T) {
		// Create and submit a sequencer transaction
		txBytes := []byte("already included tx")
		txHash := CalculateTxHash(txBytes)

		signature, err := CreateSequencerSignature(txHash, 1, seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash, 1, signature, "seq1")
		require.NoError(t, err)

		// Mark as included
		book.MarkIncluded([][]byte{txHash}, 99)

		req := &abci.RequestProcessProposal{
			Height: 100,
			Txs:    [][]byte{txBytes},
		}

		resp, err := processHandler(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, abci.ResponseProcessProposal_REJECT, resp.Status)
	})
}

func TestProposalHandler_OnBlockCommit(t *testing.T) {
	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"seq1": seqPubKey,
		},
	})

	handler := NewProposalHandler(ProposalHandlerConfig{
		Book:      book,
		TxDecoder: mockTxDecoder,
		Logger:    log.NewNopLogger(),
	})

	// Submit some transactions
	txHashes := [][]byte{testTxHashPH("a"), testTxHashPH("b"), testTxHashPH("c")}
	for i, txHash := range txHashes {
		signature, err := CreateSequencerSignature(txHash, uint64(i), seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash, uint64(i), signature, "seq1")
		require.NoError(t, err)
	}

	t.Run("Mark transactions as included and cleanup", func(t *testing.T) {
		// Before commit
		require.Equal(t, 3, book.GetTransactionCount())

		// Commit block with 2 transactions
		err := handler.OnBlockCommit(100, [][]byte{txHashes[0], txHashes[1]})
		require.NoError(t, err)

		// After commit and cleanup, only 1 should remain
		require.Equal(t, 1, book.GetTransactionCount())

		// Verify cleanup removed the included transactions
		_, exists := book.GetTransaction(txHashes[0])
		require.False(t, exists)

		_, exists = book.GetTransaction(txHashes[1])
		require.False(t, exists)

		_, exists = book.GetTransaction(txHashes[2])
		require.True(t, exists)
	})
}

func TestProposalHandler_ValidateSequencerTransaction(t *testing.T) {
	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
	})

	handler := NewProposalHandler(ProposalHandlerConfig{
		Book:      book,
		TxDecoder: mockTxDecoder,
		Logger:    log.NewNopLogger(),
	})

	t.Run("Validate transaction with correct hash", func(t *testing.T) {
		txBytes := []byte("test transaction")
		txHash := CalculateTxHash(txBytes)

		seqTx := &SequencerTransaction{
			TxHash:         txHash,
			SequenceNumber: 0,
		}

		err := handler.ValidateSequencerTransaction(txBytes, seqTx)
		require.NoError(t, err)
	})

	t.Run("Reject transaction with incorrect hash", func(t *testing.T) {
		txBytes := []byte("test transaction")

		seqTx := &SequencerTransaction{
			TxHash:         []byte("wrong_hash_wrong_hash_wrong_hash"),
			SequenceNumber: 0,
		}

		err := handler.ValidateSequencerTransaction(txBytes, seqTx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "hash mismatch")
	})
}

package executionbook

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/stretchr/testify/require"
)

// testTxHash creates a []byte hash from a string for testing
func testTxHash(s string) []byte {
	hash := sha256.Sum256([]byte(s))
	return hash[:]
}

func TestExecutionBook_SubmitSequencerTx(t *testing.T) {
	// Generate test sequencer keys
	seq1PrivKey := ed25519.GenPrivKey()
	seq1PubKey := seq1PrivKey.PubKey()

	seq2PrivKey := ed25519.GenPrivKey()
	seq2PubKey := seq2PrivKey.PubKey()

	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"sequencer1": seq1PubKey,
			"sequencer2": seq2PubKey,
		},
	})

	t.Run("Submit valid transaction with sequence 0", func(t *testing.T) {
		txHash := testTxHash("tx0")
		seq := uint64(0)

		signature, err := CreateSequencerSignature(txHash, seq, seq1PrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash, seq, signature, "sequencer1")
		require.NoError(t, err)

		// Verify transaction was stored
		tx, exists := book.GetTransaction(txHash)
		require.True(t, exists)
		require.True(t, bytes.Equal(txHash, tx.TxHash))
		require.Equal(t, seq, tx.SequenceNumber)
		require.Equal(t, "sequencer1", tx.SequencerID)
		require.False(t, tx.Included)
	})

	t.Run("Submit transaction with sequence 1", func(t *testing.T) {
		txHash := testTxHash("tx1")
		seq := uint64(1)

		signature, err := CreateSequencerSignature(txHash, seq, seq1PrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash, seq, signature, "sequencer1")
		require.NoError(t, err)

		// Verify next sequence is 2
		require.Equal(t, uint64(2), book.GetNextSequence())
	})

	t.Run("Reject transaction with sequence gap", func(t *testing.T) {
		txHash := testTxHash("tx_gap")
		seq := uint64(5) // Gap from 2 to 5

		signature, err := CreateSequencerSignature(txHash, seq, seq1PrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash, seq, signature, "sequencer1")
		require.Error(t, err)
		require.Contains(t, err.Error(), "sequence number mismatch")
		require.Contains(t, err.Error(), "no gaps allowed")
	})

	t.Run("Reject duplicate transaction", func(t *testing.T) {
		txHash := testTxHash("tx0") // Already submitted
		seq := uint64(2)

		signature, err := CreateSequencerSignature(txHash, seq, seq1PrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash, seq, signature, "sequencer1")
		require.Error(t, err)
		require.Contains(t, err.Error(), "already submitted")
	})

	t.Run("Reject invalid signature", func(t *testing.T) {
		txHash := testTxHash("tx_invalid")
		seq := uint64(2)

		// Sign with wrong key
		signature, err := CreateSequencerSignature(txHash, seq, seq2PrivKey)
		require.NoError(t, err)

		// Submit with sequencer1 (wrong key)
		err = book.SubmitSequencerTx(txHash, seq, signature, "sequencer1")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid sequencer signature")
	})

	t.Run("Reject unknown sequencer", func(t *testing.T) {
		txHash := testTxHash("tx_unknown")
		seq := uint64(2)

		signature, err := CreateSequencerSignature(txHash, seq, seq1PrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash, seq, signature, "unknown_sequencer")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown sequencer")
	})
}

func TestExecutionBook_GetOrderedTransactions(t *testing.T) {
	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"seq1": seqPubKey,
		},
	})

	// Submit transactions in order
	txHashes := [][]byte{testTxHash("tx0"), testTxHash("tx1"), testTxHash("tx2")}
	for i, txHash := range txHashes {
		signature, err := CreateSequencerSignature(txHash, uint64(i), seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash, uint64(i), signature, "seq1")
		require.NoError(t, err)
	}

	t.Run("Get all pending transactions in order", func(t *testing.T) {
		ordered := book.GetOrderedTransactions()
		require.Len(t, ordered, 3)

		// Verify order
		for i, tx := range ordered {
			require.True(t, bytes.Equal(txHashes[i], tx.TxHash))
			require.Equal(t, uint64(i), tx.SequenceNumber)
		}
	})

	t.Run("After marking some included, only get pending", func(t *testing.T) {
		// Mark first transaction as included
		book.MarkIncluded([][]byte{txHashes[0]}, 100)

		ordered := book.GetOrderedTransactions()
		require.Len(t, ordered, 2)
		require.True(t, bytes.Equal(txHashes[1], ordered[0].TxHash))
		require.True(t, bytes.Equal(txHashes[2], ordered[1].TxHash))
	})
}

func TestExecutionBook_MarkIncluded(t *testing.T) {
	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"seq1": seqPubKey,
		},
	})

	// Submit transactions
	txHash := testTxHash("tx0")
	signature, err := CreateSequencerSignature(txHash, 0, seqPrivKey)
	require.NoError(t, err)

	err = book.SubmitSequencerTx(txHash, 0, signature, "seq1")
	require.NoError(t, err)

	t.Run("Mark transaction as included", func(t *testing.T) {
		tx, exists := book.GetTransaction(txHash)
		require.True(t, exists)
		require.False(t, tx.Included)

		// Mark as included
		book.MarkIncluded([][]byte{txHash}, 100)

		tx, exists = book.GetTransaction(txHash)
		require.True(t, exists)
		require.True(t, tx.Included)
		require.Equal(t, uint64(100), tx.BlockHeight)
	})
}

func TestExecutionBook_CleanupIncludedTransactions(t *testing.T) {
	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"seq1": seqPubKey,
		},
	})

	// Submit 3 transactions
	txHashes := [][]byte{testTxHash("a"), testTxHash("b"), testTxHash("c")}
	for i := 0; i < 3; i++ {
		signature, err := CreateSequencerSignature(txHashes[i], uint64(i), seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHashes[i], uint64(i), signature, "seq1")
		require.NoError(t, err)
	}

	// Mark 2 as included
	book.MarkIncluded([][]byte{txHashes[0], txHashes[1]}, 100)

	t.Run("Cleanup removes included transactions", func(t *testing.T) {
		// GetTransactionCount() only counts pending (non-included) transactions
		// So even before cleanup, it should show 1 pending transaction
		require.Equal(t, 1, book.GetTransactionCount())

		// Verify transactions still exist before cleanup
		_, existsA := book.GetTransaction(txHashes[0])
		require.True(t, existsA)
		_, existsB := book.GetTransaction(txHashes[1])
		require.True(t, existsB)

		cleaned := book.CleanupIncludedTransactions()
		require.Equal(t, 2, cleaned)

		// Only 1 pending transaction should remain
		require.Equal(t, 1, book.GetTransactionCount())

		// Verify removed transactions don't exist
		_, exists := book.GetTransaction(txHashes[0])
		require.False(t, exists)

		_, exists = book.GetTransaction(txHashes[1])
		require.False(t, exists)

		// Verify pending transaction still exists
		_, exists = book.GetTransaction(txHashes[2])
		require.True(t, exists)
	})
}

func TestExecutionBook_UpdateBlockHeight(t *testing.T) {
	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
	})

	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()
	book.AddSequencer("seq1", seqPubKey)

	// Submit some transactions
	txHashes := [][]byte{testTxHash("a"), testTxHash("b"), testTxHash("c")}
	for i := 0; i < 3; i++ {
		signature, err := CreateSequencerSignature(txHashes[i], uint64(i), seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHashes[i], uint64(i), signature, "seq1")
		require.NoError(t, err)
	}

	require.Equal(t, uint64(3), book.GetNextSequence())

	t.Run("Update block height does NOT reset sequence (global sequence)", func(t *testing.T) {
		book.UpdateBlockHeight(101)

		// Sequence should NOT be reset - it's global
		require.Equal(t, uint64(3), book.GetNextSequence())

		// Next transaction should use sequence 3, not 0
		txHash := testTxHash("newblock")
		signature, err := CreateSequencerSignature(txHash, 3, seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash, 3, signature, "seq1")
		require.NoError(t, err)

		// Sequence should now be 4
		require.Equal(t, uint64(4), book.GetNextSequence())
	})
}

func TestExecutionBook_AddRemoveSequencer(t *testing.T) {
	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
	})

	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	t.Run("Add sequencer", func(t *testing.T) {
		book.AddSequencer("new_seq", seqPubKey)

		// Should be able to submit transactions
		txHash := testTxHash("tx0")
		signature, err := CreateSequencerSignature(txHash, 0, seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash, 0, signature, "new_seq")
		require.NoError(t, err)
	})

	t.Run("Remove sequencer", func(t *testing.T) {
		book.RemoveSequencer("new_seq")

		// Should not be able to submit transactions
		txHash := testTxHash("tx2")
		signature, err := CreateSequencerSignature(txHash, 1, seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash, 1, signature, "new_seq")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown sequencer")
	})
}

func TestExecutionBook_GetStats(t *testing.T) {
	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"seq1": seqPubKey,
			"seq2": seqPubKey,
		},
	})

	// Submit transactions
	txHashes := [][]byte{
		testTxHash("a"), testTxHash("b"), testTxHash("c"),
		testTxHash("d"), testTxHash("e"),
	}
	for i := 0; i < 5; i++ {
		signature, err := CreateSequencerSignature(txHashes[i], uint64(i), seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHashes[i], uint64(i), signature, "seq1")
		require.NoError(t, err)
	}

	// Mark some as included
	book.MarkIncluded([][]byte{txHashes[0], txHashes[1]}, 100)

	stats := book.GetStats()
	require.Equal(t, 5, stats.TotalTransactions)
	require.Equal(t, 3, stats.PendingTransactions)
	require.Equal(t, 2, stats.IncludedTransactions)
	require.Equal(t, uint64(5), stats.NextSequence)
	require.Equal(t, 2, stats.SequencerCount)
}

func TestExecutionBook_CalculateTxHash(t *testing.T) {
	txBytes := []byte("test transaction")

	hash1 := CalculateTxHash(txBytes)
	hash2 := CalculateTxHash(txBytes)

	// Should be deterministic
	require.Equal(t, hash1, hash2)
	require.NotEmpty(t, hash1)

	// Different bytes should produce different hash
	differentBytes := []byte("different transaction")
	hash3 := CalculateTxHash(differentBytes)
	require.NotEqual(t, hash1, hash3)
}

func TestSequencerSignature_Verification(t *testing.T) {
	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"seq1": seqPubKey,
		},
	})

	txHash := testTxHash("test")
	seq := uint64(0)

	t.Run("Valid signature", func(t *testing.T) {
		signature, err := CreateSequencerSignature(txHash, seq, seqPrivKey)
		require.NoError(t, err)

		// Verify manually
		msg := book.createSequencerMessage(txHash, seq)
		isValid := seqPubKey.VerifySignature(msg, signature)
		require.True(t, isValid)
	})

	t.Run("Invalid signature with modified data", func(t *testing.T) {
		signature, err := CreateSequencerSignature(txHash, seq, seqPrivKey)
		require.NoError(t, err)

		// Try to verify with different sequence number
		msg := book.createSequencerMessage(txHash, seq+1)
		isValid := seqPubKey.VerifySignature(msg, signature)
		require.False(t, isValid)
	})
}

package executionbook

import (
	"testing"

	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/stretchr/testify/require"
)

func TestExecutionBook_BookSizeLimit(t *testing.T) {
	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	t.Run("Unlimited book size (BookSize = 0)", func(t *testing.T) {
		book := NewExecutionBook(ExecutionBookConfig{
			Logger: log.NewNopLogger(),
			SequencerPubKeys: map[string]cryptotypes.PubKey{
				"seq1": seqPubKey,
			},
			BookSize: 0, // Unlimited
		})

		// Should be able to add many transactions
		for i := 0; i < 100; i++ {
			txHash := testTxHash(string(rune('a' + i)))
			signature, err := CreateSequencerSignature(txHash, uint64(i), seqPrivKey)
			require.NoError(t, err)

			err = book.SubmitSequencerTx(txHash, uint64(i), signature, "seq1")
			require.NoError(t, err, "Should accept transaction %d with unlimited book size", i)
		}

		require.Equal(t, 100, book.GetTransactionCount())
	})

	t.Run("Limited book size - reject when full", func(t *testing.T) {
		book := NewExecutionBook(ExecutionBookConfig{
			Logger: log.NewNopLogger(),
			SequencerPubKeys: map[string]cryptotypes.PubKey{
				"seq1": seqPubKey,
			},
			BookSize: 5, // Limit to 5 transactions
		})

		// Add 5 transactions (should succeed)
		for i := 0; i < 5; i++ {
			txHash := testTxHash(string(rune('a' + i)))
			signature, err := CreateSequencerSignature(txHash, uint64(i), seqPrivKey)
			require.NoError(t, err)

			err = book.SubmitSequencerTx(txHash, uint64(i), signature, "seq1")
			require.NoError(t, err, "Should accept transaction %d", i)
		}

		require.Equal(t, 5, book.GetTransactionCount())

		// 6th transaction should fail
		txHash := testTxHash("f")
		signature, err := CreateSequencerSignature(txHash, 5, seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash, 5, signature, "seq1")
		require.Error(t, err)
		require.Contains(t, err.Error(), "execution book is full")
		require.Contains(t, err.Error(), "5/5")
	})

	t.Run("Book size limit - accept after cleanup", func(t *testing.T) {
		book := NewExecutionBook(ExecutionBookConfig{
			Logger: log.NewNopLogger(),
			SequencerPubKeys: map[string]cryptotypes.PubKey{
				"seq1": seqPubKey,
			},
			BookSize: 3, // Limit to 3 transactions
		})

		// Add 3 transactions
		txHashes := make([][]byte, 3)
		for i := 0; i < 3; i++ {
			txHashes[i] = testTxHash(string(rune('a' + i)))
			signature, err := CreateSequencerSignature(txHashes[i], uint64(i), seqPrivKey)
			require.NoError(t, err)

			err = book.SubmitSequencerTx(txHashes[i], uint64(i), signature, "seq1")
			require.NoError(t, err)
		}

		// Mark first 2 as included
		book.MarkIncluded([][]byte{txHashes[0], txHashes[1]}, 100)

		// Now only 1 pending transaction
		require.Equal(t, 1, book.GetTransactionCount())

		// Should be able to add 2 more transactions
		for i := 3; i < 5; i++ {
			txHash := testTxHash(string(rune('a' + i)))
			signature, err := CreateSequencerSignature(txHash, uint64(i), seqPrivKey)
			require.NoError(t, err)

			err = book.SubmitSequencerTx(txHash, uint64(i), signature, "seq1")
			require.NoError(t, err, "Should accept transaction %d after some are marked included", i)
		}

		require.Equal(t, 3, book.GetTransactionCount())

		// 6th transaction should fail (3 pending)
		txHash := testTxHash("f")
		signature, err := CreateSequencerSignature(txHash, 5, seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash, 5, signature, "seq1")
		require.Error(t, err)
		require.Contains(t, err.Error(), "execution book is full")
	})

	t.Run("Book size limit - cleanup frees space", func(t *testing.T) {
		book := NewExecutionBook(ExecutionBookConfig{
			Logger: log.NewNopLogger(),
			SequencerPubKeys: map[string]cryptotypes.PubKey{
				"seq1": seqPubKey,
			},
			BookSize: 3,
		})

		// Add 3 transactions (fills the book)
		txHashes := make([][]byte, 3)
		for i := 0; i < 3; i++ {
			txHashes[i] = testTxHash(string(rune('a' + i)))
			signature, err := CreateSequencerSignature(txHashes[i], uint64(i), seqPrivKey)
			require.NoError(t, err)

			err = book.SubmitSequencerTx(txHashes[i], uint64(i), signature, "seq1")
			require.NoError(t, err)
		}

		// Mark all as included
		book.MarkIncluded(txHashes, 100)

		// Should be able to add now because included transactions don't count towards book size
		txHash := testTxHash("d")
		signature, err := CreateSequencerSignature(txHash, 3, seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash, 3, signature, "seq1")
		require.NoError(t, err, "Should accept because included txs don't count towards book size")

		// Now add 2 more to reach the limit (1 pending + 2 new = 3 pending)
		for i := 4; i < 6; i++ {
			txHash := testTxHash(string(rune('a' + i)))
			signature, err := CreateSequencerSignature(txHash, uint64(i), seqPrivKey)
			require.NoError(t, err)

			err = book.SubmitSequencerTx(txHash, uint64(i), signature, "seq1")
			require.NoError(t, err, "Should accept transaction %d", i)
		}

		require.Equal(t, 3, book.GetTransactionCount(), "Should have 3 pending transactions")

		// 7th transaction should fail (3 pending)
		txHash7 := testTxHash("g")
		signature7, err := CreateSequencerSignature(txHash7, 6, seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash7, 6, signature7, "seq1")
		require.Error(t, err)
		require.Contains(t, err.Error(), "execution book is full")

		// Cleanup included transactions
		cleaned := book.CleanupIncludedTransactions()
		require.Equal(t, 3, cleaned, "Should clean up 3 included transactions")
		require.Equal(t, 3, book.GetTransactionCount(), "Should still have 3 pending transactions after cleanup")
	})

	t.Run("BookSize = 1 - minimal book", func(t *testing.T) {
		book := NewExecutionBook(ExecutionBookConfig{
			Logger: log.NewNopLogger(),
			SequencerPubKeys: map[string]cryptotypes.PubKey{
				"seq1": seqPubKey,
			},
			BookSize: 1, // Only 1 transaction allowed
		})

		// Add 1 transaction
		txHash1 := testTxHash("a")
		signature1, err := CreateSequencerSignature(txHash1, 0, seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash1, 0, signature1, "seq1")
		require.NoError(t, err)

		// 2nd transaction should fail
		txHash2 := testTxHash("b")
		signature2, err := CreateSequencerSignature(txHash2, 1, seqPrivKey)
		require.NoError(t, err)

		err = book.SubmitSequencerTx(txHash2, 1, signature2, "seq1")
		require.Error(t, err)
		require.Contains(t, err.Error(), "1/1")

		// Mark as included and cleanup
		book.MarkIncluded([][]byte{txHash1}, 100)
		book.CleanupIncludedTransactions()

		// Now can add the 2nd transaction
		err = book.SubmitSequencerTx(txHash2, 1, signature2, "seq1")
		require.NoError(t, err)
	})
}

func TestExecutionBook_BookSizeConfig(t *testing.T) {
	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	t.Run("Negative book size treated as unlimited", func(t *testing.T) {
		book := NewExecutionBook(ExecutionBookConfig{
			Logger: log.NewNopLogger(),
			SequencerPubKeys: map[string]cryptotypes.PubKey{
				"seq1": seqPubKey,
			},
			BookSize: -1, // Should be treated as unlimited
		})

		// Should accept many transactions
		for i := 0; i < 10; i++ {
			txHash := testTxHash(string(rune('a' + i)))
			signature, err := CreateSequencerSignature(txHash, uint64(i), seqPrivKey)
			require.NoError(t, err)

			err = book.SubmitSequencerTx(txHash, uint64(i), signature, "seq1")
			require.NoError(t, err)
		}

		require.Equal(t, 10, book.GetTransactionCount())
	})
}

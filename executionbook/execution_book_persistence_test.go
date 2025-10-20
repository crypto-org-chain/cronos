package executionbook

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/stretchr/testify/require"
)

func TestExecutionBook_Persistence(t *testing.T) {
	// Create temp directory for state file
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "execution_book_state.json")

	// Generate test keys
	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	// Create first ExecutionBook instance
	book1 := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"seq1": seqPubKey,
		},
		StateFilePath: stateFile,
	})

	// Submit some transactions
	t.Run("Submit transactions and save state", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			txHash := testTxHash(string(rune('a' + i)))
			signature, err := CreateSequencerSignature(txHash, uint64(i), seqPrivKey)
			require.NoError(t, err)

			err = book1.SubmitSequencerTx(txHash, uint64(i), signature, "seq1")
			require.NoError(t, err)
		}

		// Wait for async save to complete
		time.Sleep(100 * time.Millisecond)

		// Verify state file exists
		_, err := os.Stat(stateFile)
		require.NoError(t, err, "State file should exist")
	})

	t.Run("Update block height and save", func(t *testing.T) {
		book1.UpdateBlockHeight(100)
		time.Sleep(100 * time.Millisecond)
	})

	// Create second ExecutionBook instance to test recovery
	t.Run("Recover state from file", func(t *testing.T) {
		book2 := NewExecutionBook(ExecutionBookConfig{
			Logger: log.NewNopLogger(),
			SequencerPubKeys: map[string]cryptotypes.PubKey{
				"seq1": seqPubKey,
			},
			StateFilePath: stateFile,
		})

		// Verify recovered state
		require.Equal(t, uint64(3), book2.GetNextSequence(), "Next sequence should be recovered")
		require.Equal(t, 3, book2.GetTransactionCount(), "Transaction count should be recovered")

		stats := book2.GetStats()
		require.Equal(t, uint64(100), stats.CurrentBlockHeight, "Block height should be recovered")
		require.Equal(t, 3, stats.TotalTransactions, "Total transactions should be recovered")

		// Verify transactions are in correct order
		orderedTxs := book2.GetOrderedTransactions()
		require.Len(t, orderedTxs, 3)
		for i := 0; i < 3; i++ {
			require.Equal(t, uint64(i), orderedTxs[i].SequenceNumber)
		}
	})

	t.Run("Continue from recovered state", func(t *testing.T) {
		book2 := NewExecutionBook(ExecutionBookConfig{
			Logger: log.NewNopLogger(),
			SequencerPubKeys: map[string]cryptotypes.PubKey{
				"seq1": seqPubKey,
			},
			StateFilePath: stateFile,
		})

		// Submit next transaction with correct sequence
		txHash := testTxHash("d")
		signature, err := CreateSequencerSignature(txHash, 3, seqPrivKey)
		require.NoError(t, err)

		err = book2.SubmitSequencerTx(txHash, 3, signature, "seq1")
		require.NoError(t, err, "Should accept next sequence number")

		require.Equal(t, uint64(4), book2.GetNextSequence())
	})
}

func TestExecutionBook_PersistenceWithCleanup(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "execution_book_state.json")

	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	book1 := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"seq1": seqPubKey,
		},
		StateFilePath: stateFile,
	})

	// Submit transactions
	txHashes := [][]byte{testTxHash("a"), testTxHash("b"), testTxHash("c")}
	for i, txHash := range txHashes {
		signature, err := CreateSequencerSignature(txHash, uint64(i), seqPrivKey)
		require.NoError(t, err)

		err = book1.SubmitSequencerTx(txHash, uint64(i), signature, "seq1")
		require.NoError(t, err)
	}

	// Mark some as included
	book1.MarkIncluded([][]byte{txHashes[0], txHashes[1]}, 100)

	// Cleanup included transactions
	cleaned := book1.CleanupIncludedTransactions()
	require.Equal(t, 2, cleaned)

	// Wait for async save
	time.Sleep(100 * time.Millisecond)

	// Recover and verify only pending transaction is present
	book2 := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"seq1": seqPubKey,
		},
		StateFilePath: stateFile,
	})

	require.Equal(t, 1, book2.GetTransactionCount(), "Only one pending transaction should remain")
	require.Equal(t, uint64(3), book2.GetNextSequence(), "Sequence should still be 3")

	// Verify the remaining transaction
	orderedTxs := book2.GetOrderedTransactions()
	require.Len(t, orderedTxs, 1)
	require.Equal(t, uint64(2), orderedTxs[0].SequenceNumber)
}

func TestExecutionBook_NoPersistence(t *testing.T) {
	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	// Create ExecutionBook without state file
	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"seq1": seqPubKey,
		},
		// No StateFilePath
	})

	// Submit transaction
	txHash := testTxHash("a")
	signature, err := CreateSequencerSignature(txHash, 0, seqPrivKey)
	require.NoError(t, err)

	err = book.SubmitSequencerTx(txHash, 0, signature, "seq1")
	require.NoError(t, err)

	// SaveState should not error even without file path
	err = book.SaveState()
	require.NoError(t, err, "SaveState should succeed (no-op) without file path")

	// LoadState should not error even without file path
	err = book.LoadState()
	require.NoError(t, err, "LoadState should succeed (no-op) without file path")
}

func TestExecutionBook_CorruptedStateFile(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "execution_book_state.json")

	// Write corrupted JSON
	err := os.WriteFile(stateFile, []byte("invalid json {[}"), 0644)
	require.NoError(t, err)

	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	// Should start fresh when state file is corrupted
	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"seq1": seqPubKey,
		},
		StateFilePath: stateFile,
	})

	// Should start from sequence 0
	require.Equal(t, uint64(0), book.GetNextSequence())
	require.Equal(t, 0, book.GetTransactionCount())
}

func TestExecutionBook_StateFileAtomicity(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "execution_book_state.json")

	seqPrivKey := ed25519.GenPrivKey()
	seqPubKey := seqPrivKey.PubKey()

	book := NewExecutionBook(ExecutionBookConfig{
		Logger: log.NewNopLogger(),
		SequencerPubKeys: map[string]cryptotypes.PubKey{
			"seq1": seqPubKey,
		},
		StateFilePath: stateFile,
	})

	// Submit transaction
	txHash := testTxHash("a")
	signature, err := CreateSequencerSignature(txHash, 0, seqPrivKey)
	require.NoError(t, err)

	err = book.SubmitSequencerTx(txHash, 0, signature, "seq1")
	require.NoError(t, err)

	// Wait for save
	time.Sleep(100 * time.Millisecond)

	// Temp file should not exist after successful save
	tempFile := stateFile + ".tmp"
	_, err = os.Stat(tempFile)
	require.True(t, os.IsNotExist(err), "Temp file should not exist after save")

	// State file should exist
	_, err = os.Stat(stateFile)
	require.NoError(t, err, "State file should exist")
}

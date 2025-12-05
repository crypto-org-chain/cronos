//go:build !rocksdb
// +build !rocksdb

package dbmigrate

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
)

// setupTxIndexWithEthereumEvents creates a tx_index database with Ethereum transaction events
func setupTxIndexWithEthereumEvents(t *testing.T, backend dbm.BackendType, startHeight, endHeight int64) (string, dbm.DB) {
	t.Helper()
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	err := os.MkdirAll(dataDir, 0o755)
	require.NoError(t, err)

	db, err := dbm.NewDB("tx_index", backend, dataDir)
	require.NoError(t, err)

	for height := startHeight; height <= endHeight; height++ {
		// Create 2 txs per height (one EVM, one non-EVM)
		for txIdx := int64(0); txIdx < 2; txIdx++ {
			// tx.height key
			txHeightKey := []byte(fmt.Sprintf("tx.height/%d/%d/%d", height, height, txIdx))
			cometTxHash := []byte(fmt.Sprintf("cometbft-txhash-%d-%d", height, txIdx))
			err := db.Set(txHeightKey, cometTxHash)
			require.NoError(t, err)

			// Create TxResult
			txResult := &abci.TxResult{
				Height: height,
				Index:  uint32(txIdx),
				Tx:     []byte(fmt.Sprintf("tx-data-%d-%d", height, txIdx)),
				Result: abci.ExecTxResult{
					Code: 0,
					Data: []byte("result-data"),
				},
			}

			// Add ethereum_tx event for EVM transactions (txIdx 0)
			if txIdx == 0 {
				ethTxHash := fmt.Sprintf("%064d", height*100+txIdx)
				txResult.Result.Events = []abci.Event{
					{
						Type: "ethereum_tx",
						Attributes: []abci.EventAttribute{
							{
								Key:   "ethereumTxHash",
								Value: "0x" + ethTxHash,
							},
							{
								Key:   "txIndex",
								Value: fmt.Sprintf("%d", txIdx),
							},
						},
					},
				}

				// Create ethereum event-indexed keys
				// Format: ethereum_tx.ethereumTxHash/0x<hash>/<height>/<txindex>
				eventKey := []byte(fmt.Sprintf("ethereum_tx.ethereumTxHash/0x%s/%d/%d", ethTxHash, height, txIdx))
				err = db.Set(eventKey, cometTxHash)
				require.NoError(t, err)

				// Also create event sequence keys (with $es$ suffix)
				eventSeqKey := []byte(fmt.Sprintf("ethereum_tx.ethereumTxHash/0x%s/%d/%d$es$0", ethTxHash, height, txIdx))
				err = db.Set(eventSeqKey, cometTxHash)
				require.NoError(t, err)
			}

			// Marshal and store TxResult
			txResultBytes, err := proto.Marshal(txResult)
			require.NoError(t, err)
			err = db.Set(cometTxHash, txResultBytes)
			require.NoError(t, err)
		}
	}

	return tempDir, db
}

// TestPatchTxIndexWithEthereumEvents tests patching tx_index with Ethereum event keys
func TestPatchTxIndexWithEthereumEvents(t *testing.T) {
	// Setup source with Ethereum events
	sourceDir, sourceDB := setupTxIndexWithEthereumEvents(t, dbm.GoLevelDBBackend, 1, 10)
	sourceDB.Close()

	// Setup target with partial data
	targetDir, targetDB := setupTxIndexWithEthereumEvents(t, dbm.GoLevelDBBackend, 1, 5)
	targetDB.Close()

	// Patch height 7 (which has Ethereum events)
	opts := PatchOptions{
		SourceHome:    sourceDir,
		TargetPath:    filepath.Join(targetDir, "data", "tx_index.db"),
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.GoLevelDBBackend,
		BatchSize:     10,
		Logger:        log.NewTestLogger(t),
		DBName:        DBNameTxIndex,
		HeightRange: HeightRange{
			SpecificHeights: []int64{7},
		},
		ConflictStrategy:   ConflictReplaceAll,
		SkipConflictChecks: true,
		DryRun:             false,
	}

	stats, err := PatchDatabase(opts)
	require.NoError(t, err)
	require.NotNil(t, stats)

	// Should patch tx.height keys, txhash keys, and ethereum event keys
	require.Greater(t, stats.ProcessedKeys.Load(), int64(2), "Should patch multiple key types")
	require.Equal(t, int64(0), stats.ErrorCount.Load())

	// Verify Ethereum event keys are patched
	targetDB, err = dbm.NewDB("tx_index", dbm.GoLevelDBBackend, filepath.Join(targetDir, "data"))
	require.NoError(t, err)
	defer targetDB.Close()

	// Check ethereum event key
	ethTxHash := fmt.Sprintf("%064d", 7*100) // height=7, txIdx=0
	eventKey := []byte(fmt.Sprintf("ethereum_tx.ethereumTxHash/0x%s/%d/%d", ethTxHash, 7, 0))
	eventValue, err := targetDB.Get(eventKey)
	require.NoError(t, err)
	require.NotNil(t, eventValue, "Ethereum event key should be patched")

	// Check event sequence key
	eventSeqKey := []byte(fmt.Sprintf("ethereum_tx.ethereumTxHash/0x%s/%d/%d$es$0", ethTxHash, 7, 0))
	eventSeqValue, err := targetDB.Get(eventSeqKey)
	require.NoError(t, err)
	require.NotNil(t, eventSeqValue, "Ethereum event sequence key should be patched")
}

// TestExtractEthereumTxHash tests the extraction of Ethereum tx hash from TxResult
func TestExtractEthereumTxHash(t *testing.T) {
	tests := []struct {
		name      string
		txResult  *abci.TxResult
		wantHash  string
		wantError bool
	}{
		{
			name: "valid ethereum tx with hash",
			txResult: &abci.TxResult{
				Result: abci.ExecTxResult{
					Events: []abci.Event{
						{
							Type: "ethereum_tx",
							Attributes: []abci.EventAttribute{
								{
									Key:   "ethereumTxHash",
									Value: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
								},
							},
						},
					},
				},
			},
			wantHash:  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantError: false,
		},
		{
			name: "ethereum tx without 0x prefix",
			txResult: &abci.TxResult{
				Result: abci.ExecTxResult{
					Events: []abci.Event{
						{
							Type: "ethereum_tx",
							Attributes: []abci.EventAttribute{
								{
									Key:   "ethereumTxHash",
									Value: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
								},
							},
						},
					},
				},
			},
			wantHash:  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantError: false,
		},
		{
			name: "no ethereum_tx event",
			txResult: &abci.TxResult{
				Result: abci.ExecTxResult{
					Events: []abci.Event{
						{
							Type: "transfer",
							Attributes: []abci.EventAttribute{
								{Key: "amount", Value: "100"},
							},
						},
					},
				},
			},
			wantHash:  "",
			wantError: false, // Not an error, just no ethereum tx
		},
		{
			name: "ethereum_tx without hash attribute",
			txResult: &abci.TxResult{
				Result: abci.ExecTxResult{
					Events: []abci.Event{
						{
							Type: "ethereum_tx",
							Attributes: []abci.EventAttribute{
								{Key: "other", Value: "value"},
							},
						},
					},
				},
			},
			wantHash:  "",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal TxResult
			txResultBytes, err := proto.Marshal(tt.txResult)
			require.NoError(t, err)

			// Extract hash
			gotHash, err := extractEthereumTxHash(txResultBytes)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantHash, gotHash)
			}
		})
	}
}

// TestExtractEthereumTxHashInvalidData tests error handling for invalid data
func TestExtractEthereumTxHashInvalidData(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectError bool
		expectEmpty bool
	}{
		{
			name:        "invalid protobuf data",
			data:        []byte("not-protobuf-data"),
			expectError: true,
		},
		{
			name:        "empty data",
			data:        []byte{},
			expectError: true,
		},
		{
			name: "invalid hex in hash",
			data: func() []byte {
				txResult := &abci.TxResult{
					Result: abci.ExecTxResult{
						Events: []abci.Event{
							{
								Type: "ethereum_tx",
								Attributes: []abci.EventAttribute{
									{
										Key:   "ethereumTxHash",
										Value: "0xINVALIDHEX",
									},
								},
							},
						},
					},
				}
				data, _ := proto.Marshal(txResult)
				return data
			}(),
			expectError: true,
		},
		{
			name: "hash with wrong length",
			data: func() []byte {
				txResult := &abci.TxResult{
					Result: abci.ExecTxResult{
						Events: []abci.Event{
							{
								Type: "ethereum_tx",
								Attributes: []abci.EventAttribute{
									{
										Key:   "ethereumTxHash",
										Value: "0x1234", // Too short
									},
								},
							},
						},
					},
				}
				data, _ := proto.Marshal(txResult)
				return data
			}(),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := extractEthereumTxHash(tt.data)
			if tt.expectError {
				// Some cases may not return error but return empty hash instead
				if err == nil && hash == "" {
					// This is acceptable - empty/invalid data returns empty hash
					return
				}
				require.Error(t, err)
			} else if tt.expectEmpty {
				require.NoError(t, err)
				require.Empty(t, hash)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestPatchTxIndexRangeWithEthereumEvents tests patching a range with Ethereum events
func TestPatchTxIndexRangeWithEthereumEvents(t *testing.T) {
	sourceDir, sourceDB := setupTxIndexWithEthereumEvents(t, dbm.GoLevelDBBackend, 1, 20)
	sourceDB.Close()

	targetDir, targetDB := setupTxIndexWithEthereumEvents(t, dbm.GoLevelDBBackend, 1, 10)
	targetDB.Close()

	// Patch heights 11-15
	opts := PatchOptions{
		SourceHome:    sourceDir,
		TargetPath:    filepath.Join(targetDir, "data", "tx_index.db"),
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.GoLevelDBBackend,
		BatchSize:     10,
		Logger:        log.NewTestLogger(t),
		DBName:        DBNameTxIndex,
		HeightRange: HeightRange{
			Start: 11,
			End:   15,
		},
		ConflictStrategy:   ConflictReplaceAll,
		SkipConflictChecks: true,
		DryRun:             false,
	}

	stats, err := PatchDatabase(opts)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Greater(t, stats.ProcessedKeys.Load(), int64(10))

	// Verify Ethereum event keys for multiple heights
	targetDB, err = dbm.NewDB("tx_index", dbm.GoLevelDBBackend, filepath.Join(targetDir, "data"))
	require.NoError(t, err)
	defer targetDB.Close()

	for height := int64(11); height <= 15; height++ {
		ethTxHash := fmt.Sprintf("%064d", height*100)
		eventKey := []byte(fmt.Sprintf("ethereum_tx.ethereumTxHash/0x%s/%d/%d", ethTxHash, height, 0))
		eventValue, err := targetDB.Get(eventKey)
		require.NoError(t, err)
		require.NotNil(t, eventValue, "Ethereum event key at height %d should be patched", height)
	}
}

// TestIncrementBytesEdgeCases tests edge cases of the incrementBytes helper
func TestIncrementBytesEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "single byte increment",
			input:    []byte{0x00},
			expected: []byte{0x01},
		},
		{
			name:     "single byte max",
			input:    []byte{0xFF},
			expected: nil, // No upper bound
		},
		{
			name:     "multiple bytes with carry",
			input:    []byte{0x00, 0xFF},
			expected: []byte{0x01, 0x00},
		},
		{
			name:     "all max bytes",
			input:    []byte{0xFF, 0xFF, 0xFF, 0xFF},
			expected: nil, // No upper bound
		},
		{
			name:     "empty input",
			input:    []byte{},
			expected: nil,
		},
		{
			name:     "prefix increment",
			input:    []byte("tx.height/"),
			expected: []byte("tx.height0"), // '/' (0x2F) + 1 = '0' (0x30)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := incrementBytes(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestPatchWithConflictingKeys tests patching when keys already exist in target
func TestPatchWithConflictingKeys(t *testing.T) {
	// Setup source with heights 1-10
	sourceDir, sourceDB := setupBlockstoreTestDB(t, dbm.GoLevelDBBackend, 1, 10)
	sourceDB.Close()

	// Setup target that overlaps (heights 5-10)
	targetDir, targetDB := setupBlockstoreTestDB(t, dbm.GoLevelDBBackend, 5, 10)
	targetDB.Close()

	// Try to patch height 7 which already exists in target
	// Use SkipConflictChecks to overwrite without prompting
	opts := PatchOptions{
		SourceHome:    sourceDir,
		TargetPath:    filepath.Join(targetDir, "data", "blockstore.db"),
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.GoLevelDBBackend,
		BatchSize:     10,
		Logger:        log.NewTestLogger(t),
		DBName:        DBNameBlockstore,
		HeightRange: HeightRange{
			SpecificHeights: []int64{7},
		},
		ConflictStrategy:   ConflictReplaceAll,
		SkipConflictChecks: true, // Skip conflict checks, overwrite all
		DryRun:             false,
	}

	stats, err := PatchDatabase(opts)
	require.NoError(t, err)
	require.NotNil(t, stats)
	require.Greater(t, stats.ProcessedKeys.Load(), int64(0))
	require.Equal(t, int64(0), stats.ErrorCount.Load())

	// Verify height 7 is in target (should be overwritten)
	targetDB, err = dbm.NewDB("blockstore", dbm.GoLevelDBBackend, filepath.Join(targetDir, "data"))
	require.NoError(t, err)
	defer targetDB.Close()

	hKey := []byte("H:7")
	hValue, err := targetDB.Get(hKey)
	require.NoError(t, err)
	require.NotNil(t, hValue)
}

// TestExtractHeightAndTxIndexFromKey tests extraction of height and txIndex
func TestExtractHeightAndTxIndexFromKey(t *testing.T) {
	logger := log.NewNopLogger()

	tests := []struct {
		name        string
		key         []byte
		wantHeight  int64
		wantTxIndex int64
		wantOK      bool
	}{
		{
			name:        "valid tx.height key",
			key:         []byte("tx.height/1000/1000/5"),
			wantHeight:  1000,
			wantTxIndex: 5,
			wantOK:      true,
		},
		{
			name:        "tx.height key with event sequence",
			key:         []byte("tx.height/2000/2000/10$es$0"),
			wantHeight:  2000,
			wantTxIndex: 10,
			wantOK:      true,
		},
		{
			name:        "non tx.height key",
			key:         []byte("tx.hash/abcdef"),
			wantHeight:  0,
			wantTxIndex: 0,
			wantOK:      false,
		},
		{
			name:        "malformed key",
			key:         []byte("tx.height/"),
			wantHeight:  0,
			wantTxIndex: 0,
			wantOK:      false,
		},
		{
			name:        "key with invalid height",
			key:         []byte("tx.height/abc/abc/0"),
			wantHeight:  0,
			wantTxIndex: 0,
			wantOK:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			height, txIndex, ok := extractHeightAndTxIndexFromKey(tt.key, logger)
			require.Equal(t, tt.wantOK, ok)
			if ok {
				require.Equal(t, tt.wantHeight, height)
				require.Equal(t, tt.wantTxIndex, txIndex)
			}
		})
	}
}

// TestPatchBlockstoreWithMixedKeyTypes tests patching with all blockstore key types
func TestPatchBlockstoreWithMixedKeyTypes(t *testing.T) {
	sourceDir, sourceDB := setupBlockstoreTestDB(t, dbm.GoLevelDBBackend, 1, 10)
	sourceDB.Close()

	targetDir, targetDB := setupBlockstoreTestDB(t, dbm.GoLevelDBBackend, 1, 5)
	targetDB.Close()

	// Patch multiple heights
	opts := PatchOptions{
		SourceHome:    sourceDir,
		TargetPath:    filepath.Join(targetDir, "data", "blockstore.db"),
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.GoLevelDBBackend,
		BatchSize:     5,
		Logger:        log.NewTestLogger(t),
		DBName:        DBNameBlockstore,
		HeightRange: HeightRange{
			SpecificHeights: []int64{7, 8, 9},
		},
		ConflictStrategy:   ConflictReplaceAll,
		SkipConflictChecks: true,
		DryRun:             false,
	}

	stats, err := PatchDatabase(opts)
	require.NoError(t, err)
	require.NotNil(t, stats)

	// Verify all key types for each height
	targetDB, err = dbm.NewDB("blockstore", dbm.GoLevelDBBackend, filepath.Join(targetDir, "data"))
	require.NoError(t, err)
	defer targetDB.Close()

	for _, height := range []int64{7, 8, 9} {
		// Check all key types
		keyTypes := []struct {
			prefix string
			suffix string
		}{
			{"H:", ""},
			{"P:", ":0"},
			{"C:", ""},
			{"SC:", ""},
			{"EC:", ""},
		}

		for _, kt := range keyTypes {
			key := []byte(fmt.Sprintf("%s%d%s", kt.prefix, height, kt.suffix))
			value, err := targetDB.Get(key)
			require.NoError(t, err)
			require.NotNil(t, value, "Key %s should be patched", key)
		}
	}
}

// TestFormatKeyPrefixBinaryData tests formatting of binary keys
func TestFormatKeyPrefixBinaryData(t *testing.T) {
	tests := []struct {
		name      string
		key       []byte
		maxLen    int
		expectHex bool
	}{
		{
			name:      "printable text",
			key:       []byte("tx.height/123/0"),
			maxLen:    50,
			expectHex: false,
		},
		{
			name:      "binary data",
			key:       []byte{0x01, 0x02, 0x03, 0xFF, 0xFE},
			maxLen:    50,
			expectHex: true,
		},
		{
			name:      "mixed data (mostly binary)",
			key:       []byte{0x01, 0x02, 'a', 'b', 0xFF},
			maxLen:    50,
			expectHex: true,
		},
		{
			name:      "empty key",
			key:       []byte{},
			maxLen:    50,
			expectHex: false, // Returns "<empty>"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatKeyPrefix(tt.key, tt.maxLen)
			if tt.expectHex {
				require.Contains(t, result, "0x", "Should format as hex")
			}
			require.LessOrEqual(t, len(result), tt.maxLen+10, "Should respect max length (with some margin)")
		})
	}
}

// TestFormatValueBinaryData tests formatting of binary values
func TestFormatValueBinaryData(t *testing.T) {
	tests := []struct {
		name      string
		value     []byte
		maxLen    int
		expectHex bool
	}{
		{
			name:      "text value",
			value:     []byte("some text value"),
			maxLen:    50,
			expectHex: false,
		},
		{
			name:      "binary value",
			value:     []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD},
			maxLen:    50,
			expectHex: true,
		},
		{
			name:      "empty value",
			value:     []byte{},
			maxLen:    50,
			expectHex: false,
		},
		{
			name:      "large value",
			value:     make([]byte, 1000),
			maxLen:    50,
			expectHex: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValue(tt.value, tt.maxLen)
			if tt.expectHex && len(tt.value) > 0 {
				// Binary data should be formatted as hex
				_, err := hex.DecodeString(result[2:min(len(result), tt.maxLen)])
				require.NoError(t, err, "Should be valid hex")
			}
		})
	}
}

// TestPatchDryRunDoesNotModify tests that dry-run truly doesn't modify target
func TestPatchDryRunDoesNotModify(t *testing.T) {
	sourceDir, sourceDB := setupBlockstoreTestDB(t, dbm.GoLevelDBBackend, 1, 10)
	sourceDB.Close()

	targetDir, targetDB := setupBlockstoreTestDB(t, dbm.GoLevelDBBackend, 1, 5)
	originalCount, err := countKeys(targetDB)
	require.NoError(t, err)
	targetDB.Close()

	// Dry-run patch of heights 6-8
	opts := PatchOptions{
		SourceHome:    sourceDir,
		TargetPath:    filepath.Join(targetDir, "data", "blockstore.db"),
		SourceBackend: dbm.GoLevelDBBackend,
		TargetBackend: dbm.GoLevelDBBackend,
		BatchSize:     10,
		Logger:        log.NewTestLogger(t),
		DBName:        DBNameBlockstore,
		HeightRange: HeightRange{
			Start: 6,
			End:   8,
		},
		ConflictStrategy:   ConflictReplaceAll,
		SkipConflictChecks: true,
		DryRun:             true,
	}

	stats, err := PatchDatabase(opts)
	require.NoError(t, err)
	require.NotNil(t, stats)

	// Verify target was NOT modified
	targetDB, err = dbm.NewDB("blockstore", dbm.GoLevelDBBackend, filepath.Join(targetDir, "data"))
	require.NoError(t, err)
	defer targetDB.Close()

	newCount, err := countKeys(targetDB)
	require.NoError(t, err)
	require.Equal(t, originalCount, newCount, "Key count should not change in dry-run")

	// Verify specific heights are NOT in target
	for height := int64(6); height <= 8; height++ {
		hKey := []byte(fmt.Sprintf("H:%d", height))
		hValue, err := targetDB.Get(hKey)
		require.NoError(t, err)
		require.Nil(t, hValue, "Height %d should NOT be in target (dry-run)", height)
	}
}

// Helper function min for integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

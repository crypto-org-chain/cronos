package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseBackendType tests the backend type parsing function
func TestParseBackendType(t *testing.T) {
	tests := []struct {
		name        string
		input       BackendType
		expectError bool
	}{
		{
			name:        "goleveldb",
			input:       GoLevelDB,
			expectError: false,
		},
		{
			name:        "leveldb alias",
			input:       LevelDB,
			expectError: false,
		},
		{
			name:        "rocksdb",
			input:       RocksDB,
			expectError: false,
		},
		{
			name:        "invalid backend",
			input:       "invaliddb",
			expectError: true,
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseBackendType(tt.input)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, result)
			}
		})
	}
}

// TestValidDatabaseNames tests that all expected database names are valid
func TestValidDatabaseNames(t *testing.T) {
	expectedDatabases := []string{
		"application",
		"blockstore",
		"state",
		"tx_index",
		"evidence",
	}

	for _, dbName := range expectedDatabases {
		t.Run(dbName, func(t *testing.T) {
			require.True(t, validDatabaseNames[dbName], "database %s should be valid", dbName)
		})
	}

	// Test invalid names
	invalidNames := []string{
		"invalid",
		"app",
		"cometbft",
		"",
		"application.db",
		"blockstore_db",
	}

	for _, dbName := range invalidNames {
		t.Run("invalid_"+dbName, func(t *testing.T) {
			require.False(t, validDatabaseNames[dbName], "database %s should be invalid", dbName)
		})
	}
}

// TestDatabaseNameParsing tests parsing of comma-separated database names
func TestDatabaseNameParsing(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedDBs    []string
		expectError    bool
		errorSubstring string
	}{
		{
			name:        "single database",
			input:       "application",
			expectedDBs: []string{"application"},
			expectError: false,
		},
		{
			name:        "two databases",
			input:       "blockstore,tx_index",
			expectedDBs: []string{"blockstore", "tx_index"},
			expectError: false,
		},
		{
			name:        "all databases",
			input:       "application,blockstore,state,tx_index,evidence",
			expectedDBs: []string{"application", "blockstore", "state", "tx_index", "evidence"},
			expectError: false,
		},
		{
			name:        "with spaces",
			input:       "blockstore, tx_index, state",
			expectedDBs: []string{"blockstore", "tx_index", "state"},
			expectError: false,
		},
		{
			name:        "with extra spaces",
			input:       "  application  ,  blockstore  ",
			expectedDBs: []string{"application", "blockstore"},
			expectError: false,
		},
		{
			name:           "invalid database name",
			input:          "application,invalid_db,blockstore",
			expectError:    true,
			errorSubstring: "invalid database name",
		},
		{
			name:           "only invalid database",
			input:          "invalid_db",
			expectError:    true,
			errorSubstring: "invalid database name",
		},
		{
			name:        "empty after trimming",
			input:       "application,,blockstore",
			expectedDBs: []string{"application", "blockstore"},
			expectError: false,
		},
		{
			name:           "only empty strings",
			input:          ",,,",
			expectError:    true,
			errorSubstring: "no valid databases specified",
		},
		{
			name:           "empty string",
			input:          "",
			expectError:    true,
			errorSubstring: "no databases specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbNames, err := parseDatabaseNames(tt.input)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorSubstring != "" {
					require.Contains(t, err.Error(), tt.errorSubstring)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedDBs, dbNames)
			}
		})
	}
}

// TestBackendTypeConstants tests the backend type constant values
func TestBackendTypeConstants(t *testing.T) {
	require.Equal(t, BackendType("goleveldb"), GoLevelDB)
	require.Equal(t, BackendType("leveldb"), LevelDB)
	require.Equal(t, BackendType("rocksdb"), RocksDB)
}

// TestDatabaseNameConstants tests the database name constant values
func TestDatabaseNameConstants(t *testing.T) {
	require.Equal(t, DatabaseName("application"), Application)
	require.Equal(t, DatabaseName("blockstore"), Blockstore)
	require.Equal(t, DatabaseName("state"), State)
	require.Equal(t, DatabaseName("tx_index"), TxIndex)
	require.Equal(t, DatabaseName("evidence"), Evidence)
}

// TestDBTypeConstants tests the db-type constant values
func TestDBTypeConstants(t *testing.T) {
	require.Equal(t, DbType("app"), App)
	require.Equal(t, DbType("cometbft"), CometBFT)
	require.Equal(t, DbType("all"), All)
}

// TestDBTypeMapping tests the mapping of db-type to database names
func TestDBTypeMapping(t *testing.T) {
	tests := []struct {
		name           string
		dbType         DbType
		expectedDBs    []DatabaseName
		expectError    bool
		errorSubstring string
	}{
		{
			name:        "app type",
			dbType:      App,
			expectedDBs: []DatabaseName{Application},
			expectError: false,
		},
		{
			name:        "cometbft type",
			dbType:      CometBFT,
			expectedDBs: []DatabaseName{Blockstore, State, TxIndex, Evidence},
			expectError: false,
		},
		{
			name:        "all type",
			dbType:      All,
			expectedDBs: []DatabaseName{Application, Blockstore, State, TxIndex, Evidence},
			expectError: false,
		},
		{
			name:           "invalid type",
			dbType:         "invalid",
			expectError:    true,
			errorSubstring: "invalid db-type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbNames, err := getDBNamesFromType(tt.dbType)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorSubstring != "" {
					require.Contains(t, err.Error(), tt.errorSubstring)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedDBs, dbNames)
			}
		})
	}
}

// TestDatabasesFlagPrecedence tests that --databases flag takes precedence over --db-type
func TestDatabasesFlagPrecedence(t *testing.T) {
	tests := []struct {
		name          string
		databasesFlag string
		dbTypeFlag    DbType
		expectedDBs   []string
		useDatabases  bool
	}{
		{
			name:          "only db-type",
			databasesFlag: "",
			dbTypeFlag:    App,
			expectedDBs:   []string{"application"},
			useDatabases:  false,
		},
		{
			name:          "only databases",
			databasesFlag: "blockstore,tx_index",
			dbTypeFlag:    App,
			expectedDBs:   []string{"blockstore", "tx_index"},
			useDatabases:  true,
		},
		{
			name:          "both flags - databases takes precedence",
			databasesFlag: "state",
			dbTypeFlag:    All,
			expectedDBs:   []string{"state"},
			useDatabases:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dbNamesStr []string
			var err error

			// Use the same logic as the command
			if tt.databasesFlag != "" {
				dbNamesStr, err = parseDatabaseNames(tt.databasesFlag)
				require.NoError(t, err)
			} else {
				dbNames, err := getDBNamesFromType(tt.dbTypeFlag)
				require.NoError(t, err)
				// Convert []DatabaseName to []string
				dbNamesStr = make([]string, len(dbNames))
				for i, name := range dbNames {
					dbNamesStr[i] = string(name)
				}
			}

			require.Equal(t, tt.expectedDBs, dbNamesStr)
			require.Equal(t, tt.useDatabases, tt.databasesFlag != "")
		})
	}
}

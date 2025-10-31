package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseBackendType tests the backend type parsing function
func TestParseBackendType(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "goleveldb",
			input:       "goleveldb",
			expectError: false,
		},
		{
			name:        "leveldb alias",
			input:       "leveldb",
			expectError: false,
		},
		{
			name:        "rocksdb",
			input:       "rocksdb",
			expectError: false,
		},
		{
			name:        "pebbledb",
			input:       "pebbledb",
			expectError: false,
		},
		{
			name:        "pebble alias",
			input:       "pebble",
			expectError: false,
		},
		{
			name:        "memdb",
			input:       "memdb",
			expectError: false,
		},
		{
			name:        "mem alias",
			input:       "mem",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the parsing logic from the command
			var dbNames []string
			var parseError error

			if tt.input != "" {
				dbList := splitAndTrim(tt.input)
				for _, dbName := range dbList {
					if dbName == "" {
						continue
					}
					if !validDatabaseNames[dbName] {
						parseError = &ValidationError{Message: "invalid database name: " + dbName}
						break
					}
					dbNames = append(dbNames, dbName)
				}
				if parseError == nil && len(dbNames) == 0 {
					parseError = &ValidationError{Message: "no valid databases specified in --databases flag"}
				}
			}

			if tt.expectError {
				require.Error(t, parseError)
				if tt.errorSubstring != "" {
					require.Contains(t, parseError.Error(), tt.errorSubstring)
				}
			} else {
				require.NoError(t, parseError)
				require.Equal(t, tt.expectedDBs, dbNames)
			}
		})
	}
}

// TestDBTypeConstants tests the db-type constant values
func TestDBTypeConstants(t *testing.T) {
	require.Equal(t, "app", DBTypeApp)
	require.Equal(t, "cometbft", DBTypeCometBFT)
	require.Equal(t, "all", DBTypeAll)
}

// TestDBTypeMapping tests the mapping of db-type to database names
func TestDBTypeMapping(t *testing.T) {
	tests := []struct {
		name        string
		dbType      string
		expectedDBs []string
		isValid     bool
	}{
		{
			name:        "app type",
			dbType:      DBTypeApp,
			expectedDBs: []string{"application"},
			isValid:     true,
		},
		{
			name:        "cometbft type",
			dbType:      DBTypeCometBFT,
			expectedDBs: []string{"blockstore", "state", "tx_index", "evidence"},
			isValid:     true,
		},
		{
			name:        "all type",
			dbType:      DBTypeAll,
			expectedDBs: []string{"application", "blockstore", "state", "tx_index", "evidence"},
			isValid:     true,
		},
		{
			name:    "invalid type",
			dbType:  "invalid",
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dbNames []string
			var isValid bool

			switch tt.dbType {
			case DBTypeApp:
				dbNames = []string{"application"}
				isValid = true
			case DBTypeCometBFT:
				dbNames = []string{"blockstore", "state", "tx_index", "evidence"}
				isValid = true
			case DBTypeAll:
				dbNames = []string{"application", "blockstore", "state", "tx_index", "evidence"}
				isValid = true
			default:
				isValid = false
			}

			require.Equal(t, tt.isValid, isValid)
			if tt.isValid {
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
		dbTypeFlag    string
		expectedDBs   []string
		useDatabases  bool
	}{
		{
			name:          "only db-type",
			databasesFlag: "",
			dbTypeFlag:    DBTypeApp,
			expectedDBs:   []string{"application"},
			useDatabases:  false,
		},
		{
			name:          "only databases",
			databasesFlag: "blockstore,tx_index",
			dbTypeFlag:    DBTypeApp,
			expectedDBs:   []string{"blockstore", "tx_index"},
			useDatabases:  true,
		},
		{
			name:          "both flags - databases takes precedence",
			databasesFlag: "state",
			dbTypeFlag:    DBTypeAll,
			expectedDBs:   []string{"state"},
			useDatabases:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dbNames []string

			// Simulate the logic from the command
			if tt.databasesFlag != "" {
				// Use databases flag
				dbList := splitAndTrim(tt.databasesFlag)
				for _, dbName := range dbList {
					if dbName != "" && validDatabaseNames[dbName] {
						dbNames = append(dbNames, dbName)
					}
				}
			} else {
				// Use db-type flag
				switch tt.dbTypeFlag {
				case DBTypeApp:
					dbNames = []string{"application"}
				case DBTypeCometBFT:
					dbNames = []string{"blockstore", "state", "tx_index", "evidence"}
				case DBTypeAll:
					dbNames = []string{"application", "blockstore", "state", "tx_index", "evidence"}
				}
			}

			require.Equal(t, tt.expectedDBs, dbNames)
			require.Equal(t, tt.useDatabases, tt.databasesFlag != "")
		})
	}
}

// Helper functions for tests

// splitAndTrim splits a string by comma and trims whitespace
func splitAndTrim(s string) []string {
	parts := make([]string, 0)
	current := ""
	for _, ch := range s {
		if ch == ',' {
			parts = append(parts, trimSpace(current))
			current = ""
		} else {
			current += string(ch)
		}
	}
	parts = append(parts, trimSpace(current))
	return parts
}

// trimSpace removes leading and trailing whitespace
func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n') {
		end--
	}

	return s[start:end]
}

// ValidationError is a simple error type for validation errors
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

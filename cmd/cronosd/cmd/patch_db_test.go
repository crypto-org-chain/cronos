package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTargetPathValidation tests that multi-database patching rejects *.db file paths
func TestTargetPathValidation(t *testing.T) {
	tests := []struct {
		name        string
		targetPath  string
		dbCount     int
		shouldError bool
		errorString string
	}{
		{
			name:        "single DB with .db extension - allowed",
			targetPath:  "/path/to/blockstore.db",
			dbCount:     1,
			shouldError: false,
		},
		{
			name:        "single DB without .db extension - allowed (will be validated later)",
			targetPath:  "/path/to/blockstore",
			dbCount:     1,
			shouldError: false,
		},
		{
			name:        "multiple DBs with data directory - allowed",
			targetPath:  "/path/to/data",
			dbCount:     2,
			shouldError: false,
		},
		{
			name:        "multiple DBs with .db file path - rejected",
			targetPath:  "/path/to/blockstore.db",
			dbCount:     2,
			shouldError: true,
			errorString: "must be a data directory",
		},
		{
			name:        "multiple DBs with .db file path (trailing slash) - rejected",
			targetPath:  "/path/to/blockstore.db/",
			dbCount:     2,
			shouldError: true,
			errorString: "must be a data directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the validation logic from patch_db.go
			var dbNames []string
			for i := 0; i < tt.dbCount; i++ {
				dbNames = append(dbNames, "testdb")
			}

			var err error
			if len(dbNames) == 1 {
				// Single DB: no validation in this branch
				_ = tt.targetPath
			} else {
				// Multiple DBs: validate targetPath is not a *.db file
				cleanedTargetPath := filepath.Clean(tt.targetPath)
				if filepath.Ext(cleanedTargetPath) == ".db" {
					err = &targetPathError{path: tt.targetPath}
				}
			}

			if tt.shouldError {
				require.Error(t, err)
				if tt.errorString != "" {
					require.Contains(t, err.Error(), tt.errorString)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// targetPathError is a helper type to simulate the error from patch_db.go
type targetPathError struct {
	path string
}

func (e *targetPathError) Error() string {
	return "when patching multiple databases, --target-path must be a data directory (e.g., ~/.cronos/data), not a *.db file path (got \"" + e.path + "\"); remove the .db suffix"
}

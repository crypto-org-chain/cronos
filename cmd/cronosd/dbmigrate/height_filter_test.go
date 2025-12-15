package dbmigrate

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHeightRange_IsWithinRange(t *testing.T) {
	tests := []struct {
		name   string
		hr     HeightRange
		height int64
		want   bool
	}{
		{
			name:   "empty range includes all",
			hr:     HeightRange{Start: 0, End: 0},
			height: 1000,
			want:   true,
		},
		{
			name:   "within range",
			hr:     HeightRange{Start: 100, End: 200},
			height: 150,
			want:   true,
		},
		{
			name:   "at start boundary",
			hr:     HeightRange{Start: 100, End: 200},
			height: 100,
			want:   true,
		},
		{
			name:   "at end boundary",
			hr:     HeightRange{Start: 100, End: 200},
			height: 200,
			want:   true,
		},
		{
			name:   "below start",
			hr:     HeightRange{Start: 100, End: 200},
			height: 99,
			want:   false,
		},
		{
			name:   "above end",
			hr:     HeightRange{Start: 100, End: 200},
			height: 201,
			want:   false,
		},
		{
			name:   "only start specified - within",
			hr:     HeightRange{Start: 1000, End: 0},
			height: 2000,
			want:   true,
		},
		{
			name:   "only start specified - below",
			hr:     HeightRange{Start: 1000, End: 0},
			height: 999,
			want:   false,
		},
		{
			name:   "only end specified - within",
			hr:     HeightRange{Start: 0, End: 1000},
			height: 500,
			want:   true,
		},
		{
			name:   "only end specified - above",
			hr:     HeightRange{Start: 0, End: 1000},
			height: 1001,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.hr.IsWithinRange(tt.height)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestHeightRange_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		hr   HeightRange
		want bool
	}{
		{
			name: "empty range",
			hr:   HeightRange{Start: 0, End: 0},
			want: true,
		},
		{
			name: "only start specified",
			hr:   HeightRange{Start: 100, End: 0},
			want: false,
		},
		{
			name: "only end specified",
			hr:   HeightRange{Start: 0, End: 200},
			want: false,
		},
		{
			name: "both specified",
			hr:   HeightRange{Start: 100, End: 200},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.hr.IsEmpty()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestHeightRange_String(t *testing.T) {
	tests := []struct {
		name string
		hr   HeightRange
		want string
	}{
		{
			name: "empty range",
			hr:   HeightRange{Start: 0, End: 0},
			want: "all heights",
		},
		{
			name: "both start and end",
			hr:   HeightRange{Start: 100, End: 200},
			want: "heights 100 to 200",
		},
		{
			name: "only start",
			hr:   HeightRange{Start: 1000, End: 0},
			want: "heights from 1000",
		},
		{
			name: "only end",
			hr:   HeightRange{Start: 0, End: 2000},
			want: "heights up to 2000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.hr.String()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestHeightRange_Validate(t *testing.T) {
	tests := []struct {
		name    string
		hr      HeightRange
		wantErr bool
	}{
		{
			name:    "valid range",
			hr:      HeightRange{Start: 100, End: 200},
			wantErr: false,
		},
		{
			name:    "valid empty range",
			hr:      HeightRange{Start: 0, End: 0},
			wantErr: false,
		},
		{
			name:    "valid only start",
			hr:      HeightRange{Start: 100, End: 0},
			wantErr: false,
		},
		{
			name:    "valid only end",
			hr:      HeightRange{Start: 0, End: 200},
			wantErr: false,
		},
		{
			name:    "negative start",
			hr:      HeightRange{Start: -1, End: 200},
			wantErr: true,
		},
		{
			name:    "negative end",
			hr:      HeightRange{Start: 100, End: -1},
			wantErr: true,
		},
		{
			name:    "start greater than end",
			hr:      HeightRange{Start: 200, End: 100},
			wantErr: true,
		},
		{
			name:    "start equals end",
			hr:      HeightRange{Start: 100, End: 100},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.hr.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestExtractHeightFromBlockstoreKey(t *testing.T) {
	tests := []struct {
		name       string
		key        []byte
		wantHeight int64
		wantOK     bool
	}{
		{
			name:       "block meta key H:",
			key:        makeBlockstoreKey("H:", 1000),
			wantHeight: 1000,
			wantOK:     true,
		},
		{
			name:       "block parts key P:",
			key:        makeBlockstoreKey("P:", 2000),
			wantHeight: 2000,
			wantOK:     true,
		},
		{
			name:       "block commit key C:",
			key:        makeBlockstoreKey("C:", 3000),
			wantHeight: 3000,
			wantOK:     true,
		},
		{
			name:       "seen commit key SC:",
			key:        makeSeenCommitKey(4000),
			wantHeight: 4000,
			wantOK:     true,
		},
		{
			name:       "extended commit key EC: (ABCI 2.0)",
			key:        []byte("EC:5000"),
			wantHeight: 5000,
			wantOK:     true,
		},
		{
			name:       "metadata key BS:H",
			key:        []byte("BS:H"),
			wantHeight: 0,
			wantOK:     false,
		},
		{
			name:       "too short key",
			key:        []byte("H:"),
			wantHeight: 0,
			wantOK:     false,
		},
		{
			name:       "unknown prefix",
			key:        []byte("XYZ:12345678"),
			wantHeight: 0,
			wantOK:     false,
		},
		{
			name:       "empty key",
			key:        []byte{},
			wantHeight: 0,
			wantOK:     false,
		},
		{
			name:       "height 0",
			key:        makeBlockstoreKey("H:", 0),
			wantHeight: 0,
			wantOK:     true,
		},
		{
			name:       "large height",
			key:        makeBlockstoreKey("H:", 10000000),
			wantHeight: 10000000,
			wantOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHeight, gotOK := extractHeightFromBlockstoreKey(tt.key)
			require.Equal(t, tt.wantOK, gotOK)
			if gotOK {
				require.Equal(t, tt.wantHeight, gotHeight)
			}
		})
	}
}

func TestExtractHeightFromTxIndexKey(t *testing.T) {
	tests := []struct {
		name       string
		key        []byte
		wantHeight int64
		wantOK     bool
	}{
		{
			name:       "tx.height key",
			key:        []byte("tx.height/1000/hash123"),
			wantHeight: 1000,
			wantOK:     true,
		},
		{
			name:       "tx.height key with long height",
			key:        []byte("tx.height/9999999/abcdef"),
			wantHeight: 9999999,
			wantOK:     true,
		},
		{
			name:       "tx.height key height 0",
			key:        []byte("tx.height/0/hash"),
			wantHeight: 0,
			wantOK:     true,
		},
		{
			name:       "tx.height prefix only",
			key:        []byte("tx.height/"),
			wantHeight: 0,
			wantOK:     false,
		},
		{
			name:       "non-height key",
			key:        []byte("tx.hash/abcdef"),
			wantHeight: 0,
			wantOK:     false,
		},
		{
			name:       "empty key",
			key:        []byte{},
			wantHeight: 0,
			wantOK:     false,
		},
		{
			name:       "malformed tx.height key",
			key:        []byte("tx.height/abc/hash"),
			wantHeight: 0,
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHeight, gotOK := extractHeightFromTxIndexKey(tt.key)
			require.Equal(t, tt.wantOK, gotOK)
			if gotOK {
				require.Equal(t, tt.wantHeight, gotHeight)
			}
		})
	}
}

func TestShouldIncludeKey(t *testing.T) {
	tests := []struct {
		name        string
		key         []byte
		dbName      string
		heightRange HeightRange
		want        bool
	}{
		{
			name:        "blockstore - within range",
			key:         makeBlockstoreKey("H:", 1500),
			dbName:      "blockstore",
			heightRange: HeightRange{Start: 1000, End: 2000},
			want:        true,
		},
		{
			name:        "blockstore - below range",
			key:         makeBlockstoreKey("H:", 500),
			dbName:      "blockstore",
			heightRange: HeightRange{Start: 1000, End: 2000},
			want:        false,
		},
		{
			name:        "blockstore - above range",
			key:         makeBlockstoreKey("H:", 2500),
			dbName:      "blockstore",
			heightRange: HeightRange{Start: 1000, End: 2000},
			want:        false,
		},
		{
			name:        "blockstore - metadata key always included",
			key:         []byte("BS:H"),
			dbName:      "blockstore",
			heightRange: HeightRange{Start: 1000, End: 2000},
			want:        true,
		},
		{
			name:        "blockstore - empty range includes all",
			key:         makeBlockstoreKey("H:", 500),
			dbName:      "blockstore",
			heightRange: HeightRange{Start: 0, End: 0},
			want:        true,
		},
		{
			name:        "tx_index - within range",
			key:         []byte("tx.height/1500/hash"),
			dbName:      "tx_index",
			heightRange: HeightRange{Start: 1000, End: 2000},
			want:        true,
		},
		{
			name:        "tx_index - below range",
			key:         []byte("tx.height/500/hash"),
			dbName:      "tx_index",
			heightRange: HeightRange{Start: 1000, End: 2000},
			want:        false,
		},
		{
			name:        "tx_index - non-height key always included",
			key:         []byte("tx.hash/abcdef"),
			dbName:      "tx_index",
			heightRange: HeightRange{Start: 1000, End: 2000},
			want:        true,
		},
		{
			name:        "application db - ignores height range",
			key:         []byte("some_app_key"),
			dbName:      "application",
			heightRange: HeightRange{Start: 1000, End: 2000},
			want:        true,
		},
		{
			name:        "state db - ignores height range",
			key:         []byte("some_state_key"),
			dbName:      "state",
			heightRange: HeightRange{Start: 1000, End: 2000},
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldIncludeKey(tt.key, tt.dbName, tt.heightRange)
			require.Equal(t, tt.want, got)
		})
	}
}

// Helper functions for tests

// makeBlockstoreKey creates a CometBFT blockstore key with the given prefix and height
func makeBlockstoreKey(prefix string, height int64) []byte {
	// String-encoded format
	if prefix == "P:" {
		// Block parts: "P:" + height + ":" + part
		return []byte(fmt.Sprintf("%s%d:0", prefix, height))
	}
	// For other prefixes: prefix + height
	return []byte(fmt.Sprintf("%s%d", prefix, height))
}

// makeSeenCommitKey creates a seen commit key with the given height
func makeSeenCommitKey(height int64) []byte {
	// String-encoded format: "SC:" + height
	return []byte(fmt.Sprintf("SC:%d", height))
}

func TestExtractBlockHashFromMetadata(t *testing.T) {
	tests := []struct {
		name    string
		value   []byte
		wantOK  bool
		wantLen int
	}{
		{
			name: "valid BlockMeta with hash",
			// Minimal protobuf-like structure: 0x0a (BlockID field) + len + 0x0a (Hash field) + hashlen + hash
			value: []byte{
				0x0a, 0x22, // Field 1 (BlockID), length 34
				0x0a, 0x20, // Field 1 (Hash), length 32
				// 32-byte hash
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
				0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
				0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
				0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
				// Additional fields (ignored)
				0x12, 0x00,
			},
			wantOK:  true,
			wantLen: 32,
		},
		{
			name:    "too short value",
			value:   []byte{0x0a, 0x22, 0x0a, 0x20},
			wantOK:  false,
			wantLen: 0,
		},
		{
			name:    "empty value",
			value:   []byte{},
			wantOK:  false,
			wantLen: 0,
		},
		{
			name: "value without BlockID field",
			value: []byte{
				0x12, 0x10, // Wrong field tag
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
				0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
			},
			wantOK:  false,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, ok := extractBlockHashFromMetadata(tt.value)
			require.Equal(t, tt.wantOK, ok)
			if ok {
				require.Equal(t, tt.wantLen, len(hash))
				require.NotNil(t, hash)
			} else {
				require.Nil(t, hash)
			}
		})
	}
}

package memiavl

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	DefaultChangesBz = []ChangeBz{
		[]byte("\x00\x05\x00\x00\x00hello\x05\x00\x00\x00world"), // PS: multiple <\x00> because we want 4 bytes as a key/value length field
		[]byte("\x00\x05\x00\x00\x00hello\x06\x00\x00\x00world1"),
		[]byte("\x00\x06\x00\x00\x00hello1\x06\x00\x00\x00world1"),
		[]byte("\x01\x06\x00\x00\x00hello2"),
	}

	DefaultChanges = []Change{
		{Key: []byte("hello"), Value: []byte("world")},
		{Key: []byte("hello"), Value: []byte("world1")},
		{Key: []byte("hello1"), Value: []byte("world1")},
		{Key: []byte("hello2"), Delete: true},
	}
)

func TestFlush(t *testing.T) {
	log, err := newBlockWAL("test", 0, nil)
	require.NoError(t, err)

	defer os.RemoveAll("test")
	defer log.Close()

	expectedData := []byte{}
	for _, i := range DefaultChangesBz {
		expectedData = append(expectedData, i...)
	}

	for _, change := range DefaultChanges {
		log.addChange(change)
	}

	err = log.Flush()
	require.NoError(t, err)

	data, err := log.Read(1)
	require.NoError(t, err)

	require.True(t, bytes.Equal(data, expectedData))
}

func TestIndexBlockChangesBytes(t *testing.T) {
	blockChanges := BlockChangesBz{}
	for _, i := range DefaultChangesBz {
		blockChanges = append(blockChanges, i...)
	}

	indexes, err := indexBlockChangesBytes(blockChanges)
	require.NoError(t, err)

	expectedIndexes := []uint64{0, 19, 39, 60}
	require.Equal(t, expectedIndexes, indexes)
}

func TestValid(t *testing.T) {
	tests := []struct {
		name  string
		bz    ChangeBz
		valid bool
	}{
		{
			name:  "valid",
			bz:    []byte("\x00\x05\x00\x00\x00hello\x05\x00\x00\x00world"),
			valid: true,
		},
		{
			name:  "invalid: value length",
			bz:    []byte("\x00\x05\x00\x00\x00hello\x05\x00\x00\x00world\x00"),
			valid: false,
		},
		{
			name:  "invalid: key length",
			bz:    []byte("\x00\x05\x00\x00\x00helloooooo\x05\x00\x00\x00world\x00\x00"),
			valid: false,
		},
		{
			name:  "invalid: first byte",
			bz:    []byte("\x05\x05\x00\x00\x00hello\x05\x00\x00\x00world\x01"),
			valid: false,
		},
		{
			name:  "invalid: empty bytes",
			bz:    []byte{},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.bz.Valid()
			if tt.valid {
				require.True(t, valid)
			} else {
				require.False(t, valid)
			}
		})
	}
}

func TestChangesetFromBz(t *testing.T) {
	blockChanges := BlockChangesBz{}
	for _, i := range DefaultChangesBz {
		blockChanges = append(blockChanges, i...)
	}

	changes, err := blockChanges.changesetFromBz()
	require.NoError(t, err)

	for i, change := range changes {
		require.Equal(t, DefaultChanges[i], change)
	}
}

func TestChangeFromBz(t *testing.T) {
	var changeBz ChangeBz
	for i, changeBz := range DefaultChangesBz {
		change, err := changeBz.changeFromBz()
		require.NoError(t, err)

		require.Equal(t, DefaultChanges[i], change)
	}

	// add invalid change case
	changeBz = []byte("\x00\x05\x00\x00\x00hello\x06\x00\x00\x00world")
	_, err := changeBz.changeFromBz()
	require.Error(t, err)
}

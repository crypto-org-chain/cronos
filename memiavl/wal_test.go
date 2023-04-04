package memiavl

import (
	"bytes"
	"os"
	"testing"

	"github.com/cosmos/iavl"
	"github.com/stretchr/testify/require"
)

var (
	DefaultChangesBz = [][]byte{
		[]byte("\x00\x05hello\x05world"), // PS: multiple <\x00> because we want 4 bytes as a key/value length field
		[]byte("\x00\x05hello\x06world1"),
		[]byte("\x00\x06hello1\x06world1"),
		[]byte("\x01\x06hello2"),
	}

	DefaultChanges = iavl.ChangeSet{
		Pairs: []iavl.KVPair{
			{Key: []byte("hello"), Value: []byte("world")},
			{Key: []byte("hello"), Value: []byte("world1")},
			{Key: []byte("hello1"), Value: []byte("world1")},
			{Key: []byte("hello2"), Delete: true},
		},
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

	for _, change := range DefaultChanges.Pairs {
		log.addChange(change)
	}

	err = log.Flush()
	require.NoError(t, err)

	data, err := log.Read(1)
	require.NoError(t, err)

	require.True(t, bytes.Equal(data, expectedData))
}

func TestAreValidChangeBytes(t *testing.T) {
	tests := []struct {
		name  string
		bz    []byte
		valid bool
	}{
		{
			name:  "valid",
			bz:    []byte("\x00\x05hello\x05world"),
			valid: true,
		},
		{
			name:  "invalid: value length",
			bz:    []byte("\x00\x05hello\x05world\x00"),
			valid: false,
		},
		{
			name:  "invalid: key length",
			bz:    []byte("\x00\x05helloooooo\x05world\x00\x00"),
			valid: false,
		},
		{
			name:  "invalid: first byte",
			bz:    []byte("\x05\x05hello\x05world\x01"),
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
			valid := areValidChangeBytes(tt.bz)
			if tt.valid {
				require.True(t, valid)
			} else {
				require.False(t, valid)
			}
		})
	}
}

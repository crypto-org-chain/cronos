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
		[]byte("\x00\x05hello\x05world"),
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

	err = log.Flush(DefaultChanges)
	require.NoError(t, err)

	data, err := log.Read(1)
	require.NoError(t, err)

	require.True(t, bytes.Equal(data, expectedData))
}

func removeDefaultWal() {
	os.RemoveAll(DefaultPathToWAL)
}

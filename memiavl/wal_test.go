package memiavl

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlush(t *testing.T) {
	log, err := newBlockWAL("test", 1, nil)
	require.NoError(t, err)

	defer os.RemoveAll("test")
	defer log.Close()

	changes := []Change{
		{Key: []byte("hello"), Value: []byte("world")},
		{Key: []byte("hello"), Value: []byte("world1")},
		{Key: []byte("hello1"), Value: []byte("world1")},
		{Key: []byte("hello2"), Delete: true},
	}

	changesBz := [][]byte{
		[]byte("\x00\x05\x00\x00\x00hello\x05\x00\x00\x00world"), // PS: multiple <\x00> because we want 4 bytes as a key/value length field
		[]byte("\x00\x05\x00\x00\x00hello\x06\x00\x00\x00world1"),
		[]byte("\x00\x06\x00\x00\x00hello1\x06\x00\x00\x00world1"),
		[]byte("\x01\x06\x00\x00\x00hello2"),
	}

	expectedData := []byte{}
	for _, i := range changesBz {
		expectedData = append(expectedData, i...)
	}

	for i := range changes {
		log.addChange(changes[i])
	}

	err = log.Flush()
	require.NoError(t, err)

	data, err := log.Read(1)
	require.NoError(t, err)

	require.True(t, bytes.Equal(data, expectedData))
}

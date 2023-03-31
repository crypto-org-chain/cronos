package memiavl

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/wal"
)

func TestWriteChange(t *testing.T) {
	log, err := wal.Open("test", nil)
	require.NoError(t, err)

	defer os.RemoveAll("test")
	defer log.Close()

	changes := []Change{
		{Key: []byte("hello"), Value: []byte("world")},
		{Key: []byte("hello"), Value: []byte("world1")},
		{Key: []byte("hello1"), Value: []byte("world1")},
		{Key: []byte("hello2"), Value: []byte("world1"), Delete: true},
	}

	expectedBz := [][]byte{
		[]byte("\x00\x05\x00\x00\x00hello\x05\x00\x00\x00world"), // PS: multiple <\x00> because we want 4 bytes as a key/value length
		[]byte("\x00\x05\x00\x00\x00hello\x06\x00\x00\x00world1"),
		[]byte("\x00\x06\x00\x00\x00hello1\x06\x00\x00\x00world1"),
		[]byte("\x01\x06\x00\x00\x00hello2"),
	}

	for i, change := range changes {
		_, err := writeChange(log, change, uint64(i+1))
		require.NoError(t, err)
	}

	for i := 0; i < len(changes); i++ {
		data, err := log.Read(uint64(i + 1))
		require.NoError(t, err)

		require.True(t, bytes.Equal(data, expectedBz[i]))
	}
}

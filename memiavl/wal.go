package memiavl

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tidwall/wal"
)

type Change struct {
	Delete     bool
	Key, Value []byte
}

var (
	DefaultPathToWAL string
)

const (
	// amount of bytes it takes to store key's and value's lengths
	KeyValueLen = 4
)

// ChangeBz is a byte slice containing information about one change to memiavl.
// Structure is as follows:
// deleteByte: 1 byte (0 or 1)
// keyLen: 4 bytes (length of key)
// key: key bytes

// if delete is 0, also contains:

// valueLen: 4 bytes (length of value)
// value: value bytes
type ChangeBz = []byte

func init() {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	DefaultPathToWAL = filepath.Join(userHomeDir, ".cronosd/data/application.wal")
}

func NewWAL(pathToWAL string, opts *wal.Options) (*wal.Log, error) {
	if pathToWAL == "" {
		return nil, fmt.Errorf("failed trying to create a new WAL: path to WAL is empty")
	}

	return wal.Open(pathToWAL, opts)
}

// prepareChange prepares a change for write-ahead log.
// If value provided is nil, it implies the Change should indicate deletion.
func prepareChange(key []byte, value []byte) Change {
	return Change{
		Delete: value == nil,
		Key:    key,
		Value:  value,
	}
}

// writeChange writes a change to the write-ahead log.
// Returns bytes written
func writeChange(wal *wal.Log, change Change, index uint64) (uint64, error) {
	bzWant := calculateNeededBytes(change)
	bz := make([]byte, bzWant, bzWant)

	var offset uint64 // offset is used to keep track of the current position in the byte slice
	if change.Delete {
		bz[offset] = uint8(1)
		offset++

		binary.LittleEndian.PutUint32(bz[1:KeyValueLen+1], uint32(len(change.Key)))
		offset += KeyValueLen

		bz = writeBytes(bz, change.Key, offset, uint64(len(change.Key)))
		offset += uint64(len(change.Key))
	} else {
		bz[offset] = uint8(0)
		offset++

		binary.LittleEndian.PutUint32(bz[offset:offset+KeyValueLen], uint32(len(change.Key)))
		offset += KeyValueLen

		bz = writeBytes(bz, change.Key, offset, uint64(len(change.Key)))
		offset += uint64(len(change.Key))

		binary.LittleEndian.PutUint32(bz[offset:offset+KeyValueLen], uint32(len(change.Value)))
		offset += KeyValueLen

		bz = writeBytes(bz, change.Value, offset, uint64(len(change.Value)))
		offset += uint64(len(change.Value))
	}

	err := wal.Write(index, bz)
	if err != nil {
		return 0, fmt.Errorf("failed to write change to WAL: %w", err)
	}

	return offset, nil
}

// calculateNeededBytes calculates the amount of bytes needed to store a change.
func calculateNeededBytes(change Change) uint64 {
	var neededBytes uint64
	if change.Delete {
		neededBytes = 1 + KeyValueLen + uint64(len(change.Key)) // delete/set byte + key length + key
	} else {
		neededBytes = 1 + KeyValueLen*2 + uint64(len(change.Key)) + uint64(len(change.Value)) // delete/set byte + key length + key + value length + value
	}

	return neededBytes
}

// writeBytes writes a byte slice to another byte slice at a given offset.
func writeBytes(bz, data []byte, offset, length uint64) []byte {
	for i := offset; i < offset+length; i++ {
		bz[i] = data[i-offset]
	}
	return bz
}

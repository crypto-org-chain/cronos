package memiavl

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tidwall/wal"
)

// blockWAL is a wrapper around write-ahead log that is used to store changes made to memiavl at every block.
// Version is an index at which changes are stored in WAL.
// At version X, changes stored correspond to changesets that have been made from block X-1 to block X.
type blockWAL struct {
	wal            *wal.Log
	version        uint64
	BlockChangeset []Change
}

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

type (
	// ChangeBz is a byte slice containing information about one change to memiavl.
	// Structure is as follows:
	// deleteByte: 1 byte (0 or 1)
	// keyLen: 4 bytes (length of key)
	// key: key bytes

	// if delete is 0, also contains:

	// valueLen: 4 bytes (length of value)
	// value: value bytes
	ChangeBz []byte

	// BlockChangesBz is a byte slice containing information about changes to memiavl in specific block.
	BlockChangesBz []byte
)

func init() {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	DefaultPathToWAL = filepath.Join(userHomeDir, ".cronosd/data/application.wal")
}

// newBlockWAL creates a new blockWAL.
// TODO: creating a WAL at non 0 version will fail, because the log will try to store changes from that version right away.
// It is not supported by the library, write-ahead log supposed to start with index 1 and increase monotonically.
func newBlockWAL(pathToWAL string, version uint64, opts *wal.Options) (blockWAL, error) {
	if pathToWAL == "" {
		return blockWAL{}, fmt.Errorf("failed trying to create a new WAL: path to WAL is empty")
	}
	log, err := wal.Open(pathToWAL, opts)
	if err != nil {
		return blockWAL{}, err
	}
	return blockWAL{
		wal:            log,
		version:        version,
		BlockChangeset: nil,
	}, nil
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
func (bwal blockWAL) writeBlockChanges(changes []Change, index uint64) (uint64, error) {
	var bytesWritten uint64
	var bz ChangeBz
	var block BlockChangesBz
	for _, change := range changes {
		offset := uint64(0)

		bz, offset = prepareChangeBz(change)
		block = append(block, bz...)

		bytesWritten += offset
	}

	err := bwal.wal.Write(index+1, block)
	if err != nil {
		return 0, fmt.Errorf("failed to write change to WAL: %w", err)
	}

	return bytesWritten, nil
}

// Close closes the underlying write-ahead log.
func (bwal blockWAL) Close() error {
	return bwal.wal.Close()
}

// Read reads the write-ahead log from the given index.
func (bwal blockWAL) Read(index uint64) (BlockChangesBz, error) {
	return bwal.wal.Read(index)
}

// calculateNeededBytes calculates the amount of bytes needed to store a Change.
func calculateNeededBytes(change Change) uint64 {
	var neededBytes uint64
	if change.Delete {
		neededBytes = 1 + KeyValueLen + uint64(len(change.Key)) // delete/set byte + key length + key
	} else {
		neededBytes = 1 + KeyValueLen*2 + uint64(len(change.Key)) + uint64(len(change.Value)) // delete/set byte + key length + key + value length + value
	}

	return neededBytes
}

// prepareChangeBz transforms a Change into a byte format.
func prepareChangeBz(change Change) (ChangeBz, uint64) {
	var offset uint64
	bzWant := calculateNeededBytes(change)
	bz := make([]byte, bzWant, bzWant)

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

	return bz, offset
}

// writeBytes writes a byte slice to another byte slice at a given offset.
// func writeBytes(bz, data []byte, offset, length uint64) []byte {
// 	for i := offset; i < offset+length; i++ {
// 		bz[i] = data[i-offset]
// 	}
// 	return bz
// }

// addChange adds a change to the block changeset.
func (bwal *blockWAL) addChange(change Change) {
	bwal.BlockChangeset = append(bwal.BlockChangeset, change)
}

// Flush flushes the block changeset to the write-ahead log.
func (bwal *blockWAL) Flush() error {
	_, err := bwal.writeBlockChanges(bwal.BlockChangeset, bwal.version)
	if err != nil {
		return err
	}

	bwal.BlockChangeset = nil
	bwal.version++
	return nil
}

// OpenWAL returns an instance of the write-ahead log.
func OpenWAL(pathToWAL string, opts *wal.Options) (*wal.Log, error) {
	return wal.Open(pathToWAL, opts)
}

// changesetFromBz generates a changeset from a byte slice.
func (bz BlockChangesBz) changesetFromBz() ([]Change, error) {
	var changes []Change

	indexes, err := indexBlockChangesBytes(bz)
	if err != nil {
		return nil, fmt.Errorf("failed to index block changes bytes: %w", err)
	}

	for i := 0; i < len(indexes); i++ {
		var changeBz ChangeBz
		var change Change
		if i == len(indexes)-1 { // if we are iterating through the last change bytes, we collect all remaining bytes
			changeBz = append(changeBz, bz[indexes[i]:]...)
		} else {
			changeBz = append(changeBz, bz[indexes[i]:indexes[i+1]]...)
		}
		change, err = changeBz.changeFromBz()
		if err != nil {
			return nil, fmt.Errorf("failed to generate change from bytes: %w", err)
		}

		changes = append(changes, change)
	}
	return changes, nil
}

func (bz ChangeBz) changeFromBz() (Change, error) {
	var change Change
	var offset uint64
	var keyLen, valueLen uint32

	if !bz.Valid() {
		return Change{}, fmt.Errorf("invalid change bytes")
	}

	if bz[offset] == uint8(1) {
		change.Delete = true
		offset++
	} else {
		change.Delete = false
		offset++
	}

	keyLen = binary.LittleEndian.Uint32(bz[offset : offset+KeyValueLen])
	offset += KeyValueLen

	change.Key = bz[offset : offset+uint64(keyLen)]
	offset += uint64(keyLen)

	if !change.Delete {
		valueLen = binary.LittleEndian.Uint32(bz[offset : offset+KeyValueLen])
		offset += KeyValueLen

		change.Value = bz[offset : offset+uint64(valueLen)]
		offset += uint64(valueLen)
	}

	return change, nil
}

// indexBlockChangesBytes returns an array with indexes of the first element of each ChangeBz in BlockChangesBz.
func indexBlockChangesBytes(bz BlockChangesBz) ([]uint64, error) {
	var hoppingIndex uint64
	var indexes []uint64

	indexes = append(indexes, hoppingIndex) // add 0 to indicate the first Change

	for hoppingIndex < uint64(len(bz))-1 {
		var length uint64
		length++ // account for delete/set byte either ways

		switch bz[hoppingIndex] {
		case uint8(0): // indicates set
			length += KeyValueLen * 2
			// add key and value length
			keyLen := binary.LittleEndian.Uint32(bz[1+hoppingIndex : KeyValueLen+1+hoppingIndex])                                               // key's length comes after the first bit up to the key length (4 bytes) + the current hopping index
			valueLen := binary.LittleEndian.Uint32(bz[1+KeyValueLen+keyLen+uint32(hoppingIndex) : 1+KeyValueLen*2+keyLen+uint32(hoppingIndex)]) // value's length comes after the key length (4 bytes) + the key + the current hopping index

			length += uint64(keyLen + valueLen)
		case uint8(1): // indicates delete
			length += KeyValueLen // add key length

			keyLen := binary.LittleEndian.Uint32(bz[1 : KeyValueLen+1])

			length += uint64(keyLen)
		default:
			return []uint64{}, fmt.Errorf("invalid first delete/set byte , expected: 1 or 0, got: %d", bz[hoppingIndex])
		}
		hoppingIndex += length

		if hoppingIndex < uint64(len(bz)-1) { // this is to ensure we don't add the last index (which is the length of the byte slice)
			indexes = append(indexes, hoppingIndex)
		}
	}
	return indexes, nil
}

// Valid asserts ChangeBz length is valid.
func (cbz ChangeBz) Valid() bool {
	if len(cbz) == 0 {
		return false
	}

	if cbz[0] != 0 && cbz[0] != 1 {
		return false
	}

	switch cbz[0] {
	case 0:
		keyLen := binary.LittleEndian.Uint32(cbz[1 : KeyValueLen+1])
		valueLen := binary.LittleEndian.Uint32(cbz[1+KeyValueLen+keyLen : KeyValueLen*2+keyLen+1])

		if 1+KeyValueLen*2+keyLen+valueLen != uint32(len(cbz)) { // set/del byte + key length + value length + key + value
			return false
		}
	case 1:
		keyLen := binary.LittleEndian.Uint32(cbz[1 : KeyValueLen+1])
		if 1+KeyValueLen+keyLen != uint32(len(cbz)) { // set/del byte + key length + key
			return false
		}
	}

	return true
}

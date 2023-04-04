package memiavl

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosmos/iavl"
	"github.com/tidwall/wal"
)

// blockWAL is a wrapper around write-ahead log that is used to store changes made to memiavl at every block.
// Version is an index at which changes are stored in WAL.
// At version X, changes stored correspond to changesets that have been made from block X-1 to block X.
type blockWAL struct {
	wal            *wal.Log
	version        uint64
	BlockChangeset iavl.ChangeSet
}

var (
	DefaultPathToWAL string
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
		BlockChangeset: iavl.ChangeSet{},
	}, nil
}

// prepareChange prepares a change for write-ahead log.
// If value provided is nil, it implies the Change should indicate deletion.
func preparePair(key []byte, value []byte) iavl.KVPair {
	return iavl.KVPair{
		Delete: value == nil,
		Key:    key,
		Value:  value,
	}
}

// writeChange writes a change to the write-ahead log.
// Returns bytes written
func (bwal blockWAL) writeBlockChanges(changeset iavl.ChangeSet, index uint64) error {
	bz, err := MarshalChangeSet(&changeset)
	if err != nil {
		return fmt.Errorf("failed to marshal changeset: %w", err)
	}

	err = bwal.wal.Write(index+1, bz)
	if err != nil {
		return fmt.Errorf("failed to write change to WAL: %w", err)
	}

	return nil
}

// Close closes the underlying write-ahead log.
func (bwal blockWAL) Close() error {
	return bwal.wal.Close()
}

// Read reads the write-ahead log from the given index.
func (bwal blockWAL) Read(index uint64) ([]byte, error) {
	return bwal.wal.Read(index)
}

// Flush flushes the block changeset to the write-ahead log.
func (bwal *blockWAL) Flush() error {
	err := bwal.writeBlockChanges(bwal.BlockChangeset, bwal.version)
	if err != nil {
		return err
	}

	bwal.BlockChangeset = iavl.ChangeSet{}
	bwal.version++
	return nil
}

// OpenWAL returns an instance of the write-ahead log.
func OpenWAL(pathToWAL string, opts *wal.Options) (*wal.Log, error) {
	return wal.Open(pathToWAL, opts)
}

// Valid asserts KVPair bytes' length is valid.
func areValidChangeBytes(changeBz []byte) bool {
	if len(changeBz) == 0 {
		return false
	}

	if changeBz[0] != 0 && changeBz[0] != 1 {
		return false
	}

	switch changeBz[0] {
	case 0:
		keyLen, nk := binary.Uvarint(changeBz[1:])
		if nk <= 0 {
			panic(fmt.Sprintf("failed to read key length, with n = %d", nk))
		}

		valueLen, nv := binary.Uvarint(changeBz[1+uint64(nk)+keyLen:])
		if nv <= 0 {
			panic(fmt.Sprintf("failed to read value length, with n = %d", nv))
		}

		if 1+uint64(nk)+uint64(nv)+keyLen+valueLen != uint64(len(changeBz)) { // set/del byte + key length + value length + key + value
			return false
		}
	case 1:
		keyLen, nk := binary.Uvarint(changeBz[1:])
		if nk <= 0 {
			panic(fmt.Sprintf("failed to read key length, with n = %d", nk))
		}

		if 1+uint64(nk)+keyLen != uint64(len(changeBz)) { // set/del byte + key length + key
			return false
		}
	}

	return true
}

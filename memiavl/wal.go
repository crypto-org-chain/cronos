package memiavl

import (
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
	wal     *wal.Log
	version uint64
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
		wal:     log,
		version: version,
	}, nil
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
	bz, err := bwal.wal.Read(index)
	if err != nil {
		return nil, fmt.Errorf("failed to read index %d from WAL: %w", index, err)
	}

	return bz, nil
}

// Flush flushes the block changeset to the write-ahead log.
func (bwal *blockWAL) Flush(blockChangeset iavl.ChangeSet) error {
	err := bwal.writeBlockChanges(blockChangeset, bwal.version)
	if err != nil {
		return err
	}

	bwal.version++
	return nil
}

// OpenWAL returns an instance of the write-ahead log.
func OpenWAL(pathToWAL string, opts *wal.Options) (*wal.Log, error) {
	return wal.Open(pathToWAL, opts)
}

package memiavl

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math"

	"github.com/cosmos/iavl"
	dbm "github.com/tendermint/tm-db"
	"github.com/tidwall/wal"
)

var emptyHash = sha256.New().Sum(nil)

// verify change sets by replay them to rebuild iavl tree and verify the root hashes
type Tree struct {
	version uint32
	// root node of empty tree is represented as `nil`
	root Node
	// write ahead log to store changesets for each block
	bwal           *blockWAL
	initialVersion uint32
}

// NewEmptyTree creates an empty tree at an arbitrary version.
func NewEmptyTree(version uint64, pathToWAL string) (*Tree, error) {
	if version >= math.MaxUint32 {
		panic("version overflows uint32")
	}

	var (
		wal *blockWAL
		err error
	)
	if pathToWAL != "" {
		wal, err = newBlockWAL(pathToWAL, version, nil)
		if err != nil {
			return nil, err
		}
	}
	return &Tree{version: uint32(version), bwal: wal}, nil
}

// New creates an empty tree at genesis version
func New(pathToWAL string) (*Tree, error) {
	return NewEmptyTree(0, pathToWAL)
}

// New creates a empty tree with initial-version,
// it happens when a new store created at the middle of the chain.
func NewWithInitialVersion(initialVersion int64, pathToWAL string) (*Tree, error) {
	if initialVersion >= math.MaxUint32 {
		return nil, errors.New("version overflows uint32")
	}
	tree, err := New(pathToWAL)
	if err != nil {
		return nil, err
	}
	tree.initialVersion = uint32(initialVersion)
	return tree, nil
}

// NewFromSnapshot mmap the blob files and create the root node.
func NewFromSnapshot(snapshot *Snapshot, pathToWAL string) (*Tree, error) {
	if snapshot.IsEmpty() {
		return NewEmptyTree(uint64(snapshot.Version()), pathToWAL)
	}

	var (
		wal *blockWAL
		err error
	)
	if pathToWAL != "" {
		wal, err = newBlockWAL(pathToWAL, uint64(snapshot.Version()), nil)
		if err != nil {
			return nil, err
		}
	}

	return &Tree{
		version: snapshot.Version(),
		root:    snapshot.RootNode(),
		bwal:    wal,
	}, nil
}

// ApplyChangeSet apply the change set of a whole version, and update hashes.
// Returns hash, new version, and potential error.
func (t *Tree) ApplyChangeSet(changeSet iavl.ChangeSet, updateHash bool) ([]byte, int64, error) {
	if t.bwal != nil {
		if err := t.bwal.Flush(changeSet); err != nil {
			return nil, 0, err
		}
	}

	for _, pair := range changeSet.Pairs {
		if pair.Delete {
			t.remove(pair.Key)
		} else {
			t.set(pair.Key, pair.Value)
		}
	}

	return t.saveVersion(updateHash)
}

func (t *Tree) set(key, value []byte) {
	t.root, _ = setRecursive(t.root, key, value, t.version+1)
}

func (t *Tree) remove(key []byte) {
	_, t.root, _ = removeRecursive(t.root, key, t.version+1)
}

// saveVersion increases the version number and optionally updates the hashes
func (t *Tree) saveVersion(updateHash bool) ([]byte, int64, error) {
	var hash []byte
	if updateHash {
		hash = t.root.Hash()
	}

	if t.version >= uint32(math.MaxUint32) {
		return nil, 0, errors.New("version overflows uint32")
	}
	t.version++

	// to be compatible with existing golang iavl implementation.
	// see: https://github.com/cosmos/iavl/pull/660
	if t.version == 1 && t.initialVersion > 0 {
		t.version = t.initialVersion
	}

	return hash, int64(t.version), nil
}

// Version returns the current tree version
func (t *Tree) Version() int64 {
	return int64(t.version)
}

// RootHash updates the hashes and return the current root hash
func (t *Tree) RootHash() []byte {
	if t.root == nil {
		return emptyHash
	}
	return t.root.Hash()
}

func (t *Tree) Get(key []byte) []byte {
	if t.root == nil {
		return nil
	}

	return t.root.Get(key)
}

func (t *Tree) Iterator(start, end []byte, ascending bool) dbm.Iterator {
	return NewIterator(start, end, ascending, t.root)
}

// ReplayWAL replays all the changesets on the tree sequentially until version "untilVersion" from the WAL with "walPath".
// If untilVersion is 0, it replays all the changesets from the WAL.
func (t *Tree) ReplayWAL(untilVersion uint64, walPath string) error {
	// if true, means the tree is up to date with the WAL
	if untilVersion <= uint64(t.version) && untilVersion != 0 {
		return fmt.Errorf("tree already up to date with untilVersion: %d with current version %d", untilVersion, t.version)
	}

	wal, err := wal.Open(walPath, nil)
	if err != nil {
		return fmt.Errorf("failed to open wal: %w", err)
	}

	// check if the untilVersion exists in wal
	latestVersion, err := wal.LastIndex()
	if err != nil {
		return fmt.Errorf("failed to get last index of wal: %w", err)
	}

	// error if untilVersion is greater than max version in wal
	if untilVersion > latestVersion || untilVersion == 0 && uint64(t.version) >= latestVersion {
		return fmt.Errorf("untilVersion %d is greater than latest version in wal %d", untilVersion, latestVersion)
	}

	// if untilVersion is 0, replay all changesets from the WAL
	if untilVersion == 0 {
		untilVersion, err = wal.LastIndex()
		if err != nil {
			return fmt.Errorf("failed to get last index of wal: %w", err)
		}
	}

	// collect all changesets from WAL
	for i := uint64(t.version + 1); i <= untilVersion; i++ {
		bz, err := wal.Read(i)
		if err != nil {
			return fmt.Errorf("failed to read changeset with index %d from wal: %w", i, err)
		}

		blockChanges, err := UnmarshalChangeSet(bz)
		if err != nil {
			return err
		}

		// apply changes right away
		if _, _, err := t.ApplyChangeSet(blockChanges, false); err != nil {
			return err
		}
	}

	t.RootHash()

	return nil
}

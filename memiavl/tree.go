package memiavl

import (
	"crypto/sha256"
	"errors"
	"math"

	"github.com/cosmos/iavl"
	dbm "github.com/tendermint/tm-db"
)

var emptyHash = sha256.New().Sum(nil)

// verify change sets by replay them to rebuild iavl tree and verify the root hashes
type Tree struct {
	version uint32
	// root node of empty tree is represented as `nil`
	root     Node
	snapshot *Snapshot

	initialVersion, cowVersion uint32
}

// NewEmptyTree creates an empty tree at an arbitrary version.
func NewEmptyTree(version int64) *Tree {
	if version >= math.MaxUint32 {
		panic("version overflows uint32")
	}
	return &Tree{version: uint32(version)}
}

// New creates an empty tree at genesis version
func New() *Tree {
	return NewEmptyTree(0)
}

// New creates a empty tree with initial-version,
// it happens when a new store created at the middle of the chain.
func NewWithInitialVersion(initialVersion uint32) *Tree {
	tree := New()
	tree.initialVersion = initialVersion
	return tree
}

// NewFromSnapshot mmap the blob files and create the root node.
func NewFromSnapshot(snapshot *Snapshot) *Tree {
	tree := &Tree{
		version:  snapshot.Version(),
		snapshot: snapshot,
	}

	if !snapshot.IsEmpty() {
		tree.root = snapshot.RootNode()
	}

	return tree
}

// Copy returns a snapshot of the tree which won't be corrupted by further modifications on the main tree.
func (t *Tree) Copy() *Tree {
	if _, ok := t.root.(*MemNode); ok {
		// protect the existing `MemNode`s from get modified in-place
		t.cowVersion = t.version
	}
	newTree := *t
	return &newTree
}

// ApplyChangeSet apply the change set of a whole version, and update hashes.
func (t *Tree) ApplyChangeSet(changeSet iavl.ChangeSet, updateHash bool) ([]byte, int64, error) {
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
	t.root, _ = setRecursive(t.root, key, value, t.version+1, t.cowVersion)
}

func (t *Tree) remove(key []byte) {
	_, t.root, _ = removeRecursive(t.root, key, t.version+1, t.cowVersion)
}

// saveVersion increases the version number and optionally updates the hashes
func (t *Tree) saveVersion(updateHash bool) ([]byte, int64, error) {
	var hash []byte
	if updateHash {
		hash = t.RootHash()
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

func (t *Tree) Close() error {
	var err error
	if t.snapshot != nil {
		err = t.snapshot.Close()
		t.snapshot = nil
	}
	t.root = nil
	return err
}

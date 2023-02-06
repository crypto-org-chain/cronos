package memiavl

import "crypto/sha256"

var emptyHash = sha256.New().Sum(nil)

// verify change sets by replay them to rebuild iavl tree and verify the root hashes
type Tree struct {
	version int64
	// root node of empty tree is represented as `nil`
	root Node

	initialVersion int64
}

// NewEmptyTree creates an empty tree at an arbitrary version.
func NewEmptyTree(version int64) *Tree {
	return &Tree{version: version}
}

// New creates an empty tree at genesis version
func New() *Tree {
	return NewEmptyTree(0)
}

// New creates a empty tree with initial-version,
// it happens when a new store created at the middle of the chain.
func NewWithInitialVersion(initialVersion int64) *Tree {
	tree := New()
	tree.initialVersion = initialVersion
	return tree
}

// LoadTreeFromSnapshot mmap the blob files and create the root node.
func LoadTreeFromSnapshot(snapshotDir string) (*Tree, *Snapshot, error) {
	snapshot, err := OpenSnapshot(snapshotDir)
	if err != nil {
		return nil, nil, err
	}

	return &Tree{
		version: int64(snapshot.Version()),
		root:    snapshot.RootNode(),
	}, snapshot, nil
}

func (t *Tree) Set(key, value []byte) {
	t.root, _ = setRecursive(t.root, key, value, t.version+1)
}

func (t *Tree) Remove(key []byte) {
	_, t.root, _ = removeRecursive(t.root, key, t.version+1)
}

// SaveVersion increases the version number and optionally updates the hashes
func (t *Tree) SaveVersion(updateHash bool) ([]byte, int64, error) {
	var hash []byte
	if updateHash {
		hash = t.root.Hash()
	}
	t.version++

	// to be compatible with existing golang iavl implementation.
	// see: https://github.com/cosmos/iavl/pull/660
	if t.version == 1 && t.initialVersion > 0 {
		t.version = t.initialVersion
	}

	return hash, t.version, nil
}

// Version returns the current tree version
func (t *Tree) Version() int64 {
	return t.version
}

// RootHash updates the hashes and return the current root hash
func (t *Tree) RootHash() []byte {
	if t.root == nil {
		return emptyHash
	}
	return t.root.Hash()
}

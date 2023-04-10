package memiavl

import (
	"fmt"
	"os"
	"testing"

	"github.com/cosmos/iavl"
	"github.com/stretchr/testify/require"
	db "github.com/tendermint/tm-db"
)

var (
	ChangeSets []iavl.ChangeSet
	RefHashes  [][]byte
)

func init() {
	ChangeSets = append(ChangeSets,
		iavl.ChangeSet{Pairs: []iavl.KVPair{{Key: []byte("hello"), Value: []byte("world")}}},
		iavl.ChangeSet{Pairs: []iavl.KVPair{{Key: []byte("hello"), Value: []byte("world1")}, {Key: []byte("hello1"), Value: []byte("world1")}}},
		iavl.ChangeSet{Pairs: []iavl.KVPair{{Key: []byte("hello2"), Value: []byte("world1")}, {Key: []byte("hello3"), Value: []byte("world1")}}},
	)

	changes := iavl.ChangeSet{}
	for i := 0; i < 1; i++ {
		changes.Pairs = append(changes.Pairs, iavl.KVPair{Key: []byte(fmt.Sprintf("hello%02d", i)), Value: []byte("world1")})
	}

	ChangeSets = append(ChangeSets, changes)
	ChangeSets = append(ChangeSets, iavl.ChangeSet{Pairs: []iavl.KVPair{{Key: []byte("hello"), Delete: true}, {Key: []byte("hello19"), Delete: true}}})

	changes = iavl.ChangeSet{}
	for i := 0; i < 21; i++ {
		changes.Pairs = append(changes.Pairs, iavl.KVPair{Key: []byte(fmt.Sprintf("aello%02d", i)), Value: []byte("world1")})
	}
	ChangeSets = append(ChangeSets, changes)

	changes = iavl.ChangeSet{}
	for i := 0; i < 21; i++ {
		changes.Pairs = append(changes.Pairs, iavl.KVPair{Key: []byte(fmt.Sprintf("aello%02d", i)), Delete: true})
	}
	for i := 0; i < 19; i++ {
		changes.Pairs = append(changes.Pairs, iavl.KVPair{Key: []byte(fmt.Sprintf("hello%02d", i)), Delete: true})
	}
	ChangeSets = append(ChangeSets, changes)

	// generate ref hashes with ref impl
	d := db.NewMemDB()
	refTree, err := iavl.NewMutableTree(d, 0, true)
	if err != nil {
		panic(err)
	}
	for _, changes := range ChangeSets {
		if err := applyChangeSetRef(refTree, changes); err != nil {
			panic(err)
		}
		refHash, _, err := refTree.SaveVersion()
		if err != nil {
			panic(err)
		}
		RefHashes = append(RefHashes, refHash)
	}
}

func applyChangeSetRef(t *iavl.MutableTree, changes iavl.ChangeSet) error {
	for _, change := range changes.Pairs {
		if change.Delete {
			if _, _, err := t.Remove(change.Key); err != nil {
				return err
			}
		} else {
			if _, err := t.Set(change.Key, change.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

func TestRootHashes(t *testing.T) {
	t.Cleanup(removeDefaultWal) // removes default wal file from filesystem

	tree, err := NewEmptyTree(0, DefaultPathToWAL)
	require.NoError(t, err)
	defer os.RemoveAll(DefaultPathToWAL)
	defer tree.bwal.Close()

	for i, changes := range ChangeSets {
		hash, v, err := tree.ApplyChangeSet(changes, true)
		require.NoError(t, err)
		require.Equal(t, i+1, int(v))
		require.Equal(t, RefHashes[i], hash)
	}
}

func TestNewKey(t *testing.T) {
	t.Cleanup(removeDefaultWal) // removes default wal file from filesystem
	tree, err := NewEmptyTree(0, DefaultPathToWAL)
	require.NoError(t, err)

	for i := 0; i < 4; i++ {
		tree.set([]byte(fmt.Sprintf("key-%d", i)), []byte{1})
	}
	_, _, err = tree.saveVersion(true)
	require.NoError(t, err)

	// the smallest key in the right half of the tree
	require.Equal(t, tree.root.Key(), []byte("key-2"))

	// remove this key
	tree.remove([]byte("key-2"))

	// check root node's key is changed
	require.Equal(t, []byte("key-3"), tree.root.Key())
}

func TestEmptyTree(t *testing.T) {
	t.Cleanup(removeDefaultWal)

	tree, err := New(DefaultPathToWAL)
	require.NoError(t, err)

	require.Equal(t, emptyHash, tree.RootHash())
}

func TestWAL(t *testing.T) {
	t.Cleanup(removeDefaultWal) // removes default wal file from filesystem
	tree, err := NewEmptyTree(0, DefaultPathToWAL)
	require.NoError(t, err)

	_, version, err := tree.ApplyChangeSet(DefaultChanges, true)
	require.NoError(t, err)

	// get data from WAL
	data, err := tree.bwal.Read(uint64(version))
	require.NoError(t, err)

	expectedData := []byte{}
	for _, i := range DefaultChangesBz {
		expectedData = append(expectedData, i...)
	}

	require.Equal(t, expectedData, data)
}

func TestReplayWAL(t *testing.T) {
	t.Cleanup(removeDefaultWal) // removes default wal file from filesystem

	tree, err := NewEmptyTree(0, DefaultPathToWAL)
	require.NoError(t, err)
	secondTreeWALPath := "test_wal"

	defer os.RemoveAll(secondTreeWALPath)

	for _, cs := range ChangeSets {
		_, _, err := tree.ApplyChangeSet(cs, false)
		require.NoError(t, err)
	}
	tree.RootHash()
	v := uint64(tree.Version())

	// replay WAL
	tree2, err := NewEmptyTree(0, secondTreeWALPath)
	require.NoError(t, err)
	defer tree2.bwal.Close()

	randomWalPath := "random_wal_path"
	defer os.RemoveAll(randomWalPath)

	tests := map[string]struct {
		untilVersion uint64
		walPath      string
		tree         *Tree
		expectingErr bool
	}{
		"valid: replay all versions": {
			untilVersion: 0,
			tree:         tree2,
			walPath:      DefaultPathToWAL,
		},
		"invalid: untilVersion too high": {
			untilVersion: 100,
			tree:         tree2,
			walPath:      DefaultPathToWAL,
			expectingErr: true,
		},
		"invalid: tree already up to date": {
			untilVersion: 0,
			tree:         tree,
			walPath:      DefaultPathToWAL,
			expectingErr: true,
		},
		"invalid: tree already up to date (explicit version)": {
			untilVersion: v,
			tree:         tree,
			walPath:      DefaultPathToWAL,
			expectingErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.tree.ReplayWAL(tc.untilVersion, tc.walPath)

			if tc.expectingErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			deepEqualTrees(t, tree, tree2)
		})
	}
}

// deepEqualTrees compares two trees' nodes recursively.
func deepEqualTrees(t *testing.T, tree1 *Tree, tree2 *Tree) {
	if tree1.root == nil && tree2.root == nil {
		return
	} else if tree1.root == nil || tree2.root == nil {
		require.FailNow(t, "one of the trees is nil")
	}
	require.Equal(t, tree1.version, tree2.version)

	deepRecursiveEqual(t, tree1.root, tree2.root)
}

func deepRecursiveEqual(t *testing.T, node1 Node, node2 Node) {
	require.Equal(t, node1.Key(), node2.Key())
	require.Equal(t, node1.Value(), node2.Value())
	require.Equal(t, node1.Height(), node2.Height())
	require.Equal(t, node1.Size(), node2.Size())
	require.Equal(t, node1.Version(), node2.Version())

	if isLeaf(node1) && isLeaf(node2) {
		return
	}

	if node1.Left() != nil {
		deepRecursiveEqual(t, node1.Left(), node2.Left())
	}
	if node1.Right() != nil {
		deepRecursiveEqual(t, node1.Right(), node2.Right())
	}

}

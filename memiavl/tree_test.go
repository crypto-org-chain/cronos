package memiavl

import (
	"fmt"
	"testing"

	"github.com/cosmos/iavl"
	"github.com/stretchr/testify/require"
	db "github.com/tendermint/tm-db"
)

type Change struct {
	Delete     bool
	Key, Value []byte
}

var (
	ChangeSets [][]Change
	RefHashes  [][]byte
)

func init() {
	var changes []Change
	ChangeSets = append(ChangeSets,
		[]Change{{Key: []byte("hello"), Value: []byte("world")}},
		[]Change{{Key: []byte("hello"), Value: []byte("world1")}, {Key: []byte("hello1"), Value: []byte("world1")}},
		[]Change{{Key: []byte("hello2"), Value: []byte("world1")}, {Key: []byte("hello3"), Value: []byte("world1")}},
	)

	changes = nil
	for i := 0; i < 1; i++ {
		changes = append(changes, Change{Key: []byte(fmt.Sprintf("hello%02d", i)), Value: []byte("world1")})
	}

	ChangeSets = append(ChangeSets, changes)
	ChangeSets = append(ChangeSets, []Change{{Key: []byte("hello"), Delete: true}, {Key: []byte("hello19"), Delete: true}})

	changes = nil
	for i := 0; i < 21; i++ {
		changes = append(changes, Change{Key: []byte(fmt.Sprintf("aello%02d", i)), Value: []byte("world1")})
	}
	ChangeSets = append(ChangeSets, changes)

	changes = nil
	for i := 0; i < 21; i++ {
		changes = append(changes, Change{Key: []byte(fmt.Sprintf("aello%02d", i)), Delete: true})
	}
	for i := 0; i < 19; i++ {
		changes = append(changes, Change{Key: []byte(fmt.Sprintf("hello%02d", i)), Delete: true})
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

func applyChangeSet(t *Tree, changes []Change) {
	for _, change := range changes {
		if change.Delete {
			t.Remove(change.Key)
		} else {
			t.Set(change.Key, change.Value)
		}
	}
}

func applyChangeSetRef(t *iavl.MutableTree, changes []Change) error {
	for _, change := range changes {
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
	tree := NewEmptyTree(0)

	for i, changes := range ChangeSets {
		applyChangeSet(tree, changes)
		hash, v, err := tree.SaveVersion(true)
		require.NoError(t, err)
		require.Equal(t, i+1, int(v))
		require.Equal(t, RefHashes[i], hash)
	}
}

func TestNewKey(t *testing.T) {
	tree := NewEmptyTree(0)
	for i := 0; i < 4; i++ {
		tree.Set([]byte(fmt.Sprintf("key-%d", i)), []byte{1})
	}
	_, _, err := tree.SaveVersion(true)
	require.NoError(t, err)

	// the smallest key in the right half of the tree
	require.Equal(t, tree.root.Key(), []byte("key-2"))

	// remove this key
	tree.Remove([]byte("key-2"))

	// check root node's key is changed
	require.Equal(t, []byte("key-3"), tree.root.Key())
}

func TestEmptyTree(t *testing.T) {
	tree := New()
	require.Equal(t, emptyHash, tree.RootHash())
}

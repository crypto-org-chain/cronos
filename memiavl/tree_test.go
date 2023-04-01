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
	tree := NewEmptyTree(0)
	defer os.RemoveAll(DefaultPathToWAL)
	defer tree.bwal.Close()

	for i, changes := range ChangeSets {
		hash, v, err := tree.ApplyChangeSet(&changes, true)
		require.NoError(t, err)
		require.Equal(t, i+1, int(v))
		require.Equal(t, RefHashes[i], hash)
	}
}

func TestNewKey(t *testing.T) {
	tree := NewEmptyTree(0)

	defer os.RemoveAll(DefaultPathToWAL)
	defer tree.bwal.Close()

	for i := 0; i < 4; i++ {
		tree.set([]byte(fmt.Sprintf("key-%d", i)), []byte{1})
	}
	_, _, err := tree.saveVersion(true)
	require.NoError(t, err)

	// the smallest key in the right half of the tree
	require.Equal(t, tree.root.Key(), []byte("key-2"))

	// remove this key
	tree.remove([]byte("key-2"))

	// check root node's key is changed
	require.Equal(t, []byte("key-3"), tree.root.Key())
}

func TestEmptyTree(t *testing.T) {
	tree := New()

	defer os.RemoveAll(DefaultPathToWAL)
	defer tree.bwal.Close()

	require.Equal(t, emptyHash, tree.RootHash())
}

func TestWAL(t *testing.T) {
	tree := NewEmptyTree(0)

	defer os.RemoveAll(DefaultPathToWAL)
	defer tree.bwal.Close()

	tree.Set([]byte("hello"), []byte("world"))
	tree.Set([]byte("hello1"), []byte("world1"))
	tree.Remove([]byte("hello1"))
	tree.Set([]byte("hello2"), []byte("world2"))

	// save version
	_, version, err := tree.SaveVersion(true)
	require.NoError(t, err)

	// get data from WAL
	data, err := tree.bwal.Read(uint64(version))
	require.NoError(t, err)

	changesBz := []ChangeBz{
		[]byte("\x00\x05\x00\x00\x00hello\x05\x00\x00\x00world"), // PS: multiple <\x00> because we want 4 bytes as a key/value length field
		[]byte("\x00\x06\x00\x00\x00hello1\x06\x00\x00\x00world1"),
		[]byte("\x01\x06\x00\x00\x00hello1"),
		[]byte("\x00\x06\x00\x00\x00hello2\x06\x00\x00\x00world2"),
	}

	expectedData := BlockChangesBz{}
	for _, i := range changesBz {
		expectedData = append(expectedData, i...)
	}

	require.Equal(t, expectedData, data)
}

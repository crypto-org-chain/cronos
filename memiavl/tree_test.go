package memiavl

import (
	"fmt"
	"math/rand"
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
	tree := NewEmptyTree(0, DefaultPathToWAL)
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
	tree := NewEmptyTree(0, DefaultPathToWAL)

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
	tree := New(DefaultPathToWAL)

	defer os.RemoveAll(DefaultPathToWAL)
	defer tree.bwal.Close()

	require.Equal(t, emptyHash, tree.RootHash())
}

func TestWAL(t *testing.T) {
	tree := NewEmptyTree(0, DefaultPathToWAL)

	defer os.RemoveAll(DefaultPathToWAL)
	defer tree.bwal.Close()

	tree.set([]byte("hello"), []byte("world"))
	tree.set([]byte("hello1"), []byte("world1"))
	tree.remove([]byte("hello1"))
	tree.set([]byte("hello2"), []byte("world2"))

	// save version
	_, version, err := tree.saveVersion(true)
	require.NoError(t, err)

	// get data from WAL
	data, err := tree.bwal.Read(uint64(version))
	require.NoError(t, err)

	changesBz := [][]byte{
		[]byte("\x00\x05hello\x05world"),
		[]byte("\x00\x06hello1\x06world1"),
		[]byte("\x01\x06hello1"),
		[]byte("\x00\x06hello2\x06world2"),
	}

	expectedData := []byte{}
	for _, i := range changesBz {
		expectedData = append(expectedData, i...)
	}

	require.Equal(t, expectedData, data)
}

func TestReplayWAL(t *testing.T) {
	tree := NewEmptyTree(0, DefaultPathToWAL)
	secondTreeWALPath := "test_wal"

	defer os.RemoveAll(DefaultPathToWAL)
	defer os.RemoveAll(secondTreeWALPath)
	defer tree.bwal.Close()

	// FOR REVIEW: maybe it is not worth it randomizing this test?
	var randKeys [][]byte
	// apply 50 random sets
	for i := 0; i < 50; i++ {
		randomKey, randomValue := randomSet()
		tree.set(randomKey, randomValue)

		randKeys = append(randKeys, randomKey)
	}
	_, _, err := tree.saveVersion(true)
	require.NoError(t, err)

	// apply 7 random removes
	for i := 0; i < 7; i++ {
		randomRemoveIndex := randomRemove(randKeys)
		tree.remove(randKeys[randomRemoveIndex])
		randKeys = removeIndex(randKeys, randomRemoveIndex)
	}

	// save version
	_, version, err := tree.saveVersion(false)
	require.NoError(t, err)

	// replay WAL
	tree2 := NewEmptyTree(0, secondTreeWALPath)
	defer tree2.bwal.Close()
	err = tree2.ReplayWAL(uint64(version), DefaultPathToWAL) // using wal from tree 1
	require.NoError(t, err)

	fmt.Println(tree2.version)
	deepEqualTrees(t, tree, tree2)
}

func randomSet() ([]byte, []byte) {
	key := make([]byte, 32)
	value := make([]byte, 32)
	rand.Read(key)
	rand.Read(value)
	return key, value
}

func randomRemove(allKeys [][]byte) int {
	randint := rand.Int()
	return randint % len(allKeys)
}

func removeIndex(s [][]byte, index int) [][]byte {
	ret := make([][]byte, 0)
	ret = append(ret, s[:index]...)
	return append(ret, s[index+1:]...)
}

// deepEqualTrees compares two trees' nodes recursively.
func deepEqualTrees(t *testing.T, tree1 *Tree, tree2 *Tree) {
	require.Equal(t, tree1.version, tree2.version)
	if tree1.root == nil && tree2.root == nil {
		return
	} else if tree1.root == nil || tree2.root == nil {
		require.Fail(t, "only one of trees has nil root")
	}

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

package memiavl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnapshotEncodingRoundTrip(t *testing.T) {
	// setup test tree
	tree := NewEmptyTree(0)
	for _, changes := range ChangeSets[:len(ChangeSets)-1] {
		applyChangeSet(tree, changes)
		_, _, err := tree.SaveVersion(true)
		require.NoError(t, err)
	}

	snapshotDir := t.TempDir()
	require.NoError(t, tree.WriteSnapshot(snapshotDir))

	tree2, snapshot, err := LoadTreeFromSnapshot(snapshotDir)
	require.NoError(t, err)

	require.Equal(t, tree.Version(), tree2.Version())
	require.Equal(t, tree.RootHash(), tree2.RootHash())

	// verify all the node hashes in snapshot
	for i := 0; i < snapshot.NodesLen(); i++ {
		node := snapshot.Node(uint32(i))
		require.Equal(t, node.Hash(), HashNode(node))
	}

	require.NoError(t, snapshot.Close())

	// test modify tree loaded from snapshot
	tree3, snapshot, err := LoadTreeFromSnapshot(snapshotDir)
	require.NoError(t, err)
	applyChangeSet(tree3, ChangeSets[len(ChangeSets)-1])
	hash, v, err := tree3.SaveVersion(true)
	require.NoError(t, err)
	require.Equal(t, RefHashes[len(ChangeSets)-1], hash)
	require.Equal(t, len(ChangeSets), int(v))
	require.NoError(t, snapshot.Close())
}

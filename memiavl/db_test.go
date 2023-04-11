package memiavl

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRewriteSnapshot(t *testing.T) {
	db, err := Load(t.TempDir(), Options{
		CreateIfMissing: true,
		InitialStores:   []string{"test"},
	})
	require.NoError(t, err)

	for i, changes := range ChangeSets {
		cs := MultiChangeSet{
			Changesets: []*NamedChangeSet{
				{
					Name:      "test",
					Changeset: changes,
				},
			},
		}
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			_, v, err := db.Commit(cs)
			require.NoError(t, err)
			require.Equal(t, i+1, int(v))
			require.Equal(t, RefHashes[i], db.lastCommitInfo.StoreInfos[0].CommitId.Hash)
			require.NoError(t, db.RewriteSnapshot())
			require.NoError(t, db.Reload())
		})
	}
}

func TestRewriteSnapshotBackground(t *testing.T) {
	db, err := Load(t.TempDir(), Options{
		CreateIfMissing: true,
		InitialStores:   []string{"test"},
	})
	require.NoError(t, err)

	for i, changes := range ChangeSets {
		cs := MultiChangeSet{
			Changesets: []*NamedChangeSet{
				{
					Name:      "test",
					Changeset: changes,
				},
			},
		}
		_, v, err := db.Commit(cs)
		require.NoError(t, err)
		require.Equal(t, i+1, int(v))
		require.Equal(t, RefHashes[i], db.lastCommitInfo.StoreInfos[0].CommitId.Hash)

		_ = db.RewriteSnapshotBackground()
		time.Sleep(time.Millisecond * 20)
	}
	<-db.snapshotRewriteChan
}

func TestWAL(t *testing.T) {
	dir := t.TempDir()
	db, err := Load(dir, Options{CreateIfMissing: true, InitialStores: []string{"test"}})
	require.NoError(t, err)

	for _, changes := range ChangeSets {
		cs := MultiChangeSet{
			Changesets: []*NamedChangeSet{
				{
					Name:      "test",
					Changeset: changes,
				},
			},
		}
		_, _, err := db.Commit(cs)
		require.NoError(t, err)
	}
	require.NoError(t, db.Close())

	db, err = Load(dir, Options{})
	require.NoError(t, err)

	require.Equal(t, RefHashes[len(RefHashes)-1], db.lastCommitInfo.StoreInfos[0].CommitId.Hash)
}

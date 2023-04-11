package memiavl

import (
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/cosmos/iavl"
	"github.com/stretchr/testify/require"
)

func TestRewriteSnapshot(t *testing.T) {
	db, err := Load(t.TempDir(), Options{
		CreateIfMissing: true,
		InitialStores:   []string{"test"},
	})
	require.NoError(t, err)

	for i, changes := range ChangeSets {
		cs := []*NamedChangeSet{
			{
				Name:      "test",
				Changeset: changes,
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
		cs := []*NamedChangeSet{
			{
				Name:      "test",
				Changeset: changes,
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
	db, err := Load(dir, Options{CreateIfMissing: true, InitialStores: []string{"test", "delete"}})
	require.NoError(t, err)

	for _, changes := range ChangeSets {
		cs := []*NamedChangeSet{
			{
				Name:      "test",
				Changeset: changes,
			},
		}
		_, _, err := db.Commit(cs)
		require.NoError(t, err)
	}

	require.Equal(t, 2, len(db.lastCommitInfo.StoreInfos))

	require.NoError(t, db.ApplyUpgrades([]*TreeNameUpgrade{
		{
			Name:       "newtest",
			RenameFrom: "test",
		},
		{
			Name:   "delete",
			Delete: true,
		},
	}))
	_, _, err = db.Commit(nil)
	require.NoError(t, err)

	require.NoError(t, db.Close())

	db, err = Load(dir, Options{})
	require.NoError(t, err)

	require.Equal(t, "newtest", db.lastCommitInfo.StoreInfos[0].Name)
	require.Equal(t, 1, len(db.lastCommitInfo.StoreInfos))
	require.Equal(t, RefHashes[len(RefHashes)-1], db.lastCommitInfo.StoreInfos[0].CommitId.Hash)
}

func TestInitialVersion(t *testing.T) {
	dir := t.TempDir()
	db, err := Load(dir, Options{CreateIfMissing: true, InitialStores: []string{"test"}})
	require.NoError(t, err)

	db.SetInitialVersion(100)

	hash, v, err := db.Commit([]*NamedChangeSet{
		{
			Name: "test",
			Changeset: iavl.ChangeSet{
				Pairs: []*iavl.KVPair{
					{
						Key:   []byte("hello"),
						Value: []byte("world"),
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, int64(100), v)
	require.Equal(t, "2b650e7f3495c352dbf575759fee86850e4fc63291a5889847890ebf12e3f585", hex.EncodeToString(hash))

	hash, v, err = db.Commit([]*NamedChangeSet{
		{
			Name: "test",
			Changeset: iavl.ChangeSet{
				Pairs: []*iavl.KVPair{
					{
						Key:   []byte("hello"),
						Value: []byte("world1"),
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, int64(101), v)
	require.Equal(t, "b7669ab9c167dcf1bb6e69b88ae8573bda2905f556a8f37a65de6cabd31a552d", hex.EncodeToString(hash))

	require.NoError(t, db.Close())

	db, err = Load(dir, Options{})
	require.NoError(t, err)
	require.Equal(t, uint32(100), db.initialVersion)
	require.Equal(t, int64(101), db.Version())
	require.Equal(t, "b7669ab9c167dcf1bb6e69b88ae8573bda2905f556a8f37a65de6cabd31a552d", hex.EncodeToString(db.Hash()))
}

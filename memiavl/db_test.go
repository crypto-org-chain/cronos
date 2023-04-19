package memiavl

import (
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"testing"

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
		CreateIfMissing:    true,
		InitialStores:      []string{"test"},
		SnapshotKeepRecent: 1,
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

		err = db.RewriteSnapshotBackground()
		require.NoError(t, err)
		for {
			if cleaned, _ := db.cleanupSnapshotRewrite(); cleaned {
				break
			}
		}
	}

	db.pruneSnapshotLock.Lock()
	defer db.pruneSnapshotLock.Unlock()

	entries, err := os.ReadDir(db.dir)
	require.NoError(t, err)
	version := uint64(db.lastCommitInfo.Version)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), SnapshotPrefix) {
			currentVersion, err := strconv.ParseUint(strings.TrimPrefix(entry.Name(), SnapshotPrefix), 10, 32)
			require.NoError(t, err)
			require.GreaterOrEqual(t, currentVersion, version-uint64(db.snapshotKeepRecent))
			require.LessOrEqual(t, currentVersion, version)
		}
	}
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

func mockNameChangeSet(name, key, value string) []*NamedChangeSet {
	return []*NamedChangeSet{
		{
			Name: name,
			Changeset: iavl.ChangeSet{
				Pairs: mockKVPairs(key, value),
			},
		},
	}
}

// 0/1 -> v :1
// ...
// 100 -> v: 100
func TestInitialVersion(t *testing.T) {
	name := "test"
	name1 := "new"
	name2 := "new2"
	key := "hello"
	value := "world"
	for _, initialVersion := range []int64{0, 1, 100} {
		dir := t.TempDir()
		db, err := Load(dir, Options{CreateIfMissing: true, InitialStores: []string{name}})
		require.NoError(t, err)
		db.SetInitialVersion(initialVersion)
		hash, v, err := db.Commit(mockNameChangeSet(name, key, value))
		require.NoError(t, err)
		if initialVersion <= 1 {
			require.Equal(t, int64(1), v)
		} else {
			require.Equal(t, initialVersion, v)
		}
		require.Equal(t, "2b650e7f3495c352dbf575759fee86850e4fc63291a5889847890ebf12e3f585", hex.EncodeToString(hash))
		hash, v, err = db.Commit(mockNameChangeSet(name, key, "world1"))
		require.NoError(t, err)
		if initialVersion <= 1 {
			require.Equal(t, int64(2), v)
			require.Equal(t, "e102a3393cc7a5c0ea115b00bcdf9d77f407040627354b0dde57f6d7edadfd83", hex.EncodeToString(hash))
		} else {
			require.Equal(t, initialVersion+1, v)
			require.Equal(t, "b7669ab9c167dcf1bb6e69b88ae8573bda2905f556a8f37a65de6cabd31a552d", hex.EncodeToString(hash))
		}
		require.NoError(t, db.Close())

		db, err = Load(dir, Options{})
		require.NoError(t, err)
		require.Equal(t, uint32(initialVersion), db.initialVersion)
		require.Equal(t, v, db.Version())
		require.Equal(t, hex.EncodeToString(hash), hex.EncodeToString(hash))

		db.ApplyUpgrades([]*TreeNameUpgrade{{Name: name1}})
		_, v, err = db.Commit((mockNameChangeSet(name1, key, value)))
		require.NoError(t, err)
		if initialVersion <= 1 {
			require.Equal(t, int64(3), v)
		} else {
			require.Equal(t, initialVersion+2, v)
		}
		require.Equal(t, 2, len(db.lastCommitInfo.StoreInfos))
		info := db.lastCommitInfo.StoreInfos[0]
		require.Equal(t, name1, info.Name)
		require.Equal(t, v, info.CommitId.Version)
		require.Equal(t, "6032661ab0d201132db7a8fa1da6a0afe427e6278bd122c301197680ab79ca02", hex.EncodeToString(info.CommitId.Hash))
		// the nodes are created with version 1, which is compatible with iavl behavior: https://github.com/cosmos/iavl/pull/660
		require.Equal(t, info.CommitId.Hash, HashNode(newLeafNode([]byte(key), []byte(value), 1)))

		require.NoError(t, db.RewriteSnapshot())
		require.NoError(t, db.Reload())

		db.ApplyUpgrades([]*TreeNameUpgrade{{Name: name2}})
		_, v, err = db.Commit((mockNameChangeSet(name2, key, value)))
		require.NoError(t, err)
		if initialVersion <= 1 {
			require.Equal(t, int64(4), v)
		} else {
			require.Equal(t, initialVersion+3, v)
		}
		require.Equal(t, 3, len(db.lastCommitInfo.StoreInfos))
		info2 := db.lastCommitInfo.StoreInfos[1]
		require.Equal(t, name2, info2.Name)
		require.Equal(t, v, info2.CommitId.Version)
		require.Equal(t, hex.EncodeToString(info.CommitId.Hash), hex.EncodeToString(info2.CommitId.Hash))
	}
}

func TestLoadVersion(t *testing.T) {
	dir := t.TempDir()
	db, err := Load(dir, Options{
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
			_, _, err := db.Commit(cs)
			require.NoError(t, err)
		})
	}

	for v, expItems := range ExpectItems {
		if v == 0 {
			continue
		}
		tmp, err := Load(dir, Options{
			TargetVersion: uint32(v),
		})
		require.NoError(t, err)
		require.Equal(t, RefHashes[v-1], tmp.TreeByName("test").RootHash())
		require.Equal(t, expItems, collectIter(tmp.TreeByName("test").Iterator(nil, nil, true)))
	}
}

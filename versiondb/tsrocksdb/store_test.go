package tsrocksdb

import (
	"encoding/binary"
	"testing"

	"github.com/crypto-org-chain/cronos/versiondb"
	"github.com/linxGnu/grocksdb"
	"github.com/stretchr/testify/require"
)

func TestTSVersionDB(t *testing.T) {
	versiondb.Run(t, func() versiondb.VersionStore {
		store, err := NewStore(t.TempDir())
		require.NoError(t, err)
		return store
	})
}

func TestUserTimestamp(t *testing.T) {
	db, cfHandle, err := OpenVersionDB(t.TempDir())
	require.NoError(t, err)

	var ts [8]byte
	binary.LittleEndian.PutUint64(ts[:], 1000)

	err = db.PutCFWithTS(grocksdb.NewDefaultWriteOptions(), cfHandle, []byte("hello"), ts[:], []byte{1})
	require.NoError(t, err)
	err = db.PutCFWithTS(grocksdb.NewDefaultWriteOptions(), cfHandle, []byte("zempty"), ts[:], []byte{})
	require.NoError(t, err)

	v := int64(999)
	bz, err := db.GetCF(newTSReadOptions(&v), cfHandle, []byte("hello"))
	require.NoError(t, err)
	require.False(t, bz.Exists())
	bz.Free()

	bz, err = db.GetCF(newTSReadOptions(nil), cfHandle, []byte("hello"))
	require.NoError(t, err)
	require.Equal(t, []byte{1}, bz.Data())
	bz.Free()

	v = int64(1000)
	it := db.NewIteratorCF(newTSReadOptions(&v), cfHandle)
	it.SeekToFirst()
	require.True(t, it.Valid())
	require.Equal(t, []byte("hello"), it.Key().Data())

	bz, err = db.GetCF(newTSReadOptions(&v), cfHandle, []byte("zempty"))
	require.NoError(t, err)
	require.Equal(t, []byte{}, bz.Data())
	bz.Free()

	binary.LittleEndian.PutUint64(ts[:], 1002)
	err = db.PutCFWithTS(grocksdb.NewDefaultWriteOptions(), cfHandle, []byte("hella"), ts[:], []byte{2})
	require.NoError(t, err)

	v = int64(1002)
	it = db.NewIteratorCF(newTSReadOptions(&v), cfHandle)
	it.SeekToFirst()
	require.True(t, it.Valid())
	require.Equal(t, []byte("hella"), it.Key().Data())
}

package tsrocksdb

import (
	"encoding/binary"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/crypto-org-chain/cronos/versiondb"
	"github.com/linxGnu/grocksdb"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/store/types"
)

func TestTSVersionDB(t *testing.T) {
	versiondb.Run(t, func() versiondb.VersionStore {
		store, err := NewStore(t.TempDir())
		require.NoError(t, err)
		return store
	})
}

// TestUserTimestamp tests the behaviors of user-defined timestamp feature of rocksdb
func TestUserTimestampBasic(t *testing.T) {
	key := []byte("hello")
	writeOpts := grocksdb.NewDefaultWriteOptions()

	db, cfHandle, err := OpenVersionDB(t.TempDir())
	require.NoError(t, err)

	var ts [8]byte
	binary.LittleEndian.PutUint64(ts[:], 1000)

	err = db.PutCFWithTS(writeOpts, cfHandle, key, ts[:], []byte{1})
	require.NoError(t, err)
	err = db.PutCFWithTS(writeOpts, cfHandle, []byte("zempty"), ts[:], []byte{})
	require.NoError(t, err)

	// key don't exists in older version
	v := int64(999)
	bz, err := db.GetCF(newTSReadOptions(&v), cfHandle, key)
	require.NoError(t, err)
	require.False(t, bz.Exists())
	bz.Free()

	// key exists in latest version
	bz, err = db.GetCF(newTSReadOptions(nil), cfHandle, key)
	require.NoError(t, err)
	require.Equal(t, []byte{1}, bz.Data())
	bz.Free()

	// iterator can find the key in right version
	v = int64(1000)
	it := db.NewIteratorCF(newTSReadOptions(&v), cfHandle)
	it.SeekToFirst()
	require.True(t, it.Valid())
	bz = it.Key()
	require.Equal(t, key, bz.Data())
	bz.Free()

	// key exists in right version, and empty value is supported
	bz, err = db.GetCF(newTSReadOptions(&v), cfHandle, []byte("zempty"))
	require.NoError(t, err)
	require.Equal(t, []byte{}, bz.Data())
	bz.Free()

	binary.LittleEndian.PutUint64(ts[:], 1002)
	err = db.PutCFWithTS(writeOpts, cfHandle, []byte("hella"), ts[:], []byte{2})
	require.NoError(t, err)

	// iterator can find keys from both versions
	v = int64(1002)
	it = db.NewIteratorCF(newTSReadOptions(&v), cfHandle)
	it.SeekToFirst()
	require.True(t, it.Valid())
	bz = it.Key()
	require.Equal(t, []byte("hella"), bz.Data())
	bz.Free()

	it.Next()
	require.True(t, it.Valid())
	bz = it.Key()
	require.Equal(t, key, bz.Data())
	bz.Free()

	for i := 1; i < 100; i++ {
		binary.LittleEndian.PutUint64(ts[:], uint64(i))
		err := db.PutCFWithTS(writeOpts, cfHandle, key, ts[:], []byte{byte(i)})
		require.NoError(t, err)
	}

	for i := int64(1); i < 100; i++ {
		binary.LittleEndian.PutUint64(ts[:], uint64(i))
		bz, err := db.GetCF(newTSReadOptions(&i), cfHandle, key)
		require.NoError(t, err)
		require.Equal(t, []byte{byte(i)}, bz.Data())
		bz.Free()
	}
}

func TestUserTimestampPruning(t *testing.T) {
	key := []byte("hello")
	writeOpts := grocksdb.NewDefaultWriteOptions()

	dir := t.TempDir()
	db, cfHandle, err := OpenVersionDB(dir)
	require.NoError(t, err)

	var ts [TimestampSize]byte
	for _, i := range []uint64{1, 100, 200} {
		binary.LittleEndian.PutUint64(ts[:], i)
		err := db.PutCFWithTS(writeOpts, cfHandle, key, ts[:], []byte{byte(i)})
		require.NoError(t, err)
	}

	i := int64(49)

	bz, err := db.GetCF(newTSReadOptions(&i), cfHandle, key)
	require.NoError(t, err)
	require.True(t, bz.Exists())
	bz.Free()

	// prune old versions
	binary.LittleEndian.PutUint64(ts[:], 50)
	compactOpts := grocksdb.NewCompactRangeOptions()
	compactOpts.SetFullHistoryTsLow(ts[:])
	db.CompactRangeCFOpt(cfHandle, grocksdb.Range{}, compactOpts)

	// queries for versions older than 50 are not allowed
	_, err = db.GetCF(newTSReadOptions(&i), cfHandle, key)
	require.Error(t, err)

	// the value previously at version 1 is still there
	i = 50
	bz, err = db.GetCF(newTSReadOptions(&i), cfHandle, key)
	require.NoError(t, err)
	require.True(t, bz.Exists())
	require.Equal(t, []byte{1}, bz.Data())
	bz.Free()

	i = 200
	bz, err = db.GetCF(newTSReadOptions(&i), cfHandle, key)
	require.NoError(t, err)
	require.Equal(t, []byte{200}, bz.Data())
	bz.Free()

	// reopen db and trim version 200
	cfHandle.Destroy()
	db.Close()
	db, cfHandle, err = OpenVersionDBAndTrimHistory(dir, 199)
	require.NoError(t, err)

	// the version 200 is gone, 100 is the latest value
	bz, err = db.GetCF(newTSReadOptions(&i), cfHandle, key)
	require.NoError(t, err)
	require.Equal(t, []byte{100}, bz.Data())
	bz.Free()
}

func TestSkipVersionZero(t *testing.T) {
	storeKey := "test"

	var wrongTz [8]byte
	binary.LittleEndian.PutUint64(wrongTz[:], 100)

	key1 := []byte("hello1")
	key2 := []byte("hello2")
	key2Wrong := cloneAppend(key2, wrongTz[:])
	key3 := []byte("hello3")

	store, err := NewStore(t.TempDir())
	require.NoError(t, err)

	err = store.PutAtVersion(0, []*types.StoreKVPair{
		{StoreKey: storeKey, Key: key2Wrong, Value: []byte{2}},
	})
	require.NoError(t, err)
	err = store.PutAtVersion(100, []*types.StoreKVPair{
		{StoreKey: storeKey, Key: key1, Value: []byte{1}},
	})
	require.NoError(t, err)
	err = store.PutAtVersion(100, []*types.StoreKVPair{
		{StoreKey: storeKey, Key: key3, Value: []byte{3}},
	})
	require.NoError(t, err)

	i := int64(999)
	bz, err := store.GetAtVersion(storeKey, key2Wrong, &i)
	require.NoError(t, err)
	require.Equal(t, []byte{2}, bz)

	it, err := store.IteratorAtVersion(storeKey, nil, nil, &i)
	require.NoError(t, err)
	require.Equal(t,
		[]kvPair{
			{Key: key1, Value: []byte{1}},
			{Key: key2Wrong, Value: []byte{2}},
			{Key: key3, Value: []byte{3}},
		},
		consumeIterator(it),
	)

	store.SetSkipVersionZero(true)

	bz, err = store.GetAtVersion(storeKey, key2Wrong, &i)
	require.NoError(t, err)
	require.Empty(t, bz)
	bz, err = store.GetAtVersion(storeKey, key1, &i)
	require.NoError(t, err)
	require.Equal(t, []byte{1}, bz)

	it, err = store.IteratorAtVersion(storeKey, nil, nil, &i)
	require.NoError(t, err)
	require.Equal(t,
		[]kvPair{
			{Key: key1, Value: []byte{1}},
			{Key: key3, Value: []byte{3}},
		},
		consumeIterator(it),
	)

	store.SetSkipVersionZero(false)
	err = store.FixData([]string{storeKey}, false)
	require.NoError(t, err)

	bz, err = store.GetAtVersion(storeKey, key2, &i)
	require.NoError(t, err)
	require.Equal(t, []byte{2}, bz)
}

type kvPair struct {
	Key   []byte
	Value []byte
}

func consumeIterator(it dbm.Iterator) []kvPair {
	var result []kvPair
	for ; it.Valid(); it.Next() {
		result = append(result, kvPair{it.Key(), it.Value()})
	}
	it.Close()
	return result
}

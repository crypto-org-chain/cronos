package tsrocksdb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"cosmossdk.io/store/types"
	"github.com/cosmos/iavl"
	"github.com/crypto-org-chain/cronos/versiondb"
	"github.com/linxGnu/grocksdb"
)

const (
	TimestampSize = 8

	StorePrefixTpl   = "s/k:%s/"
	latestVersionKey = "s/latest"

	ImportCommitBatchSize = 10000
)

var (
	errKeyEmpty = errors.New("key cannot be empty")

	_ versiondb.VersionStore = Store{}

	defaultWriteOpts     = grocksdb.NewDefaultWriteOptions()
	defaultSyncWriteOpts = grocksdb.NewDefaultWriteOptions()
	defaultReadOpts      = grocksdb.NewDefaultReadOptions()
)

func init() {
	defaultSyncWriteOpts.SetSync(true)
}

type Store struct {
	db       *grocksdb.DB
	cfHandle *grocksdb.ColumnFamilyHandle

	// see: https://github.com/crypto-org-chain/cronos/issues/1683
	skipVersionZero bool
}

func NewStore(dir string) (Store, error) {
	db, cfHandle, err := OpenVersionDB(dir)
	if err != nil {
		return Store{}, err
	}
	return Store{
		db:       db,
		cfHandle: cfHandle,
	}, nil
}

func NewStoreWithDB(db *grocksdb.DB, cfHandle *grocksdb.ColumnFamilyHandle) Store {
	return Store{
		db:       db,
		cfHandle: cfHandle,
	}
}

func (s *Store) SetSkipVersionZero(skip bool) {
	s.skipVersionZero = skip
}

func (s Store) SetLatestVersion(version int64) error {
	var ts [TimestampSize]byte
	binary.LittleEndian.PutUint64(ts[:], uint64(version))
	return s.db.Put(defaultWriteOpts, []byte(latestVersionKey), ts[:])
}

// PutAtVersion implements VersionStore interface
func (s Store) PutAtVersion(version int64, changeSet []*types.StoreKVPair) error {
	var ts [TimestampSize]byte
	binary.LittleEndian.PutUint64(ts[:], uint64(version))

	batch := grocksdb.NewWriteBatch()
	defer batch.Destroy()
	batch.Put([]byte(latestVersionKey), ts[:])

	for _, pair := range changeSet {
		key := prependStoreKey(pair.StoreKey, pair.Key)
		if pair.Delete {
			batch.DeleteCFWithTS(s.cfHandle, key, ts[:])
		} else {
			batch.PutCFWithTS(s.cfHandle, key, ts[:], pair.Value)
		}
	}

	return s.db.Write(defaultSyncWriteOpts, batch)
}

func (s Store) GetAtVersionSlice(storeKey string, key []byte, version *int64) (*grocksdb.Slice, error) {
	value, ts, err := s.db.GetCFWithTS(
		newTSReadOptions(version),
		s.cfHandle,
		prependStoreKey(storeKey, key),
	)
	if err != nil {
		return nil, err
	}
	defer ts.Free()

	if value.Exists() && s.skipVersionZero {
		if binary.LittleEndian.Uint64(ts.Data()) == 0 {
			return nil, nil
		}
	}

	return value, err
}

// GetAtVersion implements VersionStore interface
func (s Store) GetAtVersion(storeKey string, key []byte, version *int64) ([]byte, error) {
	slice, err := s.GetAtVersionSlice(storeKey, key, version)
	if err != nil {
		return nil, err
	}
	return moveSliceToBytes(slice), nil
}

// HasAtVersion implements VersionStore interface
func (s Store) HasAtVersion(storeKey string, key []byte, version *int64) (bool, error) {
	slice, err := s.GetAtVersionSlice(storeKey, key, version)
	if err != nil {
		return false, err
	}
	defer slice.Free()
	return slice.Exists(), nil
}

// GetLatestVersion returns the latest version stored in plain state,
// it's committed after the changesets, so the data for this version is guaranteed to be persisted.
// returns -1 if the key don't exists.
func (s Store) GetLatestVersion() (int64, error) {
	bz, err := s.db.GetBytes(defaultReadOpts, []byte(latestVersionKey))
	if err != nil {
		return 0, err
	}
	if len(bz) == 0 {
		return 0, nil
	}
	return int64(binary.LittleEndian.Uint64(bz)), nil
}

// IteratorAtVersion implements VersionStore interface
func (s Store) IteratorAtVersion(storeKey string, start, end []byte, version *int64) (types.Iterator, error) {
	return s.iteratorAtVersion(storeKey, start, end, version, false)
}

// ReverseIteratorAtVersion implements VersionStore interface
func (s Store) ReverseIteratorAtVersion(storeKey string, start, end []byte, version *int64) (types.Iterator, error) {
	return s.iteratorAtVersion(storeKey, start, end, version, true)
}

func (s Store) iteratorAtVersion(storeKey string, start, end []byte, version *int64, reverse bool) (types.Iterator, error) {
	if (start != nil && len(start) == 0) || (end != nil && len(end) == 0) {
		return nil, errKeyEmpty
	}

	prefix := storePrefix(storeKey)
	start, end = iterateWithPrefix(prefix, start, end)

	itr := s.db.NewIteratorCF(newTSReadOptions(version), s.cfHandle)
	return newRocksDBIterator(itr, prefix, start, end, reverse, s.skipVersionZero), nil
}

// FeedChangeSet is used to migrate legacy change sets into versiondb
func (s Store) FeedChangeSet(version int64, store string, changeSet *iavl.ChangeSet) error {
	var ts [TimestampSize]byte
	binary.LittleEndian.PutUint64(ts[:], uint64(version))

	prefix := storePrefix(store)

	batch := grocksdb.NewWriteBatch()
	defer batch.Destroy()

	for _, pair := range changeSet.Pairs {
		key := cloneAppend(prefix, pair.Key)

		if pair.Delete {
			batch.DeleteCFWithTS(s.cfHandle, key, ts[:])
		} else {
			batch.PutCFWithTS(s.cfHandle, key, ts[:], pair.Value)
		}
	}

	return s.db.Write(defaultWriteOpts, batch)
}

// Import loads the initial version of the state
func (s Store) Import(version int64, ch <-chan versiondb.ImportEntry) error {
	batch := grocksdb.NewWriteBatch()
	defer batch.Destroy()

	var ts [TimestampSize]byte
	binary.LittleEndian.PutUint64(ts[:], uint64(version))

	var counter int
	for entry := range ch {
		key := cloneAppend(storePrefix(entry.StoreKey), entry.Key)
		batch.PutCFWithTS(s.cfHandle, key, ts[:], entry.Value)

		counter++
		if counter%ImportCommitBatchSize == 0 {
			if err := s.db.Write(defaultWriteOpts, batch); err != nil {
				return err
			}
			batch.Clear()
		}
	}

	if batch.Count() > 0 {
		if err := s.db.Write(defaultWriteOpts, batch); err != nil {
			return err
		}
	}

	return s.SetLatestVersion(version)
}

func (s Store) Flush() error {
	opts := grocksdb.NewDefaultFlushOptions()
	defer opts.Destroy()

	return errors.Join(
		s.db.Flush(opts),
		s.db.FlushCF(s.cfHandle, opts),
	)
}

// FixData fixes wrong data written in versiondb due to rocksdb upgrade, the operation is idempotent.
// see: https://github.com/crypto-org-chain/cronos/issues/1683
// call this before `SetSkipVersionZero(true)`.
func (s Store) FixData(storeKeys []types.StoreKey) error {
	for _, storeKey := range storeKeys {
		if err := s.fixDataStore(storeKey.Name()); err != nil {
			return err
		}
	}

	return nil
}

// fixDataStore iterate the wrong data at version 0, parse the timestamp from the key and write it again.
func (s Store) fixDataStore(storeName string) error {
	var version int64
	iter, err := s.IteratorAtVersion(storeName, nil, nil, &version)
	if err != nil {
		return err
	}
	defer iter.Close()

	batch := grocksdb.NewWriteBatch()
	defer batch.Destroy()

	prefix := storePrefix(storeName)
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if len(key) < TimestampSize {
			return fmt.Errorf("invalid key length: %X", key)
		}

		ts := key[len(key)-TimestampSize:]
		key = key[:len(key)-TimestampSize]
		realKey := cloneAppend(prefix, key)

		readOpts := grocksdb.NewDefaultReadOptions()
		readOpts.SetTimestamp(ts)
		oldValue, err := s.db.GetCF(readOpts, s.cfHandle, realKey)
		if err != nil {
			return err
		}
		readOpts.Destroy()

		clean := bytes.Equal(oldValue.Data(), iter.Value())
		oldValue.Free()

		if clean {
			continue
		}

		batch.PutCFWithTS(s.cfHandle, realKey, ts, iter.Value())
	}

	return s.db.Write(defaultSyncWriteOpts, batch)
}

func newTSReadOptions(version *int64) *grocksdb.ReadOptions {
	var ver uint64
	if version == nil {
		ver = math.MaxUint64
	} else {
		ver = uint64(*version)
	}

	var ts [TimestampSize]byte
	binary.LittleEndian.PutUint64(ts[:], ver)

	readOpts := grocksdb.NewDefaultReadOptions()
	readOpts.SetTimestamp(ts[:])
	return readOpts
}

func storePrefix(storeKey string) []byte {
	return []byte(fmt.Sprintf(StorePrefixTpl, storeKey))
}

// prependStoreKey prepends storeKey to the key
func prependStoreKey(storeKey string, key []byte) []byte {
	return append(storePrefix(storeKey), key...)
}

func cloneAppend(bz []byte, tail []byte) (res []byte) {
	res = make([]byte, len(bz)+len(tail))
	copy(res, bz)
	copy(res[len(bz):], tail)
	return
}

// Returns a slice of the same length (big endian)
// except incremented by one.
// Returns nil on overflow (e.g. if bz bytes are all 0xFF)
// CONTRACT: len(bz) > 0
func cpIncr(bz []byte) (ret []byte) {
	if len(bz) == 0 {
		panic("cpIncr expects non-zero bz length")
	}
	ret = make([]byte, len(bz))
	copy(ret, bz)
	for i := len(bz) - 1; i >= 0; i-- {
		if ret[i] < byte(0xFF) {
			ret[i]++
			return
		}
		ret[i] = byte(0x00)
		if i == 0 {
			// Overflow
			return nil
		}
	}
	return nil
}

// iterateWithPrefix calculate the acual iterate range
func iterateWithPrefix(prefix, begin, end []byte) ([]byte, []byte) {
	if len(prefix) == 0 {
		return begin, end
	}

	begin = cloneAppend(prefix, begin)

	if end == nil {
		end = cpIncr(prefix)
	} else {
		end = cloneAppend(prefix, end)
	}

	return begin, end
}

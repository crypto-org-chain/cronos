package versiondb

import (
	"io"
	"time"

	"cosmossdk.io/store/cachekv"
	"cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
)

const StoreTypeVersionDB = 100

var _ types.KVStore = (*Store)(nil)

// Store Implements types.KVStore
type Store struct {
	store   VersionStore
	name    string
	version *int64
}

func NewKVStore(store VersionStore, storeKey string, version *int64) *Store {
	return &Store{store, storeKey, version}
}

// GetStoreType Implements Store.
func (st *Store) GetStoreType() types.StoreType {
	// should have effect, just define an unique indentifier, don't be conflicts with cosmos-sdk's builtin ones.
	return StoreTypeVersionDB
}

// CacheWrap Implements Store.
func (st *Store) CacheWrap() types.CacheWrap {
	return cachekv.NewStore(st)
}

// Get Implements types.KVStore.
func (st *Store) Get(key []byte) []byte {
	defer telemetry.MeasureSince(time.Now(), "store", "versiondb", "get")
	value, err := st.store.GetAtVersion(st.name, key, st.version)
	if err != nil {
		panic(err)
	}
	return value
}

// Has Implements types.KVStore.
func (st *Store) Has(key []byte) (exists bool) {
	defer telemetry.MeasureSince(time.Now(), "store", "versiondb", "has")
	has, err := st.store.HasAtVersion(st.name, key, st.version)
	if err != nil {
		panic(err)
	}
	return has
}

// Iterator Implements types.KVStore.
func (st *Store) Iterator(start, end []byte) types.Iterator {
	itr, err := st.store.IteratorAtVersion(st.name, start, end, st.version)
	if err != nil {
		panic(err)
	}
	return itr
}

// ReverseIterator Implements types.KVStore.
func (st *Store) ReverseIterator(start, end []byte) types.Iterator {
	itr, err := st.store.ReverseIteratorAtVersion(st.name, start, end, st.version)
	if err != nil {
		panic(err)
	}
	return itr
}

// Set Implements types.KVStore.
func (st *Store) Set(key, value []byte) {
	panic("write operation is not supported")
}

// Delete Implements types.KVStore.
func (st *Store) Delete(key []byte) {
	panic("write operation is not supported")
}

func (st *Store) CacheWrapWithTrace(w io.Writer, tc types.TraceContext) types.CacheWrap {
	panic("not implemented")
}

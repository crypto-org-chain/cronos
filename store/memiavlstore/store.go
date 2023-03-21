package memiavlstore

import (
	"bytes"
	"io"

	"github.com/cosmos/cosmos-sdk/store/tracekv"
	"github.com/cosmos/iavl"
	"github.com/crypto-org-chain/cronos/memiavl"
	"github.com/tendermint/tendermint/libs/log"

	pruningtypes "github.com/cosmos/cosmos-sdk/pruning/types"
	"github.com/cosmos/cosmos-sdk/store/cachekv"
	"github.com/cosmos/cosmos-sdk/store/types"
)

var (
	_ types.KVStore       = (*Store)(nil)
	_ types.CommitStore   = (*Store)(nil)
	_ types.CommitKVStore = (*Store)(nil)
)

// Store Implements types.KVStore and CommitKVStore.
type Store struct {
	tree   memiavl.Tree
	logger log.Logger

	changeSet iavl.ChangeSet
}

func LoadStoreWithInitialVersion(dir string, logger log.Logger, initialVersion int64) (types.CommitKVStore, error) {
	tree, err := memiavl.Load(dir, initialVersion)
	if err != nil {
		return nil, err
	}
	return &Store{tree: *tree, logger: logger}, nil
}

func (st *Store) Commit() types.CommitID {
	hash, version, err := st.tree.ApplyChangeSet(&st.changeSet, true)
	if err != nil {
		panic(err)
	}
	st.changeSet.Pairs = st.changeSet.Pairs[:0]

	return types.CommitID{
		Version: version,
		Hash:    hash,
	}
}

func (st *Store) LastCommitID() types.CommitID {
	hash := st.tree.RootHash()
	return types.CommitID{
		Version: st.tree.Version(),
		Hash:    hash,
	}
}

// SetPruning panics as pruning options should be provided at initialization
// since IAVl accepts pruning options directly.
func (st *Store) SetPruning(_ pruningtypes.PruningOptions) {
	panic("cannot set pruning options on an initialized IAVL store")
}

// SetPruning panics as pruning options should be provided at initialization
// since IAVl accepts pruning options directly.
func (st *Store) GetPruning() pruningtypes.PruningOptions {
	panic("cannot get pruning options on an initialized IAVL store")
}

// Implements Store.
func (st *Store) GetStoreType() types.StoreType {
	return types.StoreTypeIAVL
}

func (st *Store) CacheWrap() types.CacheWrap {
	return cachekv.NewStore(st)
}

// CacheWrapWithTrace implements the Store interface.
func (st *Store) CacheWrapWithTrace(w io.Writer, tc types.TraceContext) types.CacheWrap {
	return cachekv.NewStore(tracekv.NewStore(st, w, tc))
}

// Implements types.KVStore.
//
// we assume Set is only called in `Commit`, so the written state is only visible after commit.
func (st *Store) Set(key, value []byte) {
	st.changeSet.Pairs = append(st.changeSet.Pairs, iavl.KVPair{
		Key: key, Value: value,
	})
}

// Implements types.KVStore.
func (st *Store) Get(key []byte) []byte {
	return bytes.Clone(st.tree.Get(key))
}

// Implements types.KVStore.
func (st *Store) Has(key []byte) bool {
	return st.tree.Has(key)
}

// Implements types.KVStore.
//
// we assume Delete is only called in `Commit`, so the written state is only visible after commit.
func (st *Store) Delete(key []byte) {
	st.changeSet.Pairs = append(st.changeSet.Pairs, iavl.KVPair{
		Key: key, Delete: true,
	})
}

func (st *Store) Iterator(start, end []byte) types.Iterator {
	return st.tree.Iterator(start, end, true)
}

func (st *Store) ReverseIterator(start, end []byte) types.Iterator {
	return st.tree.Iterator(start, end, false)
}

// SetInitialVersion sets the initial version of the IAVL tree. It is used when
// starting a new chain at an arbitrary height.
// implements interface StoreWithInitialVersion
func (st *Store) SetInitialVersion(version int64) {
	st.tree.SetInitialVersion(version)
}

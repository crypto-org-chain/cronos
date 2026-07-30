package mempool

import (
	"testing"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
)

// cacheMultiStoreIface is a defined (non-alias) copy of the interface, so
// embedding it below doesn't name-collide with the interface's own
// CacheMultiStore() method (an anonymous storetypes.CacheMultiStore field
// would be named "CacheMultiStore", shadowing that promoted method).
type cacheMultiStoreIface storetypes.CacheMultiStore

// fakeCacheStore is a minimal storetypes.CacheMultiStore double used purely as
// an identity marker: it embeds a nil CacheMultiStore to satisfy the
// interface, so a test using it must never invoke an unoverridden method.
type fakeCacheStore struct {
	cacheMultiStoreIface
}

func newFakeCacheStore() *fakeCacheStore { return &fakeCacheStore{} }

// fakeCommitStore is a minimal storetypes.CommitMultiStore double whose only
// live method is CacheMultiStore, standing in for BaseApp.CommitMultiStore().
type fakeCommitStore struct {
	storetypes.CommitMultiStore
	cache storetypes.CacheMultiStore
}

func (f *fakeCommitStore) CacheMultiStore() storetypes.CacheMultiStore { return f.cache }

func TestMempoolState_StoreNilOnNilReceiver(t *testing.T) {
	var s *mempoolState
	if got := s.store(); got != nil {
		t.Fatalf("nil *mempoolState must report nil store, got %v", got)
	}
}

func TestMempoolState_StoreNilOnNilBase(t *testing.T) {
	s := &mempoolState{}
	if got := s.store(); got != nil {
		t.Fatalf("unrefreshed mempoolState (nil base) must report nil store, got %v", got)
	}
}

func TestMempoolState_RefreshLockedSwapsBaseIdentity(t *testing.T) {
	first, second := newFakeCacheStore(), newFakeCacheStore()
	calls := 0
	s := &mempoolState{provider: func() storetypes.CommitMultiStore {
		calls++
		if calls == 1 {
			return &fakeCommitStore{cache: first}
		}
		return &fakeCommitStore{cache: second}
	}}

	s.refreshLocked()
	if got := s.store(); got != storetypes.MultiStore(first) {
		t.Fatalf("expected first base after initial refresh, got %v", got)
	}

	s.refreshLocked()
	if got := s.store(); got != storetypes.MultiStore(second) {
		t.Fatal("refreshLocked must swap base identity on the next call")
	}
}

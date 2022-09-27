package versiondb

import (
	"io"
	"sync"

	"github.com/cosmos/cosmos-sdk/store/cachemulti"
	"github.com/cosmos/cosmos-sdk/store/mem"
	"github.com/cosmos/cosmos-sdk/store/transient"
	"github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.MultiStore = (*MultiStore)(nil)

type MultiStore struct {
	versionDB VersionStore
	storeKeys []types.StoreKey

	// transient or memory stores
	transientStores map[types.StoreKey]types.KVStore

	traceWriter       io.Writer
	traceContext      types.TraceContext
	traceContextMutex sync.Mutex
}

func NewMultiStore(versionDB VersionStore, storeKeys []types.StoreKey) *MultiStore {
	return &MultiStore{versionDB: versionDB, storeKeys: storeKeys, transientStores: make(map[types.StoreKey]types.KVStore)}
}

func (s *MultiStore) GetStoreType() types.StoreType {
	return types.StoreTypeMulti
}

func (s *MultiStore) cacheMultiStore(version *int64) sdk.CacheMultiStore {
	stores := make(map[types.StoreKey]types.CacheWrapper, len(s.transientStores)+len(s.storeKeys))
	for k, v := range s.transientStores {
		stores[k] = v
	}
	for _, k := range s.storeKeys {
		stores[k] = NewKVStore(s.versionDB, k, version)
	}
	return cachemulti.NewStore(nil, stores, nil, s.traceWriter, s.getTracingContext())
}

func (s *MultiStore) CacheMultiStore() sdk.CacheMultiStore {
	return s.cacheMultiStore(nil)
}

func (s *MultiStore) CacheMultiStoreWithVersion(version int64) (sdk.CacheMultiStore, error) {
	return s.cacheMultiStore(&version), nil
}

// CacheWrap implements CacheWrapper/MultiStore/CommitStore.
func (s *MultiStore) CacheWrap() types.CacheWrap {
	return s.CacheMultiStore().(types.CacheWrap)
}

// CacheWrapWithTrace implements the CacheWrapper interface.
func (s *MultiStore) CacheWrapWithTrace(_ io.Writer, _ types.TraceContext) types.CacheWrap {
	return s.CacheWrap()
}

func (s *MultiStore) GetStore(storeKey types.StoreKey) sdk.Store {
	return s.GetKVStore(storeKey)
}

func (s *MultiStore) GetKVStore(storeKey types.StoreKey) sdk.KVStore {
	store, ok := s.transientStores[storeKey]
	if ok {
		return store
	}
	return NewKVStore(s.versionDB, storeKey, nil)
}

func (s *MultiStore) MountTransientStores(keys map[string]*types.TransientStoreKey) {
	for _, key := range keys {
		s.transientStores[key] = transient.NewStore()
	}
}

func (s *MultiStore) MountMemoryStores(keys map[string]*types.MemoryStoreKey) {
	for _, key := range keys {
		s.transientStores[key] = mem.NewStore()
	}
}

// SetTracer sets the tracer for the MultiStore that the underlying
// stores will utilize to trace operations. A MultiStore is returned.
func (s *MultiStore) SetTracer(w io.Writer) types.MultiStore {
	s.traceWriter = w
	return s
}

// SetTracingContext updates the tracing context for the MultiStore by merging
// the given context with the existing context by key. Any existing keys will
// be overwritten. It is implied that the caller should update the context when
// necessary between tracing operations. It returns a modified MultiStore.
func (s *MultiStore) SetTracingContext(tc types.TraceContext) types.MultiStore {
	s.traceContextMutex.Lock()
	defer s.traceContextMutex.Unlock()
	s.traceContext = s.traceContext.Merge(tc)

	return s
}

func (s *MultiStore) getTracingContext() types.TraceContext {
	s.traceContextMutex.Lock()
	defer s.traceContextMutex.Unlock()

	if s.traceContext == nil {
		return nil
	}

	ctx := types.TraceContext{}
	for k, v := range s.traceContext {
		ctx[k] = v
	}

	return ctx
}

// TracingEnabled returns if tracing is enabled for the MultiStore.
func (s *MultiStore) TracingEnabled() bool {
	return s.traceWriter != nil
}

func (s *MultiStore) LatestVersion() int64 {
	version, err := s.versionDB.GetLatestVersion()
	if err != nil {
		panic(err)
	}
	return version
}

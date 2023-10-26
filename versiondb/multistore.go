package versiondb

import (
	"io"
	"sync"

	"github.com/cosmos/cosmos-sdk/store/cachemulti"
	"github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.MultiStore = (*MultiStore)(nil)

// MultiStore wraps `VersionStore` to implement `MultiStore` interface.
type MultiStore struct {
	versionDB VersionStore
	storeKeys []types.StoreKey

	// transient or memory stores
	transientStores map[types.StoreKey]struct{}

	// proxy the calls for transient or mem stores to the parent
	parent types.MultiStore

	traceWriter       io.Writer
	traceContext      types.TraceContext
	traceContextMutex sync.Mutex
}

// NewMultiStore returns a new versiondb `MultiStore`.
func NewMultiStore(parent types.MultiStore, versionDB VersionStore, storeKeys []types.StoreKey) *MultiStore {
	return &MultiStore{versionDB: versionDB, storeKeys: storeKeys, parent: parent, transientStores: make(map[types.StoreKey]struct{})}
}

// GetStoreType implements `MultiStore` interface.
func (s *MultiStore) GetStoreType() types.StoreType {
	return types.StoreTypeMulti
}

// cacheMultiStore branch out the multistore.
func (s *MultiStore) cacheMultiStore(version *int64) sdk.CacheMultiStore {
	stores := make(map[types.StoreKey]types.CacheWrapper, len(s.transientStores)+len(s.storeKeys))
	for k := range s.transientStores {
		stores[k] = s.parent.GetKVStore(k).(types.CacheWrapper)
	}
	for _, k := range s.storeKeys {
		stores[k] = NewKVStore(s.versionDB, k, version)
	}
	return cachemulti.NewStore(nil, stores, nil, s.traceWriter, s.getTracingContext())
}

// CacheMultiStore implements `MultiStore` interface
func (s *MultiStore) CacheMultiStore() sdk.CacheMultiStore {
	return s.cacheMultiStore(nil)
}

// CacheMultiStoreWithVersion implements `MultiStore` interface
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

// GetStore implements `MultiStore` interface
func (s *MultiStore) GetStore(storeKey types.StoreKey) sdk.Store {
	return s.GetKVStore(storeKey)
}

// GetKVStore implements `MultiStore` interface
func (s *MultiStore) GetKVStore(storeKey types.StoreKey) sdk.KVStore {
	if _, ok := s.transientStores[storeKey]; ok {
		return s.parent.GetKVStore(storeKey)
	}
	return NewKVStore(s.versionDB, storeKey, nil)
}

// MountTransientStores simlates the same behavior as sdk to support grpc query service.
func (s *MultiStore) MountTransientStores(keys map[string]*types.TransientStoreKey) {
	for _, key := range keys {
		s.transientStores[key] = struct{}{}
	}
}

// MountMemoryStores simlates the same behavior as sdk to support grpc query service,
// it shares the existing mem store instance.
func (s *MultiStore) MountMemoryStores(keys map[string]*types.MemoryStoreKey) {
	for _, key := range keys {
		s.transientStores[key] = struct{}{}
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

// LatestVersion returns the latest version saved in versiondb
func (s *MultiStore) LatestVersion() int64 {
	version, err := s.versionDB.GetLatestVersion()
	if err != nil {
		panic(err)
	}
	return version
}

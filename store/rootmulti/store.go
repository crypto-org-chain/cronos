package rootmulti

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cosmos/cosmos-sdk/store/listenkv"
	"github.com/crypto-org-chain/cronos/store/memiavlstore"
	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/libs/log"

	pruningtypes "github.com/cosmos/cosmos-sdk/pruning/types"
	snapshottypes "github.com/cosmos/cosmos-sdk/snapshots/types"
	"github.com/cosmos/cosmos-sdk/store/cachemulti"
	"github.com/cosmos/cosmos-sdk/store/mem"
	"github.com/cosmos/cosmos-sdk/store/transient"
	"github.com/cosmos/cosmos-sdk/store/types"
	protoio "github.com/gogo/protobuf/io"
	dbm "github.com/tendermint/tm-db"
)

const CommitInfoFileName = "commit_infos"

var (
	_ types.CommitMultiStore = (*Store)(nil)
)

type Store struct {
	dir    string
	logger log.Logger

	storesParams map[types.StoreKey]storeParams
	keysByName   map[string]types.StoreKey
	stores       map[types.StoreKey]types.CommitKVStore
	listeners    map[types.StoreKey][]types.WriteListener

	initialVersion int64
	lastCommitInfo *types.CommitInfo
}

func NewStore(dir string, logger log.Logger) *Store {
	return &Store{
		dir:    dir,
		logger: logger,

		storesParams: make(map[types.StoreKey]storeParams),
		keysByName:   make(map[string]types.StoreKey),
		stores:       make(map[types.StoreKey]types.CommitKVStore),
		listeners:    make(map[types.StoreKey][]types.WriteListener),
	}
}

// Implements interface Committer
func (rs *Store) Commit() types.CommitID {
	var previousHeight, version int64
	if rs.lastCommitInfo.GetVersion() == 0 && rs.initialVersion > 1 {
		// This case means that no commit has been made in the store, we
		// start from initialVersion.
		version = rs.initialVersion
	} else {
		// This case can means two things:
		// - either there was already a previous commit in the store, in which
		// case we increment the version from there,
		// - or there was no previous commit, and initial version was not set,
		// in which case we start at version 1.
		previousHeight = rs.lastCommitInfo.GetVersion()
		version = previousHeight + 1
	}

	rs.lastCommitInfo = commitStores(version, rs.stores, nil)

	// TODO persist to disk

	return types.CommitID{
		Version: version,
		Hash:    rs.lastCommitInfo.Hash(),
	}
}

// Implements interface Committer
func (rs *Store) LastCommitID() types.CommitID {
	return rs.lastCommitInfo.CommitID()
}

// Implements interface Committer
func (rs *Store) SetPruning(pruningtypes.PruningOptions) {
}

// Implements interface Committer
func (rs *Store) GetPruning() pruningtypes.PruningOptions {
	return pruningtypes.NewPruningOptions(pruningtypes.PruningDefault)
}

// Implements interface Store
func (rs *Store) GetStoreType() types.StoreType {
	return types.StoreTypeMulti
}

// Implements interface CacheWrapper
func (rs *Store) CacheWrap() types.CacheWrap {
	return rs.CacheMultiStore().(types.CacheWrap)
}

// Implements interface CacheWrapper
func (rs *Store) CacheWrapWithTrace(_ io.Writer, _ types.TraceContext) types.CacheWrap {
	return rs.CacheWrap()
}

// Implements interface MultiStore
func (rs *Store) CacheMultiStore() types.CacheMultiStore {
	// TODO custom cache store
	stores := make(map[types.StoreKey]types.CacheWrapper)
	for k, v := range rs.stores {
		store := types.KVStore(v)
		// Wire the listenkv.Store to allow listeners to observe the writes from the cache store,
		// set same listeners on cache store will observe duplicated writes.
		if rs.ListeningEnabled(k) {
			store = listenkv.NewStore(store, k, rs.listeners[k])
		}
		stores[k] = store
	}
	return cachemulti.NewStore(nil, stores, rs.keysByName, nil, nil)
}

// Implements interface MultiStore
// used to createQueryContext, abci_query or grpc query service.
func (rs *Store) CacheMultiStoreWithVersion(version int64) (types.CacheMultiStore, error) {
	panic("rootmulti Store don't support historical query service")
}

// Implements interface MultiStore
func (rs *Store) GetStore(key types.StoreKey) types.Store {
	return rs.GetKVStore(key)
}

// Implements interface MultiStore
func (rs *Store) GetKVStore(key types.StoreKey) types.KVStore {
	s := rs.stores[key]
	if s == nil {
		panic(fmt.Sprintf("store does not exist for key: %s", key.Name()))
	}
	store := types.KVStore(s)

	if rs.ListeningEnabled(key) {
		store = listenkv.NewStore(store, key, rs.listeners[key])
	}

	return store
}

// Implements interface MultiStore
func (rs *Store) TracingEnabled() bool {
	return false
}

// Implements interface MultiStore
func (rs *Store) SetTracer(w io.Writer) types.MultiStore {
	return nil
}

// Implements interface MultiStore
func (rs *Store) SetTracingContext(types.TraceContext) types.MultiStore {
	return nil
}

// Implements interface MultiStore
func (rs *Store) LatestVersion() int64 {
	return rs.lastCommitInfo.Version
}

// Implements interface Snapshotter
func (rs *Store) Snapshot(height uint64, protoWriter protoio.Writer) error {
	// TODO
	return nil
}

// Implements interface Snapshotter
func (rs *Store) Restore(height uint64, format uint32, protoReader protoio.Reader) (snapshottypes.SnapshotItem, error) {
	// TODO
	return snapshottypes.SnapshotItem{}, nil
}

// Implements interface Snapshotter
func (rs *Store) PruneSnapshotHeight(height int64) {
	// TODO
}

// Implements interface Snapshotter
func (rs *Store) SetSnapshotInterval(snapshotInterval uint64) {
	// TODO
}

// Implements interface CommitMultiStore
func (rs *Store) MountStoreWithDB(key types.StoreKey, typ types.StoreType, db dbm.DB) {
	if key == nil {
		panic("MountIAVLStore() key cannot be nil")
	}
	if _, ok := rs.storesParams[key]; ok {
		panic(fmt.Sprintf("store duplicate store key %v", key))
	}
	if _, ok := rs.keysByName[key.Name()]; ok {
		panic(fmt.Sprintf("store duplicate store key name %v", key))
	}
	rs.storesParams[key] = newStoreParams(key, db, typ, 0)
	rs.keysByName[key.Name()] = key

}

// Implements interface CommitMultiStore
func (rs *Store) GetCommitStore(key types.StoreKey) types.CommitStore {
	return rs.GetCommitKVStore(key)
}

// Implements interface CommitMultiStore
func (rs *Store) GetCommitKVStore(key types.StoreKey) types.CommitKVStore {
	return rs.stores[key]
}

// Implements interface CommitMultiStore
// used by normal node startup.
func (rs *Store) LoadLatestVersion() error {
	return rs.LoadLatestVersionAndUpgrade(nil)
}

// Implements interface CommitMultiStore
// used by node startup with UpgradeStoreLoader
func (rs *Store) LoadLatestVersionAndUpgrade(upgrades *types.StoreUpgrades) error {
	cInfo := &types.CommitInfo{}
	bz, err := os.ReadFile(filepath.Join(rs.dir, CommitInfoFileName))
	if err != nil {
		// if file not exists, assume empty db
		if !os.IsNotExist(err) {
			return errors.Wrap(err, "fail to read commit info file")
		}
	} else {
		if err := cInfo.Unmarshal(bz); err != nil {
			return errors.Wrap(err, "failed unmarshal commit info")
		}
	}

	infos := make(map[string]types.StoreInfo)
	// convert StoreInfos slice to map
	for _, storeInfo := range cInfo.StoreInfos {
		infos[storeInfo.Name] = storeInfo
	}

	// load each Store (note this doesn't panic on unmounted keys now)
	newStores := make(map[types.StoreKey]types.CommitKVStore)

	storesKeys := make([]types.StoreKey, 0, len(rs.storesParams))
	for key := range rs.storesParams {
		storesKeys = append(storesKeys, key)
	}
	// deterministic iteration order for upgrades
	sort.Slice(storesKeys, func(i, j int) bool {
		return storesKeys[i].Name() < storesKeys[j].Name()
	})

	var (
		commitID types.CommitID
	)
	for _, key := range storesKeys {
		storeParams := rs.storesParams[key]

		if info, ok := infos[key.Name()]; !ok {
			commitID = info.CommitId
		} else {
			commitID = types.CommitID{}
		}

		// If it has been added, set the initial version
		if upgrades.IsAdded(key.Name()) || upgrades.RenamedFrom(key.Name()) != "" {
			storeParams.initialVersion = uint64(cInfo.Version) + 1
		} else if commitID.Version != cInfo.Version && storeParams.typ == types.StoreTypeIAVL {
			return fmt.Errorf("version of store %s mismatch root store's version; expected %d got %d", key.Name(), cInfo.Version, commitID.Version)
		}

		store, err := rs.loadCommitStoreFromParams(key, commitID, storeParams)
		if err != nil {
			return errors.Wrap(err, "failed to load store")
		}

		newStores[key] = store
		// If it was deleted, remove all data
		if upgrades.IsDeleted(key.Name()) {
			// TODO efficient deletion
		} else if oldName := upgrades.RenamedFrom(key.Name()); oldName != "" {
			// TODO efficient rename
		}
	}

	rs.lastCommitInfo = cInfo
	rs.stores = newStores
	return nil
}

func (rs *Store) loadCommitStoreFromParams(key types.StoreKey, id types.CommitID, params storeParams) (types.CommitKVStore, error) {
	switch params.typ {
	case types.StoreTypeMulti:
		panic("recursive MultiStores not yet supported")
	case types.StoreTypeIAVL:
		dir := filepath.Join(rs.dir, key.Name())
		return memiavlstore.LoadStoreWithInitialVersion(dir, rs.logger, int64(params.initialVersion))
	case types.StoreTypeDB:
		panic("recursive MultiStores not yet supported")
	case types.StoreTypeTransient:
		_, ok := key.(*types.TransientStoreKey)
		if !ok {
			return nil, fmt.Errorf("invalid StoreKey for StoreTypeTransient: %s", key.String())
		}

		return transient.NewStore(), nil

	case types.StoreTypeMemory:
		if _, ok := key.(*types.MemoryStoreKey); !ok {
			return nil, fmt.Errorf("unexpected key type for a MemoryStoreKey; got: %s", key.String())
		}

		return mem.NewStore(), nil

	default:
		panic(fmt.Sprintf("unrecognized store type %v", params.typ))
	}
}

// Implements interface CommitMultiStore
// not used in sdk
func (rs *Store) LoadVersionAndUpgrade(ver int64, upgrades *types.StoreUpgrades) error {
	panic("rootmulti store don't support LoadVersionAndUpgrade")
}

// Implements interface CommitMultiStore
// used by export cmd
func (rs *Store) LoadVersion(ver int64) error {
	if ver != 0 {
		return errors.New("rootmulti store only support load the latest version")
	}
	return rs.LoadLatestVersion()
}

// Implements interface CommitMultiStore
func (rs *Store) SetInterBlockCache(_ types.MultiStorePersistentCache) {
}

// Implements interface CommitMultiStore
// used by InitChain when the initial height is bigger than 1
func (rs *Store) SetInitialVersion(version int64) error {
	rs.initialVersion = version

	// Loop through all the stores, if it's an IAVL store, then set initial
	// version on it.
	for key, store := range rs.stores {
		if store.GetStoreType() == types.StoreTypeIAVL {
			// If the store is wrapped with an inter-block cache, we must first unwrap
			// it to get the underlying IAVL store.
			store = rs.GetCommitKVStore(key)
			store.(types.StoreWithInitialVersion).SetInitialVersion(version)
		}
	}

	return nil
}

// Implements interface CommitMultiStore
func (rs *Store) SetIAVLCacheSize(size int) {
}

// Implements interface CommitMultiStore
func (rs *Store) SetIAVLDisableFastNode(disable bool) {
}

// Implements interface CommitMultiStore
func (rs *Store) SetLazyLoading(lazyLoading bool) {
}

// Implements interface CommitMultiStore
func (rs *Store) RollbackToVersion(version int64) error {
	return errors.New("rootmulti store don't support rollback")
}

// Implements interface CommitMultiStore
func (rs *Store) ListeningEnabled(key types.StoreKey) bool {
	if ls, ok := rs.listeners[key]; ok {
		return len(ls) != 0
	}
	return false
}

// Implements interface CommitMultiStore
func (rs *Store) AddListeners(key types.StoreKey, listeners []types.WriteListener) {
	if ls, ok := rs.listeners[key]; ok {
		rs.listeners[key] = append(ls, listeners...)
	} else {
		rs.listeners[key] = listeners
	}
}

type storeParams struct {
	key            types.StoreKey
	db             dbm.DB
	typ            types.StoreType
	initialVersion uint64
}

func newStoreParams(key types.StoreKey, db dbm.DB, typ types.StoreType, initialVersion uint64) storeParams {
	return storeParams{
		key:            key,
		db:             db,
		typ:            typ,
		initialVersion: initialVersion,
	}
}

// Commits each store and returns a new commitInfo.
func commitStores(version int64, storeMap map[types.StoreKey]types.CommitKVStore, removalMap map[types.StoreKey]bool) *types.CommitInfo {
	storeInfos := make([]types.StoreInfo, 0, len(storeMap))

	for key, store := range storeMap {
		last := store.LastCommitID()

		// If a commit event execution is interrupted, a new iavl store's version will be larger than the rootmulti's metadata, when the block is replayed, we should avoid committing that iavl store again.
		var commitID types.CommitID
		if last.Version >= version {
			last.Version = version
			commitID = last
		} else {
			commitID = store.Commit()
		}
		if store.GetStoreType() == types.StoreTypeTransient {
			continue
		}

		if !removalMap[key] {
			si := types.StoreInfo{}
			si.Name = key.Name()
			si.CommitId = commitID
			storeInfos = append(storeInfos, si)
		}
	}

	sort.SliceStable(storeInfos, func(i, j int) bool {
		return strings.Compare(storeInfos[i].Name, storeInfos[j].Name) < 0
	})

	return &types.CommitInfo{
		Version:    version,
		StoreInfos: storeInfos,
	}
}

package rootmulti

import (
	"fmt"
	"io"
	"sort"

	"github.com/cosmos/cosmos-sdk/store/listenkv"
	"github.com/crypto-org-chain/cronos/memiavl"
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
	db     *memiavl.DB
	logger log.Logger

	storesParams map[types.StoreKey]storeParams
	keysByName   map[string]types.StoreKey
	stores       map[types.StoreKey]types.CommitKVStore
	listeners    map[types.StoreKey][]types.WriteListener

	initialVersion int64
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
	var changeSets []*memiavl.NamedChangeSet
	for storeKey, store := range rs.stores {
		if memiavlStore, ok := store.(*memiavlstore.Store); ok {
			changeSets = append(changeSets, &memiavl.NamedChangeSet{
				Name:      storeKey.Name(),
				Changeset: memiavlStore.PopChangeSet(),
			})
		} else {
			_ = store.Commit()
		}
	}
	sort.SliceStable(changeSets, func(i, j int) bool {
		return changeSets[i].Name < changeSets[j].Name
	})
	hash, v, err := rs.db.Commit(changeSets)
	if err != nil {
		panic(err)
	}

	return types.CommitID{
		Version: v,
		Hash:    hash,
	}
}

// Implements interface Committer
func (rs *Store) LastCommitID() types.CommitID {
	return rs.db.LastCommitInfo().CommitID()
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
	panic("uncached KVStore is not supposed to be accessed directly")
}

// Implements interface MultiStore
func (rs *Store) GetKVStore(key types.StoreKey) types.KVStore {
	panic("uncached KVStore is not supposed to be accessed directly")
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
	return rs.db.Version()
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
	storesKeys := make([]types.StoreKey, 0, len(rs.storesParams))
	for key := range rs.storesParams {
		storesKeys = append(storesKeys, key)
	}
	// deterministic iteration order for upgrades
	sort.Slice(storesKeys, func(i, j int) bool {
		return storesKeys[i].Name() < storesKeys[j].Name()
	})

	initialStores := make([]string, len(storesKeys))
	for i, key := range storesKeys {
		initialStores[i] = key.Name()
	}
	db, err := memiavl.Load(rs.dir, memiavl.Options{
		CreateIfMissing: true,
		InitialStores:   initialStores,
	})
	if err != nil {
		return errors.Wrapf(err, "fail to load memiavl at %s", rs.dir)
	}

	var treeUpgrades []*memiavl.TreeNameUpgrade
	for _, key := range storesKeys {
		switch {
		case upgrades.IsDeleted(key.Name()):
			treeUpgrades = append(treeUpgrades, &memiavl.TreeNameUpgrade{Name: key.Name(), Delete: true})
		case upgrades.IsAdded(key.Name()) || upgrades.RenamedFrom(key.Name()) != "":
			treeUpgrades = append(treeUpgrades, &memiavl.TreeNameUpgrade{Name: key.Name(), RenameFrom: upgrades.RenamedFrom(key.Name())})
		}
	}

	if len(treeUpgrades) > 0 {
		if err := db.ApplyUpgrades(treeUpgrades); err != nil {
			return err
		}
	}

	newStores := make(map[types.StoreKey]types.CommitKVStore, len(storesKeys))
	for _, key := range storesKeys {
		newStores[key], err = rs.loadCommitStoreFromParams(db, key, rs.storesParams[key])
		if err != nil {
			return err
		}
	}

	rs.db = db
	rs.stores = newStores
	return nil
}

func (rs *Store) loadCommitStoreFromParams(db *memiavl.DB, key types.StoreKey, params storeParams) (types.CommitKVStore, error) {
	switch params.typ {
	case types.StoreTypeMulti:
		panic("recursive MultiStores not yet supported")
	case types.StoreTypeIAVL:
		tree := db.TreeByName(key.Name())
		if tree == nil {
			return nil, fmt.Errorf("new store is not added in upgrades")
		}
		return memiavlstore.New(tree, rs.logger), nil
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
	return rs.db.SetInitialVersion(version)
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

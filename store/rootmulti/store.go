package rootmulti

import (
	stderrors "errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"cosmossdk.io/errors"
	protoio "github.com/gogo/protobuf/io"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"

	pruningtypes "github.com/cosmos/cosmos-sdk/pruning/types"
	snapshottypes "github.com/cosmos/cosmos-sdk/snapshots/types"
	"github.com/cosmos/cosmos-sdk/store/cachemulti"
	"github.com/cosmos/cosmos-sdk/store/listenkv"
	"github.com/cosmos/cosmos-sdk/store/mem"
	"github.com/cosmos/cosmos-sdk/store/rootmulti"
	"github.com/cosmos/cosmos-sdk/store/transient"
	"github.com/cosmos/cosmos-sdk/store/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/crypto-org-chain/cronos/memiavl"
	"github.com/crypto-org-chain/cronos/store/memiavlstore"
)

const CommitInfoFileName = "commit_infos"

var (
	_ types.CommitMultiStore = (*Store)(nil)
	_ types.Queryable        = (*Store)(nil)
)

type Store struct {
	dir    string
	db     *memiavl.DB
	logger log.Logger

	// to keep it comptaible with cosmos-sdk 0.46, merge the memstores into commit info
	lastCommitInfo *types.CommitInfo

	storesParams map[types.StoreKey]storeParams
	keysByName   map[string]types.StoreKey
	stores       map[types.StoreKey]types.CommitKVStore
	listeners    map[types.StoreKey][]types.WriteListener

	interBlockCache types.MultiStorePersistentCache

	asyncWAL bool
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
	for key := range rs.stores {
		// it'll unwrap the inter-block cache
		store := rs.GetCommitKVStore(key)
		if memiavlStore, ok := store.(*memiavlstore.Store); ok {
			changeSets = append(changeSets, &memiavl.NamedChangeSet{
				Name:      key.Name(),
				Changeset: memiavlStore.PopChangeSet(),
			})
		} else {
			_ = store.Commit()
		}
	}
	sort.SliceStable(changeSets, func(i, j int) bool {
		return changeSets[i].Name < changeSets[j].Name
	})
	_, _, err := rs.db.Commit(changeSets)
	if err != nil {
		panic(err)
	}
	rs.lastCommitInfo = amendCommitInfo(rs.db.LastCommitInfo(), rs.storesParams)
	return rs.lastCommitInfo.CommitID()
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
//
// Make sure the height is a valid snapshot height
func (rs *Store) Snapshot(height uint64, protoWriter protoio.Writer) error {
	return rs.db.Snapshot(height, protoWriter)
}

// Implements interface Snapshotter
func (rs *Store) Restore(height uint64, format uint32, protoReader protoio.Reader) (snapshottypes.SnapshotItem, error) {
	item, err := memiavl.Import(rs.dir, height, format, protoReader)
	if err != nil {
		return item, err
	}

	return item, rs.LoadLatestVersion()
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
func (rs *Store) MountStoreWithDB(key types.StoreKey, typ types.StoreType, _ dbm.DB) {
	if key == nil {
		panic("MountIAVLStore() key cannot be nil")
	}
	if _, ok := rs.storesParams[key]; ok {
		panic(fmt.Sprintf("store duplicate store key %v", key))
	}
	if _, ok := rs.keysByName[key.Name()]; ok {
		panic(fmt.Sprintf("store duplicate store key name %v", key))
	}
	rs.storesParams[key] = newStoreParams(key, typ)
	rs.keysByName[key.Name()] = key
}

// Implements interface CommitMultiStore
func (rs *Store) GetCommitStore(key types.StoreKey) types.CommitStore {
	return rs.GetCommitKVStore(key)
}

// Implements interface CommitMultiStore
func (rs *Store) GetCommitKVStore(key types.StoreKey) types.CommitKVStore {
	// If the Store has an inter-block cache, first attempt to lookup and unwrap
	// the underlying CommitKVStore by StoreKey. If it does not exist, fallback to
	// the main mapping of CommitKVStores.
	if rs.interBlockCache != nil {
		if store := rs.interBlockCache.Unwrap(key); store != nil {
			return store
		}
	}

	return rs.stores[key]
}

// Implements interface CommitMultiStore
// used by normal node startup.
func (rs *Store) LoadLatestVersion() error {
	return rs.LoadVersionAndUpgrade(0, nil)
}

// Implements interface CommitMultiStore
func (rs *Store) LoadLatestVersionAndUpgrade(upgrades *types.StoreUpgrades) error {
	return rs.LoadVersionAndUpgrade(0, upgrades)
}

// Implements interface CommitMultiStore
// used by node startup with UpgradeStoreLoader
func (rs *Store) LoadVersionAndUpgrade(version int64, upgrades *types.StoreUpgrades) error {
	if version > math.MaxUint32 {
		return fmt.Errorf("version overflows uint32: %d", version)
	}

	storesKeys := make([]types.StoreKey, 0, len(rs.storesParams))
	for key := range rs.storesParams {
		storesKeys = append(storesKeys, key)
	}
	// deterministic iteration order for upgrades
	sort.Slice(storesKeys, func(i, j int) bool {
		return storesKeys[i].Name() < storesKeys[j].Name()
	})

	initialStores := make([]string, 0, len(storesKeys))
	for _, key := range storesKeys {
		if rs.storesParams[key].typ == types.StoreTypeIAVL {
			initialStores = append(initialStores, key.Name())
		}
	}
	db, err := memiavl.Load(rs.dir, memiavl.Options{
		CreateIfMissing: true,
		InitialStores:   initialStores,
		TargetVersion:   uint32(version),
		AsyncWAL:        rs.asyncWAL,
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
	// to keep the root hash compatible with cosmos-sdk 0.46
	if db.Version() != 0 {
		rs.lastCommitInfo = amendCommitInfo(db.LastCommitInfo(), rs.storesParams)
	} else {
		rs.lastCommitInfo = &types.CommitInfo{}
	}
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
		store := types.CommitKVStore(memiavlstore.New(tree, rs.logger))

		if rs.interBlockCache != nil {
			// Wrap and get a CommitKVStore with inter-block caching. Note, this should
			// only wrap the primary CommitKVStore, not any store that is already
			// branched as that will create unexpected behavior.
			store = rs.interBlockCache.GetStoreCache(key, store)
		}

		return store, nil

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
// used by export cmd
func (rs *Store) LoadVersion(ver int64) error {
	return rs.LoadVersionAndUpgrade(ver, nil)
}

// SetInterBlockCache sets the Store's internal inter-block (persistent) cache.
// When this is defined, all CommitKVStores will be wrapped with their respective
// inter-block cache.
func (rs *Store) SetInterBlockCache(c types.MultiStorePersistentCache) {
	rs.interBlockCache = c
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

func (rs *Store) SetAsyncWAL(async bool) {
	rs.asyncWAL = async
}

// Implements interface CommitMultiStore
func (rs *Store) RollbackToVersion(version int64) error {
	return stderrors.New("rootmulti store don't support rollback")
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

// getStoreByName performs a lookup of a StoreKey given a store name typically
// provided in a path. The StoreKey is then used to perform a lookup and return
// a Store. If the Store is wrapped in an inter-block cache, it will be unwrapped
// prior to being returned. If the StoreKey does not exist, nil is returned.
func (rs *Store) GetStoreByName(name string) types.Store {
	key := rs.keysByName[name]
	if key == nil {
		return nil
	}

	return rs.GetCommitKVStore(key)
}

// Implements interface Queryable
func (rs *Store) Query(req abci.RequestQuery) abci.ResponseQuery {
	version := req.Height
	if version == 0 {
		version = rs.db.Version()
	}

	db := rs.db
	if version != rs.lastCommitInfo.Version {
		var err error
		db, err = memiavl.Load(rs.dir, memiavl.Options{TargetVersion: uint32(version)})
		if err != nil {
			return sdkerrors.QueryResult(err, false)
		}
	}

	path := req.Path
	storeName, subpath, err := parsePath(path)
	if err != nil {
		return sdkerrors.QueryResult(err, false)
	}

	store := types.Queryable(memiavlstore.New(db.TreeByName(storeName), rs.logger))

	// trim the path and make the query
	req.Path = subpath
	res := store.Query(req)

	if !req.Prove || !rootmulti.RequireProof(subpath) {
		return res
	}

	if res.ProofOps == nil || len(res.ProofOps.Ops) == 0 {
		return sdkerrors.QueryResult(errors.Wrap(sdkerrors.ErrInvalidRequest, "proof is unexpectedly empty; ensure height has not been pruned"), false)
	}

	// If the request's height is the latest height we've committed, then utilize
	// the store's lastCommitInfo as this commit info may not be flushed to disk.
	// Otherwise, we query for the commit info from disk.
	commitInfo := amendCommitInfo(db.LastCommitInfo(), rs.storesParams)

	// Restore origin path and append proof op.
	res.ProofOps.Ops = append(res.ProofOps.Ops, commitInfo.ProofOp(storeName))

	return res
}

// parsePath expects a format like /<storeName>[/<subpath>]
// Must start with /, subpath may be empty
// Returns error if it doesn't start with /
func parsePath(path string) (storeName string, subpath string, err error) {
	if !strings.HasPrefix(path, "/") {
		return storeName, subpath, errors.Wrapf(sdkerrors.ErrUnknownRequest, "invalid path: %s", path)
	}

	paths := strings.SplitN(path[1:], "/", 2)
	storeName = paths[0]

	if len(paths) == 2 {
		subpath = "/" + paths[1]
	}

	return storeName, subpath, nil
}

type storeParams struct {
	key types.StoreKey
	typ types.StoreType
}

func newStoreParams(key types.StoreKey, typ types.StoreType) storeParams {
	return storeParams{
		key: key,
		typ: typ,
	}
}

func mergeStoreInfos(commitInfo *types.CommitInfo, storeInfos []types.StoreInfo) *types.CommitInfo {
	infos := make([]types.StoreInfo, 0, len(commitInfo.StoreInfos)+len(storeInfos))
	infos = append(infos, commitInfo.StoreInfos...)
	infos = append(infos, storeInfos...)
	sort.SliceStable(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})
	return &types.CommitInfo{
		Version:    commitInfo.Version,
		StoreInfos: infos,
	}
}

// amendCommitInfo add mem stores commit infos to keep it compatible with cosmos-sdk 0.46
func amendCommitInfo(commitInfo *types.CommitInfo, storeParams map[types.StoreKey]storeParams) *types.CommitInfo {
	var extraStoreInfos []types.StoreInfo
	for key := range storeParams {
		typ := storeParams[key].typ
		if typ != types.StoreTypeIAVL && typ != types.StoreTypeTransient {
			extraStoreInfos = append(extraStoreInfos, types.StoreInfo{
				Name:     key.Name(),
				CommitId: types.CommitID{},
			})
		}
	}
	return mergeStoreInfos(commitInfo, extraStoreInfos)
}

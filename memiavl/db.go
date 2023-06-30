package memiavl

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tidwall/wal"
)

const DefaultSnapshotInterval = 1000

// DB implements DB-like functionalities on top of MultiTree:
// - async snapshot rewriting
// - Write-ahead-log
//
// The memiavl.db directory looks like this:
// ```
// > current -> snapshot-N
// > snapshot-N
// >  bank
// >    kvs
// >    nodes
// >    metadata
// >  acc
// >  ... other stores
// > wal
// ```
type DB struct {
	MultiTree
	dir    string
	logger log.Logger

	// result channel of snapshot rewrite goroutine
	snapshotRewriteChan chan snapshotResult
	// the number of old snapshots to keep (excluding the latest one)
	snapshotKeepRecent uint32
	// block interval to take a new snapshot
	snapshotInterval uint32
	// make sure only one snapshot rewrite is running
	pruneSnapshotLock sync.Mutex
	// it's more efficient to export snapshot versions, so we only support that by default
	supportExportNonSnapshotVersion bool
	triggerStateSyncExport          func(height int64)

	// invariant: the LastIndex always match the current version of MultiTree
	wal         *wal.Log
	walChanSize int
	walChan     chan *walEntry
	walQuit     chan error

	// pending store upgrades, will be written into WAL in next Commit call
	pendingUpgrades []*TreeNameUpgrade

	// The assumptions to concurrency:
	// - The methods on DB are protected by a mutex
	// - Each call of Load loads a separate instance, in query scenarios,
	//   it should be immutable, the cache stores will handle the temporary writes.
	// - The DB for the state machine will handle writes through the Commit call,
	//   this method is the sole entry point for tree modifications, and there's no concurrency internally
	//   (the background snapshot rewrite is handled separately), so we don't need locks in the Tree.
	mtx sync.Mutex
}

type Options struct {
	Logger          log.Logger
	CreateIfMissing bool
	InitialVersion  uint32
	// the initial stores when initialize the empty instance
	InitialStores      []string
	SnapshotKeepRecent uint32
	SnapshotInterval   uint32
	// it's more efficient to export snapshot versions, we can filter out the non-snapshot versions
	SupportExportNonSnapshotVersion bool
	TriggerStateSyncExport          func(height int64)
	// load the target version instead of latest version
	TargetVersion uint32
	// Buffer size for the asynchronous commit queue, -1 means synchronous commit,
	// default to 0.
	AsyncCommitBuffer int
	// ZeroCopy if true, the get and iterator methods could return a slice pointing to mmaped blob files.
	ZeroCopy bool
	// CacheSize defines the cache's max entry size for each memiavl store.
	CacheSize int
}

const (
	SnapshotPrefix = "snapshot-"
	SnapshotDirLen = len(SnapshotPrefix) + 20
)

func Load(dir string, opts Options) (*DB, error) {
	snapshotName := "current"
	if opts.TargetVersion > 0 {
		version, err := currentVersion(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to load current version: %w", err)
		}

		if int64(opts.TargetVersion) < version {
			// try to load historical snapshots
			snapshotName, err = seekSnapshot(dir, opts.TargetVersion)
			if err != nil {
				return nil, fmt.Errorf("failed to find snapshot: %w", err)
			}

			if snapshotName == "" {
				return nil, fmt.Errorf("target version is pruned: %d", opts.TargetVersion)
			}
		}
	}

	path := filepath.Join(dir, snapshotName)
	mtree, err := LoadMultiTree(path, opts.ZeroCopy, opts.CacheSize)
	if err != nil {
		if opts.CreateIfMissing && os.IsNotExist(err) {
			if err := initEmptyDB(dir, opts.InitialVersion); err != nil {
				return nil, err
			}
			mtree, err = LoadMultiTree(path, opts.ZeroCopy, opts.CacheSize)
		}
		if err != nil {
			return nil, err
		}
	}

	wal, err := OpenWAL(walPath(dir), &wal.Options{NoCopy: true, NoSync: true})
	if err != nil {
		return nil, err
	}

	if opts.TargetVersion == 0 || int64(opts.TargetVersion) > mtree.Version() {
		if err := mtree.CatchupWAL(wal, int64(opts.TargetVersion)); err != nil {
			return nil, errors.Join(err, wal.Close())
		}
	}

	db := &DB{
		MultiTree:                       *mtree,
		logger:                          opts.Logger,
		dir:                             dir,
		wal:                             wal,
		walChanSize:                     opts.AsyncCommitBuffer,
		snapshotKeepRecent:              opts.SnapshotKeepRecent,
		snapshotInterval:                opts.SnapshotInterval,
		supportExportNonSnapshotVersion: opts.SupportExportNonSnapshotVersion,
		triggerStateSyncExport:          opts.TriggerStateSyncExport,
	}

	if db.logger == nil {
		db.logger = log.NewNopLogger()
	}

	if db.snapshotInterval == 0 {
		db.snapshotInterval = DefaultSnapshotInterval
	}

	if db.Version() == 0 && len(opts.InitialStores) > 0 {
		// do the initial upgrade with the `opts.InitialStores`
		var upgrades []*TreeNameUpgrade
		for _, name := range opts.InitialStores {
			upgrades = append(upgrades, &TreeNameUpgrade{Name: name})
		}
		if err := db.ApplyUpgrades(upgrades); err != nil {
			return nil, errors.Join(err, db.Close())
		}
	}

	return db, nil
}

// SetInitialVersion wraps `MultiTree.SetInitialVersion`.
// it do an immediate snapshot rewrite, because we can't use wal log to record this change,
// because we need it to convert versions to wal index in the first place.
func (db *DB) SetInitialVersion(initialVersion int64) error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	if err := db.MultiTree.SetInitialVersion(initialVersion); err != nil {
		return err
	}

	if err := initEmptyDB(db.dir, db.initialVersion); err != nil {
		return err
	}

	return db.reload()
}

// ApplyUpgrades wraps MultiTree.ApplyUpgrades, it also append the upgrades in a temporary field,
// and include in the WAL entry in next Commit call.
func (db *DB) ApplyUpgrades(upgrades []*TreeNameUpgrade) error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	if err := db.MultiTree.ApplyUpgrades(upgrades); err != nil {
		return err
	}

	db.pendingUpgrades = append(db.pendingUpgrades, upgrades...)
	return nil
}

// checkAsyncTasks checks the status of background tasks non-blocking-ly and process the result
func (db *DB) checkAsyncTasks() error {
	return errors.Join(
		db.checkAsyncCommit(),
		db.checkBackgroundSnapshotRewrite(),
	)
}

// checkAsyncCommit check the quit signal of async wal writing
func (db *DB) checkAsyncCommit() error {
	select {
	case err := <-db.walQuit:
		// async wal writing failed, we need to abort the state machine
		return fmt.Errorf("async wal writing goroutine quit unexpectedly: %w", err)
	default:
	}

	return nil
}

// checkBackgroundSnapshotRewrite check the result of background snapshot rewrite, cleans up the old snapshots and switches to a new multitree
func (db *DB) checkBackgroundSnapshotRewrite() error {
	// check the completeness of background snapshot rewriting
	select {
	case result := <-db.snapshotRewriteChan:
		db.snapshotRewriteChan = nil

		if result.mtree == nil {
			// background snapshot rewrite failed
			return fmt.Errorf("background snapshot rewriting failed: %w", result.err)
		}

		// catchup the remaining wal
		if err := result.mtree.CatchupWAL(db.wal, 0); err != nil {
			return fmt.Errorf("catchup failed: %w", err)
		}

		// do the switch
		if err := db.reloadMultiTree(result.mtree); err != nil {
			return fmt.Errorf("switch multitree failed: %w", err)
		}
		db.logger.Info("switched to new snapshot", "version", db.MultiTree.Version())

		db.pruneSnapshots()

		// trigger state-sync snapshot export
		if db.triggerStateSyncExport != nil {
			db.triggerStateSyncExport(db.SnapshotVersion())
		}
	default:
	}

	return nil
}

// pruneSnapshot prune the old snapshots
func (db *DB) pruneSnapshots() {
	// wait until last prune finish
	db.pruneSnapshotLock.Lock()

	go func() {
		defer db.pruneSnapshotLock.Unlock()

		currentName, err := os.Readlink(currentPath(db.dir))
		if err != nil {
			db.logger.Error("failed to read current snapshot name", "err", err)
			return
		}

		entries, err := os.ReadDir(db.dir)
		if err != nil {
			db.logger.Error("failed to read db dir", "err", err)
			return
		}

		counter := db.snapshotKeepRecent
		for i := len(entries) - 1; i >= 0; i-- {
			name := entries[i].Name()
			if !entries[i].IsDir() || !isSnapshotName(name) {
				continue
			}

			if name >= currentName {
				// ignore any newer snapshot directories, there could be ongoning snapshot rewrite.
				continue
			}

			if counter > 0 {
				counter--
				continue
			}

			db.logger.Info("prune snapshot", "name", name)
			tmpPath := filepath.Join(db.dir, name+"-tmp")
			if err := os.Rename(filepath.Join(db.dir, name), tmpPath); err != nil {
				db.logger.Error("failed to move the snapshot to tmp file", "err", err)
				continue
			}

			if err := os.RemoveAll(tmpPath); err != nil {
				db.logger.Error("failed to prune snapshot", "err", err)
			}
		}

		// truncate WAL until the earliest remaining snapshot
		earliestVersion, err := firstSnapshotVersion(db.dir)
		if err != nil {
			db.logger.Error("failed to find first snapshot", "err", err)
		}

		if err := db.wal.TruncateFront(walIndex(earliestVersion+1, db.initialVersion)); err != nil {
			db.logger.Error("failed to truncate wal", "err", err, "version", earliestVersion+1)
		}
	}()
}

// Commit wraps `MultiTree.ApplyChangeSet` to add some db level operations:
// - manage background snapshot rewriting
// - write WAL
func (db *DB) Commit(changeSets []*NamedChangeSet) ([]byte, int64, error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	if err := db.checkAsyncTasks(); err != nil {
		return nil, 0, err
	}

	hash, v, err := db.MultiTree.ApplyChangeSet(changeSets, true)
	if err != nil {
		return nil, 0, err
	}

	if db.wal != nil {
		// write write-ahead-log
		entry := walEntry{index: walIndex(v, db.initialVersion), data: &WALEntry{
			Changesets: changeSets,
			Upgrades:   db.pendingUpgrades,
		}}
		if db.walChanSize >= 0 {
			if db.walChan == nil {
				db.initAsyncCommit()
			}

			// async wal writing
			db.walChan <- &entry
		} else {
			bz, err := entry.data.Marshal()
			if err != nil {
				return nil, 0, err
			}
			if err := db.wal.Write(entry.index, bz); err != nil {
				return nil, 0, err
			}
		}
	}

	db.pendingUpgrades = db.pendingUpgrades[:0]

	db.rewriteIfApplicable(v)

	return hash, v, nil
}

func (db *DB) initAsyncCommit() {
	walChan := make(chan *walEntry, db.walChanSize)
	walQuit := make(chan error)

	go func() {
		defer close(walQuit)

		for entry := range walChan {
			bz, err := entry.data.Marshal()
			if err != nil {
				walQuit <- err
				return
			}
			if err := db.wal.Write(entry.index, bz); err != nil {
				walQuit <- err
				return
			}
		}
	}()

	db.walChan = walChan
	db.walQuit = walQuit
}

// WaitAsyncCommit waits for the completion of async commit
func (db *DB) WaitAsyncCommit() error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return db.waitAsyncCommit()
}

func (db *DB) waitAsyncCommit() error {
	if db.walChan == nil {
		return nil
	}

	close(db.walChan)
	err := <-db.walQuit

	db.walChan = nil
	db.walQuit = nil
	return err
}

func (db *DB) Copy() *DB {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return db.copy(db.cacheSize)
}

func (db *DB) copy(cacheSize int) *DB {
	mtree := db.MultiTree.Copy(cacheSize)
	return &DB{
		MultiTree: *mtree,
		logger:    db.logger,
		dir:       db.dir,
	}
}

// RewriteSnapshot writes the current version of memiavl into a snapshot, and update the `current` symlink.
func (db *DB) RewriteSnapshot() error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	version := uint32(db.lastCommitInfo.Version)
	snapshotDir := snapshotName(version)
	tmpDir := snapshotDir + "-tmp"
	path := filepath.Join(db.dir, tmpDir)
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return err
	}
	if err := db.MultiTree.WriteSnapshot(path); err != nil {
		return err
	}
	if err := os.Rename(path, filepath.Join(db.dir, snapshotDir)); err != nil {
		return err
	}
	return updateCurrentSymlink(db.dir, snapshotDir)
}

func (db *DB) Reload() error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return db.reload()
}

func (db *DB) reload() error {
	mtree, err := LoadMultiTree(currentPath(db.dir), db.zeroCopy, db.cacheSize)
	if err != nil {
		return err
	}
	return db.reloadMultiTree(mtree)
}

func (db *DB) reloadMultiTree(mtree *MultiTree) error {
	if err := db.MultiTree.Close(); err != nil {
		return err
	}

	db.MultiTree = *mtree

	if len(db.pendingUpgrades) > 0 {
		if err := db.MultiTree.ApplyUpgrades(db.pendingUpgrades); err != nil {
			return err
		}
	}

	return nil
}

// rewriteIfApplicable execute the snapshot rewrite strategy according to current height
func (db *DB) rewriteIfApplicable(height int64) {
	if height%int64(db.snapshotInterval) != 0 {
		return
	}

	if err := db.rewriteSnapshotBackground(); err != nil {
		db.logger.Error("failed to rewrite snapshot in background", "err", err)
	}
}

type snapshotResult struct {
	mtree *MultiTree
	err   error
}

// RewriteSnapshotBackground rewrite snapshot in a background goroutine,
// `Commit` will check the complete status, and switch to the new snapshot.
func (db *DB) RewriteSnapshotBackground() error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return db.rewriteSnapshotBackground()
}

func (db *DB) rewriteSnapshotBackground() error {
	if db.snapshotRewriteChan != nil {
		return errors.New("there's another ongoing snapshot rewriting process")
	}

	ch := make(chan snapshotResult)
	db.snapshotRewriteChan = ch

	cloned := db.copy(0)
	wal := db.wal
	go func() {
		defer close(ch)

		cloned.logger.Info("start rewriting snapshot", "version", cloned.Version())
		if err := cloned.RewriteSnapshot(); err != nil {
			ch <- snapshotResult{err: err}
			return
		}
		cloned.logger.Info("finished rewriting snapshot", "version", cloned.Version())
		mtree, err := LoadMultiTree(currentPath(cloned.dir), cloned.zeroCopy, 0)
		if err != nil {
			ch <- snapshotResult{err: err}
			return
		}

		// do a best effort catch-up, will do another final catch-up in main thread.
		if err := mtree.CatchupWAL(wal, 0); err != nil {
			ch <- snapshotResult{err: err}
			return
		}

		cloned.logger.Info("finished best-effort WAL catchup", "version", cloned.Version(), "latest", mtree.Version())

		ch <- snapshotResult{mtree: mtree}
	}()

	return nil
}

func (db *DB) Close() error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return errors.Join(db.waitAsyncCommit(), db.MultiTree.Close(), db.wal.Close())
}

// TreeByName wraps MultiTree.TreeByName to add a lock.
func (db *DB) TreeByName(name string) *Tree {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return db.MultiTree.TreeByName(name)
}

// Hash wraps MultiTree.Hash to add a lock.
func (db *DB) Hash() []byte {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return db.MultiTree.Hash()
}

// Version wraps MultiTree.Version to add a lock.
func (db *DB) Version() int64 {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return db.MultiTree.Version()
}

// LastCommitInfo returns the last commit info.
func (db *DB) LastCommitInfo() *storetypes.CommitInfo {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return db.MultiTree.LastCommitInfo()
}

// ApplyChangeSet wraps MultiTree.ApplyChangeSet to add a lock.
func (db *DB) ApplyChangeSet(changeSets []*NamedChangeSet, updateCommitInfo bool) ([]byte, int64, error) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return db.MultiTree.ApplyChangeSet(changeSets, updateCommitInfo)
}

// UpdateCommitInfo wraps MultiTree.UpdateCommitInfo to add a lock.
func (db *DB) UpdateCommitInfo() []byte {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return db.MultiTree.UpdateCommitInfo()
}

// WriteSnapshot wraps MultiTree.WriteSnapshot to add a lock.
func (db *DB) WriteSnapshot(dir string) error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return db.MultiTree.WriteSnapshot(dir)
}

func snapshotName(version uint32) string {
	return fmt.Sprintf("%s%020d", SnapshotPrefix, version)
}

func currentPath(root string) string {
	return filepath.Join(root, "current")
}

func snapshotPath(root string, version uint32) string {
	return filepath.Join(root, snapshotName(version))
}

func currentTmpPath(root string) string {
	return filepath.Join(root, "current-tmp")
}

func currentVersion(root string) (int64, error) {
	name, err := os.Readlink(currentPath(root))
	if err != nil {
		return 0, err
	}

	version, err := parseVersion(name)
	if err != nil {
		return 0, err
	}

	return version, nil
}

func parseVersion(name string) (int64, error) {
	if !isSnapshotName(name) {
		return 0, fmt.Errorf("invalid snapshot name %s", name)
	}

	return strconv.ParseInt(name[len(SnapshotPrefix):], 10, 64)
}

// seekSnapshot find the biggest snapshot that's smaller than or equal to target version,
// returns the directory name, if not found, returns empty string.
func seekSnapshot(root string, version uint32) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}

	targetName := snapshotName(version)
	for i := len(entries) - 1; i >= 0; i-- {
		name := entries[i].Name()
		if !entries[i].IsDir() || !isSnapshotName(name) {
			continue
		}

		if name <= targetName {
			return name, nil
		}
	}

	return "", nil
}

// firstSnapshotVersion returns the earliest snapshot name in the db
func firstSnapshotVersion(root string) (int64, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		if !entry.IsDir() || !isSnapshotName(entry.Name()) {
			continue
		}

		version, err := parseVersion(entry.Name())
		if err != nil {
			return 0, err
		}

		return version, nil
	}

	return 0, errors.New("empty memiavl db")
}

func walPath(root string) string {
	return filepath.Join(root, "wal")
}

// init a empty memiavl db
//
// ```
// snapshot-0
//
//	commit_info
//
// current -> snapshot-0
// ```
func initEmptyDB(dir string, initialVersion uint32) error {
	tmp := NewEmptyMultiTree(initialVersion, 0)
	snapshotDir := snapshotName(0)
	if err := tmp.WriteSnapshot(filepath.Join(dir, snapshotDir)); err != nil {
		return err
	}
	return updateCurrentSymlink(dir, snapshotDir)
}

// updateCurrentSymlink creates or replace the current symblic link atomically.
// it could fail under concurrent usage for tmp file conflicts.
func updateCurrentSymlink(dir, snapshot string) error {
	tmpPath := currentTmpPath(dir)
	if err := os.Symlink(snapshot, tmpPath); err != nil {
		return err
	}
	// assuming file renaming operation is atomic
	return os.Rename(tmpPath, currentPath(dir))
}

type walEntry struct {
	index uint64
	data  *WALEntry
}

func isSnapshotName(name string) bool {
	return strings.HasPrefix(name, SnapshotPrefix) && len(name) == SnapshotDirLen
}

// GetLatestVersion finds the latest version number without loading the whole db,
// it's needed for upgrade module to check store upgrades,
// it returns 0 if db don't exists or is empty.
func GetLatestVersion(dir string) (int64, error) {
	metadata, err := readMetadata(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	wal, err := OpenWAL(walPath(dir), &wal.Options{NoCopy: true})
	if err != nil {
		return 0, err
	}
	lastIndex, err := wal.LastIndex()
	if err != nil {
		return 0, err
	}
	return walVersion(lastIndex, uint32(metadata.InitialVersion)), nil
}

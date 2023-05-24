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

const (
	DefaultSnapshotInterval = 1000
	// the minimal in memory states to keep since the latest snapshot, we need to support a few to support ibc relayer,
	// TODO either support loading from multiple snapshots, or make it configurable.
	MinRetainStates = 2
)

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

	snapshotRewriteChan chan snapshotResult
	snapshotKeepRecent  uint32
	snapshotInterval    uint32
	pruneSnapshotLock   sync.Mutex
	pendingForSwitch    *MultiTree

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
	// load the target version instead of latest version
	TargetVersion uint32
	// Buffer size for the asynchronous commit queue, -1 means synchronous commit,
	// default to 0.
	AsyncCommitBuffer int
	// ZeroCopy if true, the get and iterator methods could return a slice pointing to mmaped blob files.
	ZeroCopy bool
}

const (
	SnapshotPrefix = "snapshot-"
)

func Load(dir string, opts Options) (*DB, error) {
	currentDir := currentPath(dir)
	mtree, err := LoadMultiTree(currentDir, opts.ZeroCopy)
	if err != nil {
		if opts.CreateIfMissing && os.IsNotExist(err) {
			if err := initEmptyDB(dir, opts.InitialVersion); err != nil {
				return nil, err
			}
			mtree, err = LoadMultiTree(currentDir, opts.ZeroCopy)
		}
		if err != nil {
			return nil, err
		}
	}

	wal, err := wal.Open(walPath(dir), &wal.Options{NoCopy: true, NoSync: true})
	if err != nil {
		return nil, err
	}

	if err := mtree.CatchupWAL(wal, int64(opts.TargetVersion)); err != nil {
		return nil, err
	}

	db := &DB{
		MultiTree:          *mtree,
		logger:             opts.Logger,
		dir:                dir,
		wal:                wal,
		walChanSize:        opts.AsyncCommitBuffer,
		snapshotKeepRecent: opts.SnapshotKeepRecent,
		snapshotInterval:   opts.SnapshotInterval,
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
			return nil, err
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

		db.pendingForSwitch = result.mtree
	default:
	}

	if db.pendingForSwitch == nil {
		return nil
	}

	// catchup the remaining wal
	if err := db.pendingForSwitch.CatchupWAL(db.wal, 0); err != nil {
		return fmt.Errorf("catchup failed: %w", err)
	}

	// make sure at least `MinRetainStates` queryable states before switch
	if db.pendingForSwitch.Version() < db.pendingForSwitch.SnapshotVersion()+MinRetainStates {
		return nil
	}

	// do the switch
	if err := db.reloadMultiTree(db.pendingForSwitch); err != nil {
		return fmt.Errorf("switch multitree failed: %w", err)
	}
	db.logger.Info("switched to new snapshot", "version", db.MultiTree.Version())

	db.pendingForSwitch = nil
	db.pruneSnapshots(db.SnapshotVersion())
	return nil
}

// pruneSnapshot prune the old snapshots
func (db *DB) pruneSnapshots(newSnapshotVersion int64) {
	// wait until last prune finish
	db.pruneSnapshotLock.Lock()

	go func() {
		defer db.pruneSnapshotLock.Unlock()

		entries, err := os.ReadDir(db.dir)
		if err != nil {
			db.logger.Error("failed to read db dir", "err", err)
			return
		}

		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), SnapshotPrefix) {
				snapshotVersion, err := strconv.ParseInt(strings.TrimPrefix(entry.Name(), SnapshotPrefix), 10, 32)
				if err != nil {
					db.logger.Error("failed when parse snapshot file name", "err", err)
					continue
				}
				if snapshotVersion < newSnapshotVersion-int64(db.snapshotKeepRecent) {
					db.logger.Info("prune snapshot", "name", entry.Name())

					fullPath := filepath.Join(db.dir, entry.Name())
					if err := os.RemoveAll(fullPath); err != nil {
						db.logger.Error("failed when remove old snapshot", "err", err)
					}
				}
			}
		}

		if err := db.wal.TruncateFront(uint64(newSnapshotVersion + 1)); err != nil {
			db.logger.Error("failed to truncate wal", "err", err, "version", newSnapshotVersion+1)
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

	return db.copy()
}

func (db *DB) copy() *DB {
	mtree := db.MultiTree.Copy()
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
	snapshotPath := filepath.Join(db.dir, snapshotDir)
	if err := os.MkdirAll(snapshotPath, os.ModePerm); err != nil {
		return err
	}
	if err := db.MultiTree.WriteSnapshot(snapshotPath); err != nil {
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
	mtree, err := LoadMultiTree(currentPath(db.dir), db.zeroCopy)
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

func (db *DB) RewriteSnapshotBackground() error {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return db.rewriteSnapshotBackground()
}

// rewriteSnapshotBackground rewrite snapshot in a background goroutine,
// `Commit` will check the complete status, and switch to the new snapshot.
func (db *DB) rewriteSnapshotBackground() error {
	if db.snapshotRewriteChan != nil {
		return errors.New("there's another ongoing snapshot rewriting process")
	}

	ch := make(chan snapshotResult)
	db.snapshotRewriteChan = ch

	cloned := db.copy()
	wal := db.wal
	go func() {
		defer close(ch)

		cloned.logger.Info("start rewriting snapshot", "version", cloned.Version())
		if err := cloned.RewriteSnapshot(); err != nil {
			ch <- snapshotResult{err: err}
			return
		}
		cloned.logger.Info("finished rewriting snapshot", "version", cloned.Version())
		mtree, err := LoadMultiTree(currentPath(cloned.dir), db.zeroCopy)
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

	var walErr error
	if db.walChan != nil {
		close(db.walChan)
		walErr = <-db.walQuit

		db.walChan = nil
		db.walQuit = nil
	}

	return errors.Join(db.MultiTree.Close(), db.wal.Close(), walErr)
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
	return fmt.Sprintf("%s%d", SnapshotPrefix, version)
}

func currentPath(root string) string {
	return filepath.Join(root, "current")
}

func currentTmpPath(root string) string {
	return filepath.Join(root, "current-tmp")
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
	tmp := NewEmptyMultiTree(initialVersion)
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

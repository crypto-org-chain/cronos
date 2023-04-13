package memiavl

import (
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/tidwall/wal"
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
	dir string

	snapshotRewriteChan chan snapshotResult
	snapshotKeepRecent  uint32
	pruneSnapshotLock   sync.Mutex

	// invariant: the LastIndex always match the current version of MultiTree
	wal             *wal.Log
	pendingUpgrades []*TreeNameUpgrade
}

type Options struct {
	CreateIfMissing bool
	InitialVersion  uint32
	// the initial stores when initialize the empty instance
	InitialStores      []string
	SnapshotKeepRecent uint32
}

const (
	SnapshotPrefix = "snapshot-"
)

func Load(dir string, opts Options) (*DB, error) {
	currentDir := currentPath(dir)
	mtree, err := LoadMultiTree(currentDir)
	if err != nil {
		if opts.CreateIfMissing && os.IsNotExist(err) {
			if err := initEmptyDB(dir, opts.InitialVersion); err != nil {
				return nil, err
			}
			mtree, err = LoadMultiTree(currentDir)
		}
		if err != nil {
			return nil, err
		}
	}

	wal, err := wal.Open(walPath(dir), &wal.Options{NoCopy: true})
	if err != nil {
		return nil, err
	}

	if err := mtree.CatchupWAL(wal); err != nil {
		return nil, err
	}

	db := &DB{
		MultiTree:          *mtree,
		dir:                dir,
		wal:                wal,
		snapshotKeepRecent: opts.SnapshotKeepRecent,
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
func (db *DB) SetInitialVersion(initialVersion int64) {
	if err := db.MultiTree.SetInitialVersion(initialVersion); err != nil {
		panic(err)
	}

	if err := db.RewriteSnapshot(); err != nil {
		panic(err)
	}

	if err := db.Reload(); err != nil {
		panic(err)
	}
}

// ApplyUpgrades wraps MultiTree.ApplyUpgrades, it also append the upgrades in a temporary field,
// and include in the WAL entry in next Commit call.
func (db *DB) ApplyUpgrades(upgrades []*TreeNameUpgrade) error {
	if err := db.MultiTree.ApplyUpgrades(upgrades); err != nil {
		return err
	}

	db.pendingUpgrades = append(db.pendingUpgrades, upgrades...)
	return nil
}

// cleanupSnapshotRewrite cleans up the old snapshots and switches to a new multitree
// if a snapshot rewrite is in progress. It returns true if a snapshot rewrite has completed
// and false otherwise, along with any error encountered during the cleanup process.
func (db *DB) cleanupSnapshotRewrite() (bool, error) {
	if db.snapshotRewriteChan == nil {
		return false, nil
	}
	// check the completeness of background snapshot rewriting
	select {
	case result := <-db.snapshotRewriteChan:
		db.snapshotRewriteChan = nil

		if result.mtree == nil {
			// background snapshot rewrite failed
			return true, fmt.Errorf("background snapshot rewriting failed: %w", result.err)
		}

		// snapshot rewrite succeeded, catchup and switch
		if err := result.mtree.CatchupWAL(db.wal); err != nil {
			return true, fmt.Errorf("catchup failed: %w", err)
		}
		if err := db.reloadMultiTree(result.mtree); err != nil {
			return true, fmt.Errorf("switch multitree failed: %w", err)
		}
		// prune the old snapshots
		// wait until last prune finish
		db.pruneSnapshotLock.Lock()
		go func() {
			defer db.pruneSnapshotLock.Unlock()

			entries, err := os.ReadDir(db.dir)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() && strings.HasPrefix(entry.Name(), SnapshotPrefix) {
						currentVersion, err := strconv.ParseUint(strings.TrimPrefix(entry.Name(), SnapshotPrefix), 10, 32)
						if err != nil {
							fmt.Printf("failed when parse current version: %s\n", err)
							continue
						}
						if result.version-uint32(currentVersion) > db.snapshotKeepRecent {
							fullPath := filepath.Join(db.dir, entry.Name())
							if err := os.RemoveAll(fullPath); err != nil {
								fmt.Printf("failed when remove old snapshot: %s\n", err)
							}
						}
					}
				}
			}
		}()
		return true, nil

	default:
	}
	return false, nil
}

// Commit wraps `MultiTree.ApplyChangeSet` to add some db level operations:
// - manage background snapshot rewriting
// - write WAL
func (db *DB) Commit(changeSets []*NamedChangeSet) ([]byte, int64, error) {
	if _, err := db.cleanupSnapshotRewrite(); err != nil {
		return nil, 0, err
	}

	hash, v, err := db.ApplyChangeSet(changeSets, true)
	if err != nil {
		return nil, 0, err
	}

	if db.wal != nil {
		// write write-ahead-log
		entry := WALEntry{
			Changesets: changeSets,
			Upgrades:   db.pendingUpgrades,
		}
		bz, err := entry.Marshal()
		if err != nil {
			return nil, 0, err
		}
		if err := db.wal.Write(walIndex(v, db.initialVersion), bz); err != nil {
			return nil, 0, err
		}
	}

	db.pendingUpgrades = db.pendingUpgrades[:0]

	return hash, v, nil
}

func (db *DB) Copy() *DB {
	mtree := db.MultiTree.Copy()
	return &DB{
		MultiTree: *mtree,
		dir:       db.dir,
	}
}

// RewriteSnapshot writes the current version of memiavl into a snapshot, and update the `current` symlink.
func (db *DB) RewriteSnapshot() error {
	version := uint32(db.lastCommitInfo.Version)
	snapshotDir := snapshotPath(db.dir, version)
	if err := os.MkdirAll(snapshotDir, os.ModePerm); err != nil {
		return err
	}
	if err := db.WriteSnapshot(snapshotDir); err != nil {
		return err
	}
	tmpLink := filepath.Join(db.dir, "current-tmp")
	if err := os.Symlink(snapshotDir, tmpLink); err != nil {
		return err
	}
	// assuming file renaming operation is atomic
	return os.Rename(tmpLink, currentPath(db.dir))
}

func (db *DB) Reload() error {
	mtree, err := LoadMultiTree(currentPath(db.dir))
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
	db.pendingUpgrades = nil
	return nil
}

type snapshotResult struct {
	mtree   *MultiTree
	err     error
	version uint32
}

// RewriteSnapshotBackground rewrite snapshot in a background goroutine,
// `Commit` will check the complete status, and switch to the new snapshot.
func (db *DB) RewriteSnapshotBackground() error {
	if db.snapshotRewriteChan != nil {
		return errors.New("there's another ongoing snapshot rewriting process")
	}
	ch := make(chan snapshotResult)
	db.snapshotRewriteChan = ch

	cloned := db.Copy()
	wal := db.wal
	go func() {
		defer close(ch)
		if err := cloned.RewriteSnapshot(); err != nil {
			ch <- snapshotResult{err: err}
			return
		}
		mtree, err := LoadMultiTree(currentPath(db.dir))
		if err != nil {
			ch <- snapshotResult{err: err}
			return
		}
		// do a best effort catch-up first, will try catch-up again in main thread.
		if err := mtree.CatchupWAL(wal); err != nil {
			ch <- snapshotResult{err: err}
			return
		}

		ch <- snapshotResult{mtree: mtree, version: uint32(cloned.lastCommitInfo.Version)}
	}()

	return nil
}

func (db *DB) Close() error {
	return stderrors.Join(db.MultiTree.Close(), db.wal.Close())
}

func snapshotName(version uint32) string {
	return fmt.Sprintf("%s%d", SnapshotPrefix, version)
}

func snapshotPath(root string, version uint32) string {
	return filepath.Join(root, snapshotName(version))
}

func currentPath(root string) string {
	return filepath.Join(root, "current")
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
	snapshotDir := snapshotPath(dir, 0)
	if err := tmp.WriteSnapshot(snapshotDir); err != nil {
		return err
	}
	return os.Symlink(snapshotDir, currentPath(dir))
}

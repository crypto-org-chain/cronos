package memiavl

import (
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/tidwall/wal"
)

// DB implements DB-like functionalities on top of MultiTree:
// - async snapshot rewriting
// - Write-ahead-log
//
// The memiavl.db directory looks like this:
// ```
// current -> snapshot-N
// snapshot-N
//
//	bank
//	  kvs
//	  nodes
//	  metadata
//	acc
//	... other stores
//
// wal
// ```
type DB struct {
	MultiTree
	dir string

	snapshotRewriteChan chan snapshotResult

	// invariant: the LastIndex always match the current version of MultiTree
	wal *wal.Log
}

type Options struct {
	CreateIfMissing bool
	InitialVersion  uint32
	// the initial stores when initialize the empty instance
	InitialStores []string
}

func Load(dir string, opts Options) (*DB, error) {
	currentDir := currentPath(dir)
	mtree, err := LoadMultiTree(currentDir, opts.InitialVersion)
	if err != nil {
		if opts.CreateIfMissing && os.IsNotExist(err) {
			tmp := NewEmptyMultiTree(opts.InitialStores, 0)
			snapshotDir := snapshotPath(dir, 0)
			if err := tmp.WriteSnapshot(snapshotDir); err != nil {
				return nil, err
			}
			if err := os.Symlink(snapshotDir, currentDir); err != nil {
				return nil, err
			}
			mtree, err = LoadMultiTree(currentDir, opts.InitialVersion)
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

	return &DB{
		MultiTree: *mtree,
		dir:       dir,
		wal:       wal,
	}, nil
}

// Commit wraps `MultiTree.ApplyChangeSet` to add some db level operations:
// - manage background snapshot rewriting
// - write WAL
func (db *DB) Commit(changeSets MultiChangeSet) ([]byte, int64, error) {
	if db.snapshotRewriteChan != nil {
		// check the completeness of background snapshot rewriting
		select {
		case result := <-db.snapshotRewriteChan:
			db.snapshotRewriteChan = nil

			if result.mtree == nil {
				// background snapshot rewrite failed
				return nil, 0, fmt.Errorf("background snapshot rewriting failed: %w", result.err)
			}

			// snapshot rewrite succeeded, catchup and switch
			if err := result.mtree.CatchupWAL(db.wal); err != nil {
				return nil, 0, fmt.Errorf("catchup failed: %w", err)
			}
			if err := db.reloadMultiTree(result.mtree); err != nil {
				return nil, 0, fmt.Errorf("switch multitree failed: %w", err)
			}
			// prune the old snapshots
			version := uint32(db.lastCommitInfo.Version)
			latestSnapshot := snapshotName(version)
			prefix := strings.TrimSuffix(latestSnapshot, fmt.Sprintf("%d", version))
			filepath.WalkDir(db.dir, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}

				if entry.IsDir() &&
					strings.HasPrefix(entry.Name(), prefix) &&
					entry.Name() != latestSnapshot {
					os.RemoveAll(path)
				}
				return nil
			})
		default:
		}
	}

	hash, v, err := db.ApplyChangeSet(changeSets, true)
	if err != nil {
		return nil, 0, err
	}

	if db.wal != nil {
		// write write-ahead-log
		bz, err := changeSets.Marshal()
		if err != nil {
			return nil, 0, err
		}
		if err := db.wal.Write(uint64(v), bz); err != nil {
			return nil, 0, err
		}
	}

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
	mtree, err := LoadMultiTree(currentPath(db.dir), db.initialVersion)
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
	return nil
}

type snapshotResult struct {
	mtree *MultiTree
	err   error
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
		mtree, err := LoadMultiTree(currentPath(db.dir), db.initialVersion)
		if err != nil {
			ch <- snapshotResult{err: err}
			return
		}
		// do a best effort catch-up first, will try catch-up again in main thread.
		if err := mtree.CatchupWAL(wal); err != nil {
			ch <- snapshotResult{err: err}
			return
		}

		ch <- snapshotResult{mtree: mtree}
	}()

	return nil
}

func (db *DB) Close() error {
	return stderrors.Join(db.MultiTree.Close(), db.wal.Close())
}

func snapshotName(version uint32) string {
	return fmt.Sprintf("snapshot-%d", version)
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

package memiavl

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// DB implements DB-like functionalities on top of MultiTree:
// - async snapshot rewriting
// - TODO Write-ahead-log
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

	snapshotRewriteChan   chan snapshotResult
	snapshotRewriteBuffer []MultiChangeSet
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

	return &DB{
		MultiTree: *mtree,
		dir:       dir,
	}, nil
}

// Commit wraps `MultiTree.ApplyChangeSet` to add some db level operations:
// - manage background snapshot rewriting
// - write WAL
func (t *DB) Commit(changeSets MultiChangeSet) ([]byte, int64, error) {
	if t.snapshotRewriteChan != nil {
		// check the completeness of background snapshot rewriting
		select {
		case result := <-t.snapshotRewriteChan:
			rewriteBuffer := t.snapshotRewriteBuffer
			t.snapshotRewriteChan = nil
			t.snapshotRewriteBuffer = nil

			if result.mtree == nil {
				// background snapshot rewrite failed
				return nil, 0, fmt.Errorf("background snapshot rewriting failed: %w", result.err)
			}
			// snapshot rewrite succeeded, switch and catchup
			// TODO replay buffer in background to minimize blocking on main thread
			// TODO prune the old snapshots
			t.reloadMultiTree(result.mtree)
			for _, cs := range rewriteBuffer {
				if _, _, err := t.ApplyChangeSet(cs, false); err != nil {
					return nil, 0, fmt.Errorf("snapshot rewrite buffer replay failed: %w", err)
				}
			}
		default:
			t.snapshotRewriteBuffer = append(t.snapshotRewriteBuffer, changeSets)
		}
	}

	return t.ApplyChangeSet(changeSets, true)
}

func (t *DB) Copy() *DB {
	mtree := t.MultiTree.Copy()
	return &DB{
		MultiTree: *mtree,
		dir:       t.dir,
	}
}

// RewriteSnapshot writes the current version of memiavl into a snapshot, and update the `current` symlink.
func (t *DB) RewriteSnapshot() error {
	version := uint32(t.lastCommitInfo.Version)
	snapshotDir := snapshotPath(t.dir, version)
	if err := os.MkdirAll(snapshotDir, os.ModePerm); err != nil {
		return err
	}
	if err := t.WriteSnapshot(snapshotDir); err != nil {
		return err
	}
	tmpLink := filepath.Join(t.dir, "current-tmp")
	if err := os.Symlink(snapshotDir, tmpLink); err != nil {
		return err
	}
	// assuming file renaming operation is atomic
	return os.Rename(tmpLink, currentPath(t.dir))
}

func (t *DB) Reload() error {
	mtree, err := LoadMultiTree(currentPath(t.dir), t.initialVersion)
	if err != nil {
		return err
	}
	return t.reloadMultiTree(mtree)
}

func (t *DB) reloadMultiTree(mtree *MultiTree) error {
	if err := t.MultiTree.Close(); err != nil {
		return err
	}

	t.MultiTree = *mtree
	return nil
}

type snapshotResult struct {
	mtree *MultiTree
	err   error
}

// RewriteSnapshotBackground rewrite snapshot in a background goroutine,
// `Commit` will check the complete status, and switch to the new snapshot.
func (t *DB) RewriteSnapshotBackground() error {
	if t.snapshotRewriteChan != nil {
		return errors.New("there's another ongoing snapshot rewriting process")
	}
	ch := make(chan snapshotResult)
	t.snapshotRewriteChan = ch

	cloned := t.Copy()
	go func() {
		defer close(ch)
		if err := cloned.RewriteSnapshot(); err != nil {
			ch <- snapshotResult{err: err}
			return
		}
		mtree, err := LoadMultiTree(currentPath(t.dir), t.initialVersion)
		if err != nil {
			ch <- snapshotResult{err: err}
			return
		}

		ch <- snapshotResult{mtree: mtree}
	}()

	return nil
}

func snapshotPath(root string, version uint32) string {
	return filepath.Join(root, fmt.Sprintf("snapshot-%d", version))
}

func currentPath(root string) string {
	return filepath.Join(root, "current")
}

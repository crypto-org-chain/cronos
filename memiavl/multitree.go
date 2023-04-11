package memiavl

import (
	"context"
	stderrors "errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/pkg/errors"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/cosmos/iavl"
	"github.com/tidwall/wal"
	"golang.org/x/exp/slices"
	"golang.org/x/sync/errgroup"
)

const CommitInfoFileName = "commit_info"

type namedTree struct {
	tree *Tree
	name string
}

// MultiTree manages multiple memiavl tree together,
// all the trees share the same latest version, the snapshots are always created at the same version.
//
// The snapshot structure is like this:
// ```
// snapshot-V
//
//	bank
//	  kvs
//	  nodes
//	  metadata
//	acc
//	... other stores
//
// ```
type MultiTree struct {
	initialVersion uint32

	trees          []namedTree
	treesByName    map[string]int // reversed index of the trees
	lastCommitInfo storetypes.CommitInfo
}

func NewEmptyMultiTree(initialVersion uint32) *MultiTree {
	return &MultiTree{
		initialVersion: initialVersion,
		treesByName:    make(map[string]int),
	}
}

func LoadMultiTree(dir string, initialVersion uint32) (*MultiTree, error) {
	// load commit info
	bz, err := os.ReadFile(filepath.Join(dir, CommitInfoFileName))
	if err != nil {
		return nil, err
	}
	cInfo := &storetypes.CommitInfo{}
	if err := cInfo.Unmarshal(bz); err != nil {
		return nil, err
	}
	if cInfo.Version > math.MaxUint32 {
		return nil, fmt.Errorf("commit info version overflows uint32: %d", cInfo.Version)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	treeMap := make(map[string]*Tree, len(entries))
	treeNames := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		treeNames = append(treeNames, name)
		snapshot, err := OpenSnapshot(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		treeMap[name] = NewFromSnapshot(snapshot)
	}

	sort.Strings(treeNames)

	trees := make([]namedTree, len(treeNames))
	treesByName := make(map[string]int, len(trees))
	for i, name := range treeNames {
		tree := treeMap[name]
		trees[i] = namedTree{tree: tree, name: name}
		treesByName[name] = i
	}

	return &MultiTree{
		initialVersion: initialVersion,
		trees:          trees,
		treesByName:    treesByName,
		lastCommitInfo: *cInfo,
	}, nil
}

func (t *MultiTree) SetInitialVersion(initialVersion int64) {
	if initialVersion >= math.MaxUint32 {
		panic("version overflows uint32")
	}

	v := uint32(initialVersion)
	t.initialVersion = v
	for _, entry := range t.trees {
		entry.tree.initialVersion = v
	}
}

// Copy returns a snapshot of the tree which won't be corrupted by further modifications on the main tree.
func (t *MultiTree) Copy() *MultiTree {
	trees := make([]namedTree, len(t.trees))
	treesByName := make(map[string]int, len(t.trees))
	for i, entry := range t.trees {
		tree := entry.tree.Copy()
		trees[i] = namedTree{tree: tree, name: entry.name}
		treesByName[entry.name] = i
	}

	clone := *t
	clone.trees = trees
	clone.treesByName = treesByName
	return &clone
}

func (t *MultiTree) Version() int64 {
	return t.lastCommitInfo.Version
}

// ApplyUpgrades store name upgrades
func (t *MultiTree) ApplyUpgrades(upgrades []*TreeNameUpgrade) error {
	if len(upgrades) == 0 {
		return nil
	}

	t.treesByName = nil // rebuild in the end

	for _, upgrade := range upgrades {
		switch {
		case upgrade.Delete:
			i := slices.IndexFunc(t.trees, func(entry namedTree) bool {
				return entry.name == upgrade.Name
			})
			if i < 0 {
				return fmt.Errorf("unknown tree name %s", upgrade.Name)
			}
			// swap deletion
			t.trees[i], t.trees[len(t.trees)-1] = t.trees[len(t.trees)-1], t.trees[i]
			t.trees = t.trees[:len(t.trees)-1]
		case upgrade.RenameFrom != "":
			// rename tree
			i := slices.IndexFunc(t.trees, func(entry namedTree) bool {
				return entry.name == upgrade.RenameFrom
			})
			if i < 0 {
				return fmt.Errorf("unknown tree name %s", upgrade.RenameFrom)
			}
			t.trees[i].name = upgrade.Name
		default:
			// add tree
			tree := NewWithInitialVersion(uint32(t.Version()))
			t.trees = append(t.trees, namedTree{tree: tree, name: upgrade.Name})
		}
	}

	t.treesByName = make(map[string]int, len(t.trees))
	for i, tree := range t.trees {
		t.treesByName[tree.name] = i
	}

	return nil
}

// ApplyChangeSet applies change sets for all trees.
// if `updateCommitInfo` is `false`, the `lastCommitInfo.StoreInfos` is dirty.
func (t *MultiTree) ApplyChangeSet(changeSets []*NamedChangeSet, updateCommitInfo bool) ([]byte, int64, error) {
	var version int64
	if t.lastCommitInfo.Version == 0 && t.initialVersion > 1 {
		version = int64(t.initialVersion)
	} else {
		version = t.lastCommitInfo.Version + 1
	}

	var (
		infos   []storetypes.StoreInfo
		csIndex int
	)
	for _, entry := range t.trees {
		var changeSet iavl.ChangeSet

		if csIndex < len(changeSets) && entry.name == changeSets[csIndex].Name {
			changeSet = changeSets[csIndex].Changeset
			csIndex++
		}
		hash, v, err := entry.tree.ApplyChangeSet(changeSet, updateCommitInfo)
		if err != nil {
			return nil, 0, err
		}
		if updateCommitInfo {
			infos = append(infos, storetypes.StoreInfo{
				Name: entry.name,
				CommitId: storetypes.CommitID{
					Version: v,
					Hash:    hash,
				},
			})
		}
	}

	if csIndex != len(changeSets) {
		return nil, 0, fmt.Errorf("non-exhaustive change sets")
	}

	t.lastCommitInfo.Version = version
	t.lastCommitInfo.StoreInfos = infos

	var hash []byte
	if updateCommitInfo {
		hash = t.lastCommitInfo.Hash()
	}
	return hash, t.lastCommitInfo.Version, nil
}

// UpdateCommitInfo update lastCommitInfo based on current status of trees.
// it's needed if `updateCommitInfo` is set to `false` in `ApplyChangeSet`.
func (t *MultiTree) UpdateCommitInfo() []byte {
	var infos []storetypes.StoreInfo
	for _, entry := range t.trees {
		infos = append(infos, storetypes.StoreInfo{
			Name: entry.name,
			CommitId: storetypes.CommitID{
				Version: entry.tree.Version(),
				Hash:    entry.tree.RootHash(),
			},
		})
	}

	t.lastCommitInfo.StoreInfos = infos
	return t.lastCommitInfo.Hash()
}

// CatchupWAL replay the new entries in the WAL on the tree to catch-up to the latest state.
func (t *MultiTree) CatchupWAL(wal *wal.Log) error {
	walVersion, err := wal.LastIndex()
	if err != nil {
		return errors.Wrap(err, "read wal last index failed")
	}

	snapshotVersion := uint64(t.Version())
	if walVersion <= snapshotVersion {
		// already up-to-date
		return nil
	}

	for v := snapshotVersion + 1; v <= walVersion; v++ {
		bz, err := wal.Read(v)
		if err != nil {
			return errors.Wrap(err, "read wal log failed")
		}
		var entry WALEntry
		if err := entry.Unmarshal(bz); err != nil {
			return errors.Wrap(err, "unmarshal wal log failed")
		}
		if err := t.ApplyUpgrades(entry.Upgrades); err != nil {
			return errors.Wrap(err, "replay store upgrades failed")
		}
		if _, _, err := t.ApplyChangeSet(entry.Changesets, false); err != nil {
			return errors.Wrap(err, "replay change set failed")
		}
	}
	t.UpdateCommitInfo()
	return nil
}

func (t *MultiTree) WriteSnapshot(dir string) error {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	// write the snapshots in parallel
	g, _ := errgroup.WithContext(context.Background())
	for _, entry := range t.trees {
		tree, name := entry.tree, entry.name // https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables
		g.Go(func() error {
			return tree.WriteSnapshot(filepath.Join(dir, name), false)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// write commit info
	bz, err := t.lastCommitInfo.Marshal()
	if err != nil {
		return err
	}
	return writeFileSync(filepath.Join(dir, CommitInfoFileName), bz)
}

func writeFileSync(name string, data []byte) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}
	return err
}

func (t *MultiTree) Close() error {
	errs := make([]error, 0, len(t.trees))
	for _, entry := range t.trees {
		errs = append(errs, entry.tree.Close())
	}
	t.trees = nil
	t.treesByName = nil
	t.lastCommitInfo = storetypes.CommitInfo{}
	return stderrors.Join(errs...)
}

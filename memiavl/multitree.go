package memiavl

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/cosmos/iavl"
)

const CommitInfoFileName = "commit_info"

type treeEntry struct {
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

	trees          []treeEntry
	treesByName    map[string]int
	lastCommitInfo storetypes.CommitInfo
}

func NewEmptyMultiTree(names []string, initialVersion uint32) *MultiTree {
	trees := make([]treeEntry, len(names))
	treesByName := make(map[string]int, len(names))
	infos := make([]storetypes.StoreInfo, len(names))
	for i, name := range names {
		trees[i] = treeEntry{
			tree: NewWithInitialVersion(initialVersion),
			name: name,
		}
		treesByName[name] = i
		infos[i] = storetypes.StoreInfo{
			Name: name,
			CommitId: storetypes.CommitID{
				Hash: trees[i].tree.RootHash(),
			},
		}
	}
	return &MultiTree{
		initialVersion: initialVersion,
		trees:          trees,
		treesByName:    treesByName,
		lastCommitInfo: storetypes.CommitInfo{
			StoreInfos: infos,
		},
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

	trees := make([]treeEntry, len(treeNames))
	treesByName := make(map[string]int, len(treeNames))
	for i, name := range treeNames {
		trees[i] = treeEntry{tree: treeMap[name], name: name}
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
	trees := make([]treeEntry, len(t.trees))
	treesByName := make(map[string]int, len(t.trees))
	for i, entry := range t.trees {
		trees[i] = treeEntry{tree: entry.tree.Copy(), name: entry.name}
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

// ApplyChangeSet applies change sets for all trees.
// if `updateCommitInfo` is `false`, the `lastCommitInfo.StoreInfos` is dirty.
func (t *MultiTree) ApplyChangeSet(changeSets MultiChangeSet, updateCommitInfo bool) ([]byte, int64, error) {
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
		if entry.name == changeSets.Changesets[csIndex].Name {
			changeSet = changeSets.Changesets[csIndex].Changeset
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

	if csIndex != len(changeSets.Changesets) {
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
func (t *MultiTree) UpdateCommitInfo(changeSets MultiChangeSet, updateCommitInfo bool) []byte {
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

func (t *MultiTree) WriteSnapshot(dir string) error {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	// TODO make it parallel
	for _, entry := range t.trees {
		if err := entry.tree.WriteSnapshot(filepath.Join(dir, entry.name), false); err != nil {
			return err
		}
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
	return errors.Join(errs...)
}

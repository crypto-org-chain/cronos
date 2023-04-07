package memiavl

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/cosmos/iavl"
)

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
//   bank
//     kvs
//     nodes
//     metadata
//   acc
//   ... other stores
// ```
type MultiTree struct {
	initialVersion uint32
	version        uint32

	trees          []treeEntry
	treesByName    map[string]int
	lastCommitInfo storetypes.CommitInfo
}

func LoadMultiTree(dir string) (*MultiTree, error) {
	// load commit info
	bz, err := os.ReadFile(filepath.Join(dir, "commit_info"))
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

func (t *MultiTree) ApplyChangeSets(changeSet map[string]iavl.ChangeSet, updateHash bool) ([]storetypes.StoreInfo, int64, error) {
	var infos []storetypes.StoreInfo
	for _, entry := range t.trees {
		hash, v, err := entry.tree.ApplyChangeSet(changeSet[entry.name], updateHash)
		if err != nil {
			return nil, 0, err
		}
		infos = append(infos, storetypes.StoreInfo{
			Name: entry.name,
			CommitId: storetypes.CommitID{
				Version: v,
				Hash:    hash,
			},
		})
	}
	return infos, int64(t.version), nil
}

func (t *MultiTree) WriteSnapshot(dir string) error {
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
	return writeFileSync(filepath.Join(dir, "commit_info"), bz)
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

package memiavl

import (
	"math"

	"github.com/cosmos/iavl"
	"github.com/pkg/errors"
)

// Import a stream of `iavl.ExportNode`s into a new snapshot.
func Import(dir string, version int64, nodes <-chan *iavl.ExportNode, writeHashIndex bool) (returnErr error) {
	if version > int64(math.MaxUint32) {
		return errors.New("version overflows uint32")
	}

	return writeSnapshot(dir, uint32(version), writeHashIndex, func(w *snapshotWriter) (uint32, error) {
		i := &importer{
			snapshotWriter: *w,
		}

		for node := range nodes {
			if err := i.Add(node); err != nil {
				return 0, err
			}
		}

		switch len(i.indexStack) {
		case 0:
			return EmptyRootNodeIndex, nil
		case 1:
			return i.indexStack[0], nil
		default:
			return 0, errors.Errorf("invalid node structure, found stack size %v after imported", len(i.indexStack))
		}
	})
}

type importer struct {
	snapshotWriter

	indexStack []uint32
	nodeStack  []*MemNode
}

func (i *importer) Add(n *iavl.ExportNode) error {
	if n.Version > int64(math.MaxUint32) {
		return errors.New("version overflows uint32")
	}

	if n.Height == 0 {
		node := &MemNode{
			height:  uint8(n.Height),
			size:    1,
			version: uint32(n.Version),
			key:     n.Key,
			value:   n.Value,
		}
		nodeHash := node.Hash()
		idx, err := i.writeLeaf(node.version, node.key, node.value, nodeHash)
		if err != nil {
			return err
		}
		i.indexStack = append(i.indexStack, idx)
		i.nodeStack = append(i.nodeStack, node)
		return nil
	}

	// branch node
	leftIndex := i.indexStack[len(i.indexStack)-2]
	leftNode := i.nodeStack[len(i.nodeStack)-2]
	rightNode := i.nodeStack[len(i.nodeStack)-1]

	node := &MemNode{
		height:  uint8(n.Height),
		size:    leftNode.size + rightNode.size,
		version: uint32(n.Version),
		key:     n.Key,
		left:    leftNode,
		right:   rightNode,
	}
	nodeHash := node.Hash()
	idx, err := i.writeBranch(node.version, uint32(node.size), node.height, leftIndex+1, nodeHash)
	if err != nil {
		return err
	}

	i.indexStack = i.indexStack[:len(i.indexStack)-2]
	i.indexStack = append(i.indexStack, idx)

	i.nodeStack = i.nodeStack[:len(i.nodeStack)-2]
	i.nodeStack = append(i.nodeStack, node)
	return nil
}

package memiavl

import (
	"bytes"
	"crypto/sha256"
	"sort"
)

const (
	OffsetHeight   = 0
	OffsetPreTrees = OffsetHeight + 1
	OffsetVersion  = OffsetHeight + 4
	OffsetSize     = OffsetVersion + 4
	OffsetKeyLeaf  = OffsetSize + 4

	OffsetHash          = OffsetKeyLeaf + 4
	SizeHash            = sha256.Size
	SizeNodeWithoutHash = OffsetHash
	SizeNode            = SizeNodeWithoutHash + SizeHash

	OffsetLeafVersion   = 0
	OffsetLeafKeyLen    = OffsetLeafVersion + 4
	OffsetLeafKeyOffset = OffsetLeafKeyLen + 4
	OffsetLeafHash      = OffsetLeafKeyOffset + 8
	SizeLeafWithoutHash = OffsetLeafHash
	SizeLeaf            = SizeLeafWithoutHash + SizeHash
)

// PersistedNode is backed by serialized byte array, usually mmap-ed from disk file.
// Encoding format (all integers are encoded in little endian):
//
// Branch node:
// - height    : 1
// - _padding  : 3
// - version   : 4
// - size      : 4
// - key node  : 4  // node index of the smallest leaf in right branch
// - hash      : 32
// Leaf node:
// - height     : 1
// - _padding   : 3
// - version    : 4
// - key offset : 8
// - hash       : 32
type PersistedNode struct {
	snapshot *Snapshot
	isLeaf   bool
	index    uint32
}

var _ Node = PersistedNode{}

func (node PersistedNode) branchNode() NodeLayout {
	return node.snapshot.nodesLayout.Node(node.index)
}

func (node PersistedNode) leafNode() LeafLayout {
	return node.snapshot.leavesLayout.Leaf(node.index)
}

func (node PersistedNode) Height() uint8 {
	if node.isLeaf {
		return 0
	}
	return node.branchNode().Height()
}

func (node PersistedNode) Version() uint32 {
	if node.isLeaf {
		return node.leafNode().Version()
	}
	return node.branchNode().Version()
}

func (node PersistedNode) Size() int64 {
	if node.isLeaf {
		return 1
	}
	return int64(node.branchNode().Size())
}

func (node PersistedNode) Key() []byte {
	if node.isLeaf {
		return node.snapshot.LeafKey(node.index)
	}
	index := node.branchNode().KeyLeaf()
	return node.snapshot.LeafKey(index)
}

// Value result is not defined for non-leaf node.
func (node PersistedNode) Value() []byte {
	if !node.isLeaf {
		panic("can't call Value on branch node")
	}
	_, value := node.snapshot.LeafKeyValue(node.index)
	return value
}

// Left result is not defined for leaf nodes.
func (node PersistedNode) Left() Node {
	if node.isLeaf {
		panic("can't call Left on leaf node")
	}

	data := node.branchNode()
	delta := uint32(data.PreTrees())
	startLeaf := node.index + 2 - data.Size() + delta
	keyLeaf := data.KeyLeaf()
	if startLeaf+1 == keyLeaf {
		return PersistedNode{snapshot: node.snapshot, index: startLeaf, isLeaf: true}
	}
	return PersistedNode{snapshot: node.snapshot, index: keyLeaf - delta - 2}
}

// Right result is not defined for leaf nodes.
func (node PersistedNode) Right() Node {
	if node.isLeaf {
		panic("can't call Right on leaf node")
	}

	data := node.branchNode()
	keyLeaf := data.KeyLeaf()
	delta := uint32(data.PreTrees())
	if node.index+delta+1 == keyLeaf {
		return PersistedNode{snapshot: node.snapshot, index: keyLeaf, isLeaf: true}
	}
	return PersistedNode{snapshot: node.snapshot, index: node.index - 1}
}

func (node PersistedNode) Hash() []byte {
	if node.isLeaf {
		return node.leafNode().Hash()
	}
	return node.branchNode().Hash()
}

func (node PersistedNode) Mutate(version, _ uint32) *MemNode {
	if node.isLeaf {
		key, value := node.snapshot.LeafKeyValue(node.index)
		return &MemNode{
			height:  0,
			size:    1,
			version: version,
			key:     key,
			value:   value,
		}
	}
	data := node.branchNode()
	return &MemNode{
		height:  data.Height(),
		size:    int64(data.Size()),
		version: version,
		key:     node.Key(),
		left:    node.Left(),
		right:   node.Right(),
	}
}

func (node PersistedNode) Get(key []byte) ([]byte, uint32) {
	var start, count uint32
	if node.isLeaf {
		start = node.index
		count = 1
	} else {
		data := node.branchNode()
		delta := uint32(data.PreTrees())
		count = data.Size()
		start = node.index + 2 - count + delta
	}

	// binary search in the leaf node array
	i := uint32(sort.Search(int(count), func(i int) bool {
		leafKey := node.snapshot.LeafKey(start + uint32(i))
		return bytes.Compare(leafKey, key) >= 0
	}))
	leaf := i + start
	if leaf >= start+count {
		// return the next index if the key is greater than all keys in the node
		return nil, i
	}
	nodeKey, value := node.snapshot.LeafKeyValue(leaf)
	if !bytes.Equal(nodeKey, key) {
		return nil, i
	}
	return value, i
}

func (node PersistedNode) GetByIndex(leafIndex uint32) ([]byte, []byte) {
	if node.isLeaf {
		if leafIndex != 0 {
			return nil, nil
		}
		return node.snapshot.LeafKeyValue(node.index)
	}
	data := node.branchNode()
	delta := uint32(data.PreTrees())
	startLeaf := node.index + 2 - data.Size() + delta
	endLeaf := node.index + delta + 1

	i := startLeaf + leafIndex
	if i > endLeaf {
		return nil, nil
	}
	return node.snapshot.LeafKeyValue(i)
}

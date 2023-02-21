package memiavl

import (
	"bytes"
	"crypto/sha256"
)

const (
	OffsetHeight    = 0
	OffsetVersion   = OffsetHeight + 4
	OffsetSize      = OffsetVersion + 4
	OffsetKeyNode   = OffsetSize + 4
	OffsetLeafIndex = OffsetSize + 4

	OffsetHash          = OffsetKeyNode + 4
	SizeHash            = sha256.Size
	SizeNodeWithoutHash = OffsetHash
	SizeNode            = SizeNodeWithoutHash + SizeHash
)

// PersistedNode is backed by serialized byte array, usually mmap-ed from disk file.
// Encoding format (all integers are encoded in little endian):
//
// Branch node:
// - height    : uint32
// - version   : uint32
// - size      : uint32
// - key node  : uint32  // node index of the smallest leaf in right branch
// - hash      : [32]byte
// Leaf node:
// - height      : uint32
// - version     : uint32
// - size        : uint32
// - leaf index  : uint32 // can index both key and value
// - hash      : [32]byte
type PersistedNode struct {
	snapshot *Snapshot
	index    uint32
}

var _ Node = PersistedNode{}

func (node PersistedNode) data() *DNode {
	return node.snapshot.nodesData.Node(node.index)
}

func (node PersistedNode) Height() uint8 {
	return node.data().Height()
}

func (node PersistedNode) Version() uint32 {
	return node.data().Version()
}

func (node PersistedNode) Size() int64 {
	return int64(node.data().Size())
}

func (node PersistedNode) Key() []byte {
	var leafIndex uint32
	if node.Height() == 0 {
		leafIndex = node.data().LeafIndex()
	} else {
		leafIndex = node.snapshot.nodesData.Node(node.data().KeyNode()).LeafIndex()
	}
	return node.snapshot.Key(uint64(leafIndex))
}

// Value result is not defined for non-leaf node.
func (node PersistedNode) Value() []byte {
	leafIndex := node.data().LeafIndex()
	return node.snapshot.Value(uint64(leafIndex))
}

// Left result is not defined for leaf nodes.
func (node PersistedNode) Left() Node {
	return PersistedNode{snapshot: node.snapshot, index: node.data().KeyNode() - 1}
}

// Right result is not defined for leaf nodes.
func (node PersistedNode) Right() Node {
	return PersistedNode{snapshot: node.snapshot, index: node.index - 1}
}

func (node PersistedNode) Hash() []byte {
	return node.data().Hash()
}

func (node PersistedNode) Mutate(version uint32) *MemNode {
	mnode := &MemNode{
		height:  node.Height(),
		size:    node.Size(),
		version: version,
		key:     node.Key(),
	}
	if mnode.isLeaf() {
		mnode.value = node.Value()
	} else {
		mnode.left = node.Left()
		mnode.right = node.Right()
	}
	return mnode
}

func (node PersistedNode) Get(key []byte) []byte {
	return getPersistedNode(node.snapshot, node.index, key)
}

func getHeight(data []byte) int8 {
	return int8(data[OffsetHeight])
}

func getPersistedNode(snapshot *Snapshot, index uint32, key []byte) []byte {
	nodes := snapshot.nodesData
	keys := snapshot.keys
	keysOffsets := snapshot.keysOffsets

	for {
		node := nodes.Node(index)
		if node.Height() == 0 {
			leafKey := node.LeafIndex()
			start, end := keysOffsets.Get2(uint64(leafKey))
			nodeKey := keys[start:end]
			if bytes.Equal(key, nodeKey) {
				return snapshot.Value(uint64(leafKey))
			}
			return nil
		}

		keyNode := node.KeyNode()
		start, end := keysOffsets.Get2(uint64(nodes.Node(keyNode).LeafIndex()))
		nodeKey := keys[start:end]
		if bytes.Compare(key, nodeKey) == -1 {
			// left child
			index = keyNode - 1
		} else {
			// right child
			index--
		}
	}
}

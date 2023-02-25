package memiavl

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
)

const (
	OffsetHeight  = 0
	OffsetVersion = OffsetHeight + 4
	OffsetSize    = OffsetVersion + 4
	OffsetKeyNode = OffsetSize + 4

	OffsetKeyLen    = OffsetHeight + 1
	OffsetKeyOffset = OffsetKeyLen + 3
	OffsetLeafIndex = OffsetKeyOffset + 8

	OffsetHash          = OffsetKeyNode + 4
	SizeHash            = sha256.Size
	SizeNodeWithoutHash = OffsetHash
	SizeNode            = SizeNodeWithoutHash + SizeHash
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
// - key len    : 3
// - key offset : 8
// - value index : uint32
// - hash       : 32
type PersistedNode struct {
	snapshot *Snapshot
	index    uint32
}

var _ Node = PersistedNode{}

func (node PersistedNode) data() *NodeLayout {
	return node.snapshot.nodesLayout.Node(node.index)
}

func (node PersistedNode) Height() uint8 {
	return node.data().Height()
}

func (node PersistedNode) Version() uint32 {
	data := node.data()
	if data.Height() != 0 {
		return data.Version()
	}

	offset := data.KeyOffset()
	return binary.LittleEndian.Uint32(node.snapshot.keys[offset-4 : offset])
}

func (node PersistedNode) Size() int64 {
	data := node.data()
	if node.Height() == 0 {
		return 1
	}
	return int64(data.Size())
}

func (node PersistedNode) Key() []byte {
	data := node.data()
	if data.Height() != 0 {
		data = node.snapshot.nodesLayout.Node(data.KeyNode())
	}
	offset, l := data.KeySlice()
	return node.snapshot.keys[offset : offset+uint64(l)]
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
	data := node.data()
	mnode := &MemNode{
		height:  data.Height(),
		size:    int64(data.Size()),
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

// getPersistedNode specialize the get function for `PersistedNode`.
func getPersistedNode(snapshot *Snapshot, index uint32, key []byte) []byte {
	nodes := snapshot.nodesLayout
	keys := snapshot.keys

	for {
		node := nodes.Node(index)
		if node.Height() == 0 {
			offset, l := node.KeySlice()
			nodeKey := keys[offset : offset+uint64(l)]
			if bytes.Equal(key, nodeKey) {
				leafIndex := node.LeafIndex()
				return snapshot.Value(uint64(leafIndex))
			}
			return nil
		}

		keyNode := node.KeyNode()
		offset, l := nodes.Node(keyNode).KeySlice()
		nodeKey := keys[offset : offset+uint64(l)]
		if bytes.Compare(key, nodeKey) == -1 {
			// left child
			index = keyNode - 1
		} else {
			// right child
			index--
		}
	}
}

package memiavl

import (
	"bytes"
	"crypto/sha256"
)

const (
	OffsetHeight  = 0
	OffsetVersion = OffsetHeight + 4
	OffsetSize    = OffsetVersion + 4
	OffsetKeyNode = OffsetSize + 4

	// leaf node repurpose two uint32 to store the offset in kv file.
	OffsetKeyOffset = OffsetVersion + 4

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
// - _padding   : 3
// - version    : 4
// - key offset : 8
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
	return node.data().Version()
}

func (node PersistedNode) Size() int64 {
	data := node.data()
	if data.Height() == 0 {
		return 1
	}
	return int64(data.Size())
}

func (node PersistedNode) Key() []byte {
	data := node.data()
	if data.Height() != 0 {
		data = node.snapshot.nodesLayout.Node(data.KeyNode())
	}
	return node.snapshot.Key(data.KeyOffset())
}

// Value result is not defined for non-leaf node.
func (node PersistedNode) Value() []byte {
	_, value := node.snapshot.KeyValue(node.data().KeyOffset())
	return value
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

	for {
		node := nodes.Node(index)
		if node.Height() == 0 {
			nodeKey, value := snapshot.KeyValue(node.KeyOffset())
			if bytes.Equal(key, nodeKey) {
				return value
			}
			return nil
		}

		keyNode := node.KeyNode()
		nodeKey := snapshot.Key(nodes.Node(keyNode).KeyOffset())
		if bytes.Compare(key, nodeKey) == -1 {
			// left child
			index = keyNode - 1
		} else {
			// right child
			index--
		}
	}
}

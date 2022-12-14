package memiavl

import (
	"encoding/binary"
)

const (
	OffsetHeight  = 0
	OffsetVersion = OffsetHeight + 4
	OffsetSize    = OffsetVersion + 4
	OffsetKey     = OffsetSize + 8
	OffsetRight   = OffsetKey + 8
	OffsetLeft    = OffsetRight + 4
	OffsetValue   = OffsetKey + 8
	OffsetHash    = OffsetValue + 8

	SizeHash            = 32
	SizeNodeWithoutHash = OffsetHash
	SizeNode            = SizeNodeWithoutHash + SizeHash

	// encoding key/value length as 4 bytes with little endianness.
	SizeKeyLen   = 4
	SizeValueLen = 4
)

// PersistedNode is backed by serialized byte array, usually mmap-ed from disk file.
// Encoding format (all integers are encoded in little endian):
// - height  : int8          // padded to 4bytes
// - version : int32
// - size    : int64
// - key     : int64
// - left    : int32         // node index, inner node only
// - right   : int32         // node index, inner node only
// - value   : int64 offset  // leaf node only
// - hash    : [32]byte
type PersistedNode struct {
	snapshot *Snapshot
	offset   uint64
}

var _ Node = PersistedNode{}

func (node PersistedNode) Height() int8 {
	return int8(node.snapshot.nodes[node.offset+OffsetHeight])
}

func (node PersistedNode) Version() int64 {
	return int64(binary.LittleEndian.Uint32(node.snapshot.nodes[node.offset+OffsetVersion:]))
}

func (node PersistedNode) Size() int64 {
	return int64(binary.LittleEndian.Uint64(node.snapshot.nodes[node.offset+OffsetSize:]))
}

func (node PersistedNode) Key() []byte {
	keyOffset := binary.LittleEndian.Uint64(node.snapshot.nodes[node.offset+OffsetKey:])
	keyLen := uint64(binary.LittleEndian.Uint32(node.snapshot.keys[keyOffset:]))
	keyOffset += SizeKeyLen
	return node.snapshot.keys[keyOffset : keyOffset+keyLen]
}

// Value result is not defined for non-leaf node.
func (node PersistedNode) Value() []byte {
	valueOffset := binary.LittleEndian.Uint64(node.snapshot.nodes[node.offset+OffsetValue:])
	valueLen := uint64(binary.LittleEndian.Uint32(node.snapshot.values[valueOffset:]))
	valueOffset += SizeValueLen
	return node.snapshot.values[valueOffset : valueOffset+valueLen]
}

// Left result is not defined for leaf nodes.
func (node PersistedNode) Left() Node {
	nodeIndex := binary.LittleEndian.Uint32(node.snapshot.nodes[node.offset+OffsetLeft:])
	return PersistedNode{snapshot: node.snapshot, offset: uint64(nodeIndex) * SizeNode}
}

// Right result is not defined for leaf nodes.
func (node PersistedNode) Right() Node {
	nodeIndex := binary.LittleEndian.Uint32(node.snapshot.nodes[node.offset+OffsetRight:])
	return PersistedNode{snapshot: node.snapshot, offset: uint64(nodeIndex) * SizeNode}
}

func (node PersistedNode) Hash() []byte {
	offset := node.offset + OffsetHash
	return node.snapshot.nodes[offset : offset+SizeHash]
}

func (node PersistedNode) Mutate(version int64) *MemNode {
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

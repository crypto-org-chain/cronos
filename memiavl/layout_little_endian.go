//go:build !nativebyteorder
// +build !nativebyteorder

package memiavl

import (
	"encoding/binary"
)

// Nodes is a continuously stored IAVL nodes
type Nodes struct {
	data []byte
}

func NewNodes(data []byte) (Nodes, error) {
	return Nodes{data}, nil
}

func (nodes Nodes) Node(i uint32) *DNode {
	return &DNode{data: nodes.data[i*SizeNode:]}
}

type DNode struct {
	data []byte
}

func (node *DNode) Height() uint8 {
	return uint8(node.data[OffsetHeight])
}

func (node *DNode) Version() uint32 {
	return binary.LittleEndian.Uint32(node.data[OffsetVersion : OffsetVersion+4])
}

func (node *DNode) Size() uint32 {
	return binary.LittleEndian.Uint32(node.data[OffsetSize : OffsetSize+4])
}

func (node *DNode) KeyNode() uint32 {
	return binary.LittleEndian.Uint32(node.data[OffsetKeyNode : OffsetKeyNode+4])
}

func (node *DNode) LeafIndex() uint32 {
	return binary.LittleEndian.Uint32(node.data[OffsetLeafIndex : OffsetLeafIndex+4])
}

func (node *DNode) Hash() []byte {
	return node.data[OffsetHash : OffsetHash+SizeHash]
}

type PlainOffsetTable struct {
	data []byte
}

func (t PlainOffsetTable) Get2(i uint64) (uint32, uint32) {
	offset := i * 4
	start := binary.LittleEndian.Uint32(t.data[offset:])
	end := binary.LittleEndian.Uint32(t.data[offset+4:])
	return start, end
}

func NewPlainOffsetTable(data []byte) (PlainOffsetTable, error) {
	return PlainOffsetTable{data}, nil
}

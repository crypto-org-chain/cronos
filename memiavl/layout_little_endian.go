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

func (nodes Nodes) Node(i uint32) *NodeLayout {
	return &NodeLayout{data: nodes.data[i*SizeNode:]}
}

// see comment of `PersistedNode`
type NodeLayout struct {
	data []byte
}

func (node *NodeLayout) Height() uint8 {
	return node.data[OffsetHeight]
}

func (node *NodeLayout) Version() uint32 {
	return binary.LittleEndian.Uint32(node.data[OffsetVersion : OffsetVersion+4])
}

func (node *NodeLayout) Size() uint32 {
	return binary.LittleEndian.Uint32(node.data[OffsetSize : OffsetSize+4])
}

func (node *NodeLayout) KeyNode() uint32 {
	return binary.LittleEndian.Uint32(node.data[OffsetKeyNode : OffsetKeyNode+4])
}

func (node *NodeLayout) LeafIndex() uint32 {
	return binary.LittleEndian.Uint32(node.data[OffsetLeafIndex : OffsetLeafIndex+4])
}

func (node *NodeLayout) Hash() []byte {
	return node.data[OffsetHash : OffsetHash+SizeHash]
}

type PlainOffsetTable struct {
	data []byte
}

func (t PlainOffsetTable) Get2(i uint64) (uint64, uint64) {
	ichunk := i / OffsetRestartInteval
	ii := i % OffsetRestartInteval
	irestart := ichunk * (OffsetRestartInteval + 1) * 4
	data := t.data[irestart:]

	_ = data[3*4-1]
	restart := binary.LittleEndian.Uint64(data[:8])

	if ii == 0 {
		return restart, restart + uint64(binary.LittleEndian.Uint32(data[8:12]))
	}
	if ii == OffsetRestartInteval-1 {
		// the next one is at the beginning of the next chunk
		return restart + uint64(binary.LittleEndian.Uint32(data[OffsetRestartInteval*4:])),
			binary.LittleEndian.Uint64(data[(OffsetRestartInteval+1)*4:])
	}
	// the next one is in the same chunk
	return restart + uint64(binary.LittleEndian.Uint32(data[(ii+1)*4:])),
		restart + uint64(binary.LittleEndian.Uint32(data[(ii+2)*4:]))
}

func NewPlainOffsetTable(data []byte) (PlainOffsetTable, error) {
	return PlainOffsetTable{data}, nil
}

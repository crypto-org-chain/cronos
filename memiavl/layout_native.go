//go:build nativebyteorder
// +build nativebyteorder

package memiavl

import (
	"errors"
	"unsafe"
)

// Nodes is a continuously stored IAVL nodes
type Nodes struct {
	nodes []NodeLayout
}

func NewNodes(buf []byte) (Nodes, error) {
	// check alignment and size of the buffer
	p := unsafe.Pointer(unsafe.SliceData(buf))
	if uintptr(p)%unsafe.Alignof(NodeLayout{}) != 0 {
		return Nodes{}, errors.New("input buffer is not aligned")
	}
	size := int(unsafe.Sizeof(NodeLayout{}))
	if len(buf)%size != 0 {
		return Nodes{}, errors.New("input buffer length is not correct")
	}
	nodes := unsafe.Slice((*NodeLayout)(p), len(buf)/size)
	return Nodes{nodes}, nil
}

func (nodes Nodes) Node(i uint32) *NodeLayout {
	return &nodes.nodes[i]
}

// see comment of `PersistedNode`
type NodeLayout struct {
	data [4]uint32
	hash [32]byte
}

func (node *NodeLayout) Height() uint8 {
	return uint8(node.data[0])
}

func (node *NodeLayout) Version() uint32 {
	return node.data[1]
}

func (node *NodeLayout) Size() uint32 {
	return node.data[2]
}

func (node *NodeLayout) KeyNode() uint32 {
	return node.data[3]
}

func (node *NodeLayout) LeafIndex() uint32 {
	return node.data[3]
}

func (node *NodeLayout) Hash() []byte {
	return node.hash[:]
}

type PlainOffsetTable struct {
	data []uint32
}

func (t PlainOffsetTable) Get2(i uint64) (uint32, uint32) {
	return t.data[i], t.data[i+1]
}

func NewPlainOffsetTable(buf []byte) (PlainOffsetTable, error) {
	// check alignment and size of the buffer
	p := unsafe.Pointer(unsafe.SliceData(buf))
	if uintptr(p)%4 != 0 {
		return PlainOffsetTable{}, errors.New("input buffer is not aligned")
	}
	if len(buf)%4 != 0 {
		return PlainOffsetTable{}, errors.New("input buffer length is not correct")
	}
	data := unsafe.Slice((*uint32)(p), len(buf)/4)
	return PlainOffsetTable{data}, nil
}

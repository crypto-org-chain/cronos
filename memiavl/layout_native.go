//go:build nativebyteorder
// +build nativebyteorder

package memiavl

import (
	"errors"
	"unsafe"
)

// Nodes is a continuously stored IAVL nodes
type Nodes struct {
	nodes []DNode
}

func NewNodes(buf []byte) (Nodes, error) {
	// check alignment and size of the buffer
	p := unsafe.Pointer(unsafe.SliceData(buf))
	if uintptr(p)%unsafe.Alignof(DNode{}) != 0 {
		return Nodes{}, errors.New("input buffer is not aligned")
	}
	size := int(unsafe.Sizeof(DNode{}))
	if len(buf)%size != 0 {
		return Nodes{}, errors.New("input buffer length is not correct")
	}
	nodes := unsafe.Slice((*DNode)(p), len(buf)/size)
	return Nodes{nodes}, nil
}

func (nodes Nodes) Node(i uint32) *DNode {
	return &nodes.nodes[i]
}

// # branch
// - height
// - version
// - size
// - key node
// - hash
//
// # leaf
// - height
// - version
// - size
// - leaf_index   # index both key and value
// - hash
type DNode struct {
	data [4]uint32
	hash [32]byte
}

func (node *DNode) Height() uint8 {
	return uint8(node.data[0])
}

func (node *DNode) Version() uint32 {
	return node.data[1]
}

func (node *DNode) Size() uint32 {
	return node.data[2]
}

func (node *DNode) KeyNode() uint32 {
	return node.data[3]
}

func (node *DNode) LeafIndex() uint32 {
	return node.data[3]
}

func (node *DNode) Hash() []byte {
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

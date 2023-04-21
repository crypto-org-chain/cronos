//go:build nativebyteorder
// +build nativebyteorder

package memiavl

import (
	"errors"
	"unsafe"
)

type NodeLayout = *nodeLayout

// Nodes is a continuously stored IAVL nodes
type Nodes struct {
	nodes []nodeLayout
}

func NewNodes(buf []byte) (Nodes, error) {
	// check alignment and size of the buffer
	p := unsafe.Pointer(unsafe.SliceData(buf))
	if uintptr(p)%unsafe.Alignof(nodeLayout{}) != 0 {
		return Nodes{}, errors.New("input buffer is not aligned")
	}
	size := int(unsafe.Sizeof(nodeLayout{}))
	if len(buf)%size != 0 {
		return Nodes{}, errors.New("input buffer length is not correct")
	}
	nodes := unsafe.Slice((*nodeLayout)(p), len(buf)/size)
	return Nodes{nodes}, nil
}

func (nodes Nodes) Node(i uint32) NodeLayout {
	return &nodes.nodes[i]
}

// see comment of `PersistedNode`
type nodeLayout struct {
	data [4]uint32
	hash [32]byte
}

func (node *nodeLayout) Height() uint8 {
	return uint8(node.data[0])
}

func (node *nodeLayout) Version() uint32 {
	return node.data[1]
}

func (node *nodeLayout) Size() uint32 {
	return node.data[2]
}

func (node *nodeLayout) KeyNode() uint32 {
	return node.data[3]
}

func (node *nodeLayout) KeyOffset() uint64 {
	return uint64(node.data[2]) | uint64(node.data[3])<<32
}

func (node *nodeLayout) Hash() []byte {
	return node.hash[:]
}

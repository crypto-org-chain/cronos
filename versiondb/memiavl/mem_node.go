package memiavl

import (
	"encoding/binary"
	"io"
)

type MemNode struct {
	height  int8
	size    int64
	version int64
	key     []byte
	value   []byte
	left    Node
	right   Node

	hash []byte
}

var _ Node = (*MemNode)(nil)

func newLeafNode(key, value []byte, version int64) *MemNode {
	return &MemNode{
		key: key, value: value, version: version, size: 1,
	}
}

func (node *MemNode) isLeaf() bool {
	return node.height == 0
}

func (node *MemNode) Height() int8 {
	return node.height
}

func (node *MemNode) Size() int64 {
	return node.size
}

func (node *MemNode) Version() int64 {
	return node.version
}

func (node *MemNode) Key() []byte {
	return node.key
}

func (node *MemNode) Value() []byte {
	return node.value
}

func (node *MemNode) Left() Node {
	return node.left
}

func (node *MemNode) Right() Node {
	return node.right
}

// Mutate clears hash and update version field to prepare for further modifications.
func (node *MemNode) Mutate(version int64) *MemNode {
	node.version = version
	node.hash = nil
	return node
}

// Computes the hash of the node without computing its descendants. Must be
// called on nodes which have descendant node hashes already computed.
func (node *MemNode) Hash() []byte {
	if node == nil {
		return nil
	}
	if node.hash != nil {
		return node.hash
	}
	node.hash = HashNode(node)
	return node.hash
}

func (node *MemNode) updateHeightSize() {
	node.height = maxInt8(node.left.Height(), node.right.Height()) + 1
	node.size = node.left.Size() + node.right.Size()
}

func (node *MemNode) calcBalance() int {
	return int(node.left.Height()) - int(node.right.Height())
}

func calcBalance(node Node) int {
	return int(node.Left().Height()) - int(node.Right().Height())
}

// Invariant: node is returned by `Mutate(version)`.
//
//	   S               L
//	  / \      =>     / \
//	 L                   S
//	/ \                 / \
//	  LR               LR
func (node *MemNode) rotateRight(version int64) *MemNode {
	newSelf := node.left.Mutate(version)
	node.left = node.left.Right()
	newSelf.right = node
	node.updateHeightSize()
	newSelf.updateHeightSize()
	return newSelf
}

// Invariant: node is returned by `Mutate(version)`.
//
//	 S              R
//	/ \     =>     / \
//	    R         S
//	   / \       / \
//	 RL             RL
func (node *MemNode) rotateLeft(version int64) *MemNode {
	newSelf := node.right.Mutate(version)
	node.right = node.right.Left()
	newSelf.left = node
	node.updateHeightSize()
	newSelf.updateHeightSize()
	return newSelf
}

// Invariant: node is returned by `Mutate(version)`.
func (node *MemNode) reBalance(version int64) *MemNode {
	balance := node.calcBalance()
	switch {
	case balance > 1:
		leftBalance := calcBalance(node.left)
		if leftBalance >= 0 {
			// left left
			return node.rotateRight(version)
		}
		// left right
		node.left = node.left.Mutate(version).rotateLeft(version)
		return node.rotateRight(version)
	case balance < -1:
		rightBalance := calcBalance(node.right)
		if rightBalance <= 0 {
			// right right
			return node.rotateLeft(version)
		}
		// right left
		node.right = node.right.Mutate(version).rotateRight(version)
		return node.rotateLeft(version)
	default:
		// nothing changed
		return node
	}
}

// EncodeBytes writes a varint length-prefixed byte slice to the writer.
func EncodeBytes(w io.Writer, bz []byte) error {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], uint64(len(bz)))
	if _, err := w.Write(buf[0:n]); err != nil {
		return err
	}
	_, err := w.Write(bz)
	return err
}

func maxInt8(a, b int8) int8 {
	if a > b {
		return a
	}
	return b
}

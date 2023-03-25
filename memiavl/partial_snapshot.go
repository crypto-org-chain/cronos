package memiavl

// import (
// 	"encoding/binary"
// 	"fmt"
// 	"io/ioutil"
// 	"os"
// 	"path/filepath"
// )

// type PartialSnapshot struct {
// 	rootHash   []byte
// 	sinceHash  []byte
// 	dir        string
// 	nodeWriter *os.File
// }

// func OpenPartialSnapshot(snapshotDir, rootHash, sinceHash string) (*PartialSnapshot, error) {
// 	dir := filepath.Join(snapshotDir, fmt.Sprintf("partial-%s-%s", rootHash, sinceHash))
// 	if err := os.MkdirAll(dir, 0755); err != nil {
// 		return nil, err
// 	}

// 	nodeFile := filepath.Join(dir, "nodes")
// 	nodeWriter, err := os.OpenFile(nodeFile, os.O_CREATE|os.O_WRONLY, 0644)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &PartialSnapshot{
// 		rootHash:   []byte(rootHash),
// 		sinceHash:  []byte(sinceHash),
// 		dir:        dir,
// 		nodeWriter: nodeWriter,
// 	}, nil
// }

// func (ps *PartialSnapshot) WriteSnapshot(tree *ImmutableTree) error {
// 	// Traverse the tree from the root, comparing each node with the previous snapshot
// 	var traverse func(n *Node) error
// 	traverse = func(n *Node) error {
// 		if n == nil {
// 			return nil
// 		}

// 		// If the current node exists in the previous snapshot, skip it
// 		if tree2, err := ReadSnapshot(ps.sinceHash); err == nil {
// 			if node, _ := tree2.GetNode(n.hash); node != nil {
// 				return nil
// 			}
// 		}

// 		// Write node's hash to the partial snapshot file
// 		if _, err := ps.nodeWriter.Write(n.hash); err != nil {
// 			return err
// 		}

// 		// Write node's data length
// 		dataLen := uint32(len(n.data))
// 		if err := binary.Write(ps.nodeWriter, binary.BigEndian, &dataLen); err != nil {
// 			return err
// 		}

// 		// Write node's data
// 		if _, err := ps.nodeWriter.Write(n.data); err != nil {
// 			return err
// 		}

// 		// Traverse left and right children
// 		if err := traverse(n.leftNode); err != nil {
// 			return err
// 		}
// 		if err := traverse(n.rightNode); err != nil {
// 			return err
// 		}

// 		return nil
// 	}

// 	if err := traverse(tree.root); err != nil {
// 		return err
// 	}

// 	// Close the partial snapshot file
// 	if err := ps.nodeWriter.Close(); err != nil {
// 		return err
// 	}

// 	// Write the metadata file
// 	metadata := fmt.Sprintf("%s\n%s", ps.rootHash, ps.sinceHash)
// 	if err := ioutil.WriteFile(filepath.Join(ps.dir, "metadata"), []byte(metadata), 0644); err != nil {
// 		return err
// 	}

// 	return nil
// }

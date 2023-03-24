package memiavl

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/cosmos/iavl"
	"github.com/pkg/errors"
)

// Import a stream of `iavl.ExportNode`s into a new snapshot.
func Import(dir string, version int64, nodes chan *iavl.ExportNode, writeHashIndex bool) (returnErr error) {
	if version > int64(math.MaxUint32) {
		return errors.New("version overflows uint32")
	}

	nodesFile := filepath.Join(dir, FileNameNodes)
	kvsFile := filepath.Join(dir, FileNameKVs)
	kvsIndexFile := filepath.Join(dir, FileNameKVIndex)

	fpNodes, err := createFile(nodesFile)
	if err != nil {
		return err
	}
	defer func() {
		if err := fpNodes.Close(); returnErr == nil {
			returnErr = err
		}
	}()

	fpKVs, err := createFile(kvsFile)
	if err != nil {
		return err
	}
	defer func() {
		if err := fpKVs.Close(); returnErr == nil {
			returnErr = err
		}
	}()

	nodesWriter := bufio.NewWriter(fpNodes)
	kvsWriter := bufio.NewWriter(fpKVs)

	i := &importer{
		snapshotWriter: *newSnapshotWriter(nodesWriter, kvsWriter),
	}

	var counter int
	for node := range nodes {
		if err := i.Add(node); err != nil {
			return err
		}
		counter++
	}

	if len(i.nodeStack) > 1 {
		return errors.Errorf("invalid node structure, found stack size %v after imported", len(i.nodeStack))
	}

	var rootIndex uint32
	if len(i.nodeStack) == 0 {
		rootIndex = EmptyRootNodeIndex
	} else {
		rootIndex = uint32(counter - 1)

		if err := nodesWriter.Flush(); err != nil {
			return err
		}
		if err := kvsWriter.Flush(); err != nil {
			return err
		}

		if err := fpKVs.Sync(); err != nil {
			return err
		}
		if err := fpNodes.Sync(); err != nil {
			return err
		}

		if writeHashIndex {
			// re-open kvs file for reading
			input, err := os.Open(kvsFile)
			if err != nil {
				return err
			}
			defer func() {
				if err := input.Close(); returnErr == nil {
					returnErr = err
				}
			}()
			// N = 2L-1
			leaves := (counter + 1) / 2
			if err := buildIndex(input, kvsIndexFile, dir, leaves); err != nil {
				return fmt.Errorf("build MPHF index failed: %w", err)
			}
		}
	}

	// write metadata
	var metadataBuf [SizeMetadata]byte
	binary.LittleEndian.PutUint32(metadataBuf[:], SnapshotFileMagic)
	binary.LittleEndian.PutUint32(metadataBuf[4:], SnapshotFormat)
	binary.LittleEndian.PutUint32(metadataBuf[8:], uint32(version))
	binary.LittleEndian.PutUint32(metadataBuf[12:], rootIndex)

	metadataFile := filepath.Join(dir, FileNameMetadata)
	fpMetadata, err := createFile(metadataFile)
	if err != nil {
		return err
	}
	defer func() {
		if err := fpMetadata.Close(); returnErr == nil {
			returnErr = err
		}
	}()

	if _, err := fpMetadata.Write(metadataBuf[:]); err != nil {
		return err
	}

	return fpMetadata.Sync()
}

type importer struct {
	snapshotWriter

	indexStack []uint32
	nodeStack  []*MemNode
}

func (i *importer) Add(n *iavl.ExportNode) error {
	if n.Version > int64(math.MaxUint32) {
		return errors.New("version overflows uint32")
	}

	if n.Height == 0 {
		node := &MemNode{
			height:  uint8(n.Height),
			size:    1,
			version: uint32(n.Version),
			key:     n.Key,
			value:   n.Value,
		}
		nodeHash := node.Hash()
		idx, err := i.writeLeaf(node.version, node.key, node.value, nodeHash)
		if err != nil {
			return err
		}
		i.indexStack = append(i.indexStack, idx)
		i.nodeStack = append(i.nodeStack, node)
		return nil
	}

	// branch node
	leftIndex := i.indexStack[len(i.indexStack)-2]
	leftNode := i.nodeStack[len(i.nodeStack)-2]
	rightNode := i.nodeStack[len(i.nodeStack)-1]

	node := &MemNode{
		height:  uint8(n.Height),
		size:    leftNode.size + rightNode.size,
		version: uint32(n.Version),
		key:     n.Key,
		left:    leftNode,
		right:   rightNode,
	}
	nodeHash := node.Hash()
	idx, err := i.writeBranch(node.version, uint32(node.size), node.height, leftIndex+1, nodeHash)
	if err != nil {
		return err
	}

	i.indexStack = i.indexStack[:len(i.indexStack)-2]
	i.indexStack = append(i.indexStack, idx)

	i.nodeStack = i.nodeStack[:len(i.nodeStack)-2]
	i.nodeStack = append(i.nodeStack, node)
	return nil
}

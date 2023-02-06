package memiavl

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-multierror"
	"github.com/ledgerwatch/erigon-lib/mmap"
)

const (
	// SnapshotFileMagic is little endian encoded b"IAVL"
	SnapshotFileMagic = 1280721225

	// the initial snapshot format
	SnapshotFormat = 0

	// magic: uint32, format: uint32, version: uint64, root node index: uint32
	SizeMetadata = 20

	// EmptyRootNodeIndex is a special value of root node index to represent empty tree
	EmptyRootNodeIndex = math.MaxUint32
)

// Snapshot manage the lifecycle of mmap-ed files for the snapshot,
// it must out live the objects that derived from it.
type Snapshot struct {
	nodesMap  *MmapFile
	keysMap   *MmapFile
	valuesMap *MmapFile

	nodes  []byte
	keys   []byte
	values []byte

	// parsed from metadata file
	version   uint64
	rootIndex uint32
}

func NewEmptySnapshot(version uint64) *Snapshot {
	return &Snapshot{
		version:   version,
		rootIndex: EmptyRootNodeIndex,
	}
}

// OpenSnapshot parse the version number and the root node index from metadata file,
// and mmap the other files.
func OpenSnapshot(snapshotDir string) (*Snapshot, error) {
	// read metadata file
	bz, err := os.ReadFile(filepath.Join(snapshotDir, "metadata"))
	if err != nil {
		return nil, err
	}
	if len(bz) != SizeMetadata {
		return nil, fmt.Errorf("wrong metadata file size, expcted: %d, found: %d", SizeMetadata, len(bz))
	}

	magic := binary.LittleEndian.Uint32(bz)
	if magic != SnapshotFileMagic {
		return nil, fmt.Errorf("invalid metadata file magic: %d", magic)
	}
	format := binary.LittleEndian.Uint32(bz[4:])
	if format != SnapshotFormat {
		return nil, fmt.Errorf("unknown snapshot format: %d", format)
	}
	version := binary.LittleEndian.Uint64(bz[8:])
	rootIndex := binary.LittleEndian.Uint32(bz[16:])

	if rootIndex == EmptyRootNodeIndex {
		// we can't mmap empty files, so have to return early
		return NewEmptySnapshot(version), nil
	}

	var nodesMap, keysMap, valuesMap *MmapFile
	cleanupHandles := func(err error) error {
		if nodesMap != nil {
			if merr := nodesMap.Close(); merr != nil {
				err = multierror.Append(merr, err)
			}
		}
		if keysMap != nil {
			if merr := keysMap.Close(); merr != nil {
				err = multierror.Append(merr, err)
			}
		}
		if valuesMap != nil {
			if merr := valuesMap.Close(); merr != nil {
				err = multierror.Append(merr, err)
			}
		}
		return err
	}

	if nodesMap, err = NewMmap(filepath.Join(snapshotDir, "nodes")); err != nil {
		return nil, cleanupHandles(err)
	}
	if keysMap, err = NewMmap(filepath.Join(snapshotDir, "keys")); err != nil {
		return nil, cleanupHandles(err)
	}
	if valuesMap, err = NewMmap(filepath.Join(snapshotDir, "values")); err != nil {
		return nil, cleanupHandles(err)
	}

	nodes := nodesMap.Data()

	if len(nodes) == 0 && rootIndex != 0 {
		return nil, cleanupHandles(
			fmt.Errorf("corrupted snapshot, nodes are empty but rootIndex is not zero: %d", rootIndex),
		)
	}

	if len(nodes) > 0 && uint64(len(nodes)) != (uint64(rootIndex)+1)*SizeNode {
		return nil, cleanupHandles(
			fmt.Errorf("nodes file size %d don't match root node index %d", len(nodes), rootIndex),
		)
	}

	return &Snapshot{
		nodesMap:  nodesMap,
		keysMap:   keysMap,
		valuesMap: valuesMap,

		// cache the pointers
		nodes:  nodesMap.Data(),
		keys:   keysMap.Data(),
		values: valuesMap.Data(),

		version:   version,
		rootIndex: rootIndex,
	}, nil
}

// Close closes the file and mmap handles, clears the buffers.
func (snapshot *Snapshot) Close() error {
	var err error

	if snapshot.nodesMap != nil {
		if merr := snapshot.nodesMap.Close(); merr != nil {
			err = multierror.Append(err, merr)
		}
	}
	if snapshot.keysMap != nil {
		if merr := snapshot.keysMap.Close(); merr != nil {
			err = multierror.Append(err, merr)
		}
	}
	if snapshot.valuesMap != nil {
		if merr := snapshot.valuesMap.Close(); merr != nil {
			err = multierror.Append(err, merr)
		}
	}

	// reset to an empty tree
	*snapshot = *NewEmptySnapshot(snapshot.version)
	return err
}

// IsEmpty returns if the snapshot is an empty tree.
func (snapshot *Snapshot) IsEmpty() bool {
	return snapshot.rootIndex == EmptyRootNodeIndex
}

// Node returns the node by index
func (snapshot *Snapshot) Node(index uint32) PersistedNode {
	return PersistedNode{
		snapshot: snapshot,
		offset:   uint64(index) * SizeNode,
	}
}

// Version returns the version of the snapshot
func (snapshot *Snapshot) Version() uint64 {
	return snapshot.version
}

// RootNode returns the root node
func (snapshot *Snapshot) RootNode() *PersistedNode {
	if len(snapshot.nodes) == 0 {
		// root node of empty tree is represented as `nil`
		return nil
	}
	node := snapshot.Node(snapshot.rootIndex)
	return &node
}

// nodesLen returns the number of nodes in the snapshot
func (snapshot *Snapshot) nodesLen() int {
	return len(snapshot.nodes) / SizeNode
}

// ScanNodes iterate over the nodes in the snapshot order (depth-first post-order)
func (snapshot *Snapshot) ScanNodes(callback func(node PersistedNode) error) error {
	for i := 0; i < snapshot.nodesLen(); i++ {
		if err := callback(snapshot.Node(uint32(i))); err != nil {
			return err
		}
	}
	return nil
}

// WriteSnapshot save the IAVL tree to a new snapshot directory.
func (t *Tree) WriteSnapshot(snapshotDir string) error {
	var rootIndex uint32
	if t.root == nil {
		rootIndex = EmptyRootNodeIndex
	} else {
		nodesFile := filepath.Join(snapshotDir, "nodes")
		keysFile := filepath.Join(snapshotDir, "keys")
		valuesFile := filepath.Join(snapshotDir, "values")

		fpNodes, err := createFile(nodesFile)
		if err != nil {
			return err
		}
		defer fpNodes.Close()

		fpKeys, err := createFile(keysFile)
		if err != nil {
			return err
		}
		defer fpKeys.Close()

		fpValues, err := createFile(valuesFile)
		if err != nil {
			return err
		}
		defer fpValues.Close()

		nodesWriter := bufio.NewWriter(fpNodes)
		keysWriter := bufio.NewWriter(fpKeys)
		valuesWriter := bufio.NewWriter(fpValues)

		w := newSnapshotWriter(nodesWriter, keysWriter, valuesWriter)
		rootIndex, _, err = w.writeRecursive(t.root)
		if err != nil {
			return err
		}

		if err := nodesWriter.Flush(); err != nil {
			return err
		}
		if err := keysWriter.Flush(); err != nil {
			return err
		}
		if err := valuesWriter.Flush(); err != nil {
			return err
		}

		if err := fpKeys.Sync(); err != nil {
			return err
		}
		if err := fpValues.Sync(); err != nil {
			return err
		}
		if err := fpNodes.Sync(); err != nil {
			return err
		}
	}

	// write metadata
	var metadataBuf [SizeMetadata]byte
	binary.LittleEndian.PutUint32(metadataBuf[:], SnapshotFileMagic)
	binary.LittleEndian.PutUint32(metadataBuf[4:], SnapshotFormat)
	binary.LittleEndian.PutUint64(metadataBuf[8:], uint64(t.version))
	binary.LittleEndian.PutUint32(metadataBuf[16:], rootIndex)

	metadataFile := filepath.Join(snapshotDir, "metadata")
	fpMetadata, err := createFile(metadataFile)
	if err != nil {
		return err
	}
	defer fpMetadata.Close()

	if _, err := fpMetadata.Write(metadataBuf[:]); err != nil {
		return err
	}

	return fpMetadata.Sync()
}

type snapshotWriter struct {
	nodesWriter, keysWriter, valuesWriter io.Writer
	nodeIndex                             uint32
	keysOffset, valuesOffset              uint64
}

func newSnapshotWriter(nodesWriter, keysWriter, valuesWriter io.Writer) *snapshotWriter {
	return &snapshotWriter{
		nodesWriter:  nodesWriter,
		keysWriter:   keysWriter,
		valuesWriter: valuesWriter,
	}
}

// writeKey append key to keys file
func (w *snapshotWriter) writeKey(key []byte) (uint64, error) {
	var buf [SizeKeyLen]byte
	if len(key) > math.MaxUint32 {
		return 0, fmt.Errorf("key length overflow: %d", len(key))
	}
	binary.LittleEndian.PutUint32(buf[:], uint32(len(key)))
	if _, err := w.keysWriter.Write(buf[:]); err != nil {
		return 0, err
	}
	if _, err := w.keysWriter.Write(key); err != nil {
		return 0, err
	}
	offset := w.keysOffset
	w.keysOffset += SizeKeyLen + uint64(len(key))
	return offset, nil
}

// writeValue append value to values file
func (w *snapshotWriter) writeValue(value []byte) (uint64, error) {
	var buf [SizeValueLen]byte
	if len(value) > math.MaxUint32 {
		return 0, fmt.Errorf("value length overflow: %d", len(value))
	}
	binary.LittleEndian.PutUint32(buf[:], uint32(len(value)))
	if _, err := w.valuesWriter.Write(buf[:]); err != nil {
		return 0, err
	}
	if _, err := w.valuesWriter.Write(value); err != nil {
		return 0, err
	}
	offset := w.valuesOffset
	w.valuesOffset += SizeValueLen + uint64(len(value))
	return offset, nil
}

// writeRecursive write the node recursively in depth-first post-order,
// returns `(nodeIndex, offset of minimal key in subtree, err)`.
func (w *snapshotWriter) writeRecursive(node Node) (uint32, uint64, error) {
	var (
		buf              [SizeNodeWithoutHash]byte
		minimalKeyOffset uint64
	)

	buf[OffsetHeight] = byte(node.Height())
	if node.Version() > math.MaxUint32 {
		return 0, 0, fmt.Errorf("version overflow: %d", node.Version())
	}
	binary.LittleEndian.PutUint32(buf[OffsetVersion:], uint32(node.Version()))
	binary.LittleEndian.PutUint64(buf[OffsetSize:], uint64(node.Size()))

	if isLeaf(node) {
		offset, err := w.writeKey(node.Key())
		if err != nil {
			return 0, 0, err
		}
		binary.LittleEndian.PutUint64(buf[OffsetKey:], offset)
		minimalKeyOffset = offset

		offset, err = w.writeValue(node.Value())
		if err != nil {
			return 0, 0, err
		}
		binary.LittleEndian.PutUint64(buf[OffsetValue:], offset)
	} else {
		// it use the minimal key from right subtree, but propagate the minimal key from left subtree.
		nodeIndex, keyOffset, err := w.writeRecursive(node.Right())
		if err != nil {
			return 0, 0, err
		}
		binary.LittleEndian.PutUint64(buf[OffsetKey:], keyOffset)
		binary.LittleEndian.PutUint32(buf[OffsetRight:], nodeIndex)

		nodeIndex, minimalKeyOffset, err = w.writeRecursive(node.Left())
		if err != nil {
			return 0, 0, err
		}
		binary.LittleEndian.PutUint32(buf[OffsetLeft:], nodeIndex)
	}

	if _, err := w.nodesWriter.Write(buf[:]); err != nil {
		return 0, 0, err
	}
	if _, err := w.nodesWriter.Write(node.Hash()); err != nil {
		return 0, 0, err
	}

	i := w.nodeIndex
	w.nodeIndex++
	return i, minimalKeyOffset, nil
}

func Mmap(f *os.File) ([]byte, *[mmap.MaxMapSize]byte, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	return mmap.Mmap(f, int(fi.Size()))
}

func createFile(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
}

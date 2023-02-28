package memiavl

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/hashicorp/go-multierror"
	"github.com/ledgerwatch/erigon-lib/mmap"
	"github.com/ledgerwatch/erigon-lib/recsplit/eliasfano32"
)

const (
	// SnapshotFileMagic is little endian encoded b"IAVL"
	SnapshotFileMagic = 1280721225

	// the initial snapshot format
	SnapshotFormat = 0

	// magic: uint32, format: uint32, version: uint32, root node index: uint32
	SizeMetadata = 16

	// EmptyRootNodeIndex is a special value of root node index to represent empty tree
	EmptyRootNodeIndex = math.MaxUint32

	Alignment = 8

	OffsetRestartInteval = 65536
)

// Snapshot manage the lifecycle of mmap-ed files for the snapshot,
// it must out live the objects that derived from it.
type Snapshot struct {
	nodesMap *MmapFile
	kvsMap   *MmapFile

	nodes []byte
	kvs   []byte

	// parsed from metadata file
	version   uint32
	rootIndex uint32

	// wrapping the raw nodes buffer
	nodesLayout Nodes
}

func NewEmptySnapshot(version uint32) *Snapshot {
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
	version := binary.LittleEndian.Uint32(bz[8:])
	rootIndex := binary.LittleEndian.Uint32(bz[12:])

	if rootIndex == EmptyRootNodeIndex {
		// we can't mmap empty files, so have to return early
		return NewEmptySnapshot(version), nil
	}

	var nodesMap, kvsMap *MmapFile
	cleanupHandles := func(err error) error {
		if nodesMap != nil {
			if merr := nodesMap.Close(); merr != nil {
				err = multierror.Append(merr, err)
			}
		}
		if kvsMap != nil {
			if merr := kvsMap.Close(); merr != nil {
				err = multierror.Append(merr, err)
			}
		}
		return err
	}

	if nodesMap, err = NewMmap(filepath.Join(snapshotDir, "nodes")); err != nil {
		return nil, cleanupHandles(err)
	}
	if kvsMap, err = NewMmap(filepath.Join(snapshotDir, "kvs")); err != nil {
		return nil, cleanupHandles(err)
	}

	nodes := nodesMap.Data()
	kvs := kvsMap.Data()

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

	nodesData, err := NewNodes(nodes)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		nodesMap: nodesMap,
		kvsMap:   kvsMap,

		// cache the pointers
		nodes: nodes,
		kvs:   kvs,

		version:   version,
		rootIndex: rootIndex,

		nodesLayout: nodesData,
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
	if snapshot.kvsMap != nil {
		if merr := snapshot.kvsMap.Close(); merr != nil {
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
		index:    index,
	}
}

// Version returns the version of the snapshot
func (snapshot *Snapshot) Version() uint32 {
	return snapshot.version
}

// RootNode returns the root node
func (snapshot *Snapshot) RootNode() PersistedNode {
	if snapshot.rootIndex == EmptyRootNodeIndex {
		panic("RootNode not supported on an empty snapshot")
	}
	return snapshot.Node(snapshot.rootIndex)
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

// Key returns a zero-copy slice of key by offset
func (snapshot *Snapshot) Key(offset uint64) []byte {
	keyLen := binary.LittleEndian.Uint32(snapshot.kvs[offset:])
	offset += 4
	return snapshot.kvs[offset : offset+uint64(keyLen)]
}

// KeyValue returns a zero-copy slice of key/value pair by offset
func (snapshot *Snapshot) KeyValue(offset uint64) ([]byte, []byte) {
	len := uint64(binary.LittleEndian.Uint32(snapshot.kvs[offset:]))
	offset += 4
	key := snapshot.kvs[offset : offset+len]
	offset += len
	len = uint64(binary.LittleEndian.Uint32(snapshot.kvs[offset:]))
	offset += 4
	value := snapshot.kvs[offset : offset+len]
	return key, value
}

// WriteSnapshot save the IAVL tree to a new snapshot directory.
func (t *Tree) WriteSnapshot(snapshotDir string) (returnErr error) {
	var rootIndex uint32
	if t.root == nil {
		rootIndex = EmptyRootNodeIndex
	} else {
		nodesFile := filepath.Join(snapshotDir, "nodes")
		kvsFile := filepath.Join(snapshotDir, "kvs")

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

		w := newSnapshotWriter(nodesWriter, kvsWriter)
		rootIndex, err = w.writeRecursive(t.root)
		if err != nil {
			return err
		}
		if err := w.writeOffsets(); err != nil {
			return err
		}

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
	}

	// write metadata
	var metadataBuf [SizeMetadata]byte
	binary.LittleEndian.PutUint32(metadataBuf[:], SnapshotFileMagic)
	binary.LittleEndian.PutUint32(metadataBuf[4:], SnapshotFormat)
	binary.LittleEndian.PutUint32(metadataBuf[8:], t.version)
	binary.LittleEndian.PutUint32(metadataBuf[12:], rootIndex)

	metadataFile := filepath.Join(snapshotDir, "metadata")
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

type snapshotWriter struct {
	nodesWriter, kvWriter io.Writer

	// record the current node index and leaf node index
	nodeIndex uint32

	// record the current writing offset in keys/values file
	kvsOffset uint64
}

func newSnapshotWriter(nodesWriter, kvsWriter io.Writer) *snapshotWriter {
	return &snapshotWriter{
		nodesWriter: nodesWriter,
		kvWriter:    kvsWriter,
	}
}

// writeKeyValue append key to keys file and record the offset
func (w *snapshotWriter) writeKeyValue(key, value []byte) error {
	var numBuf [4]byte

	binary.LittleEndian.PutUint32(numBuf[:], uint32(len(key)))
	if _, err := w.kvWriter.Write(numBuf[:]); err != nil {
		return err
	}
	if _, err := w.kvWriter.Write(key); err != nil {
		return err
	}

	binary.LittleEndian.PutUint32(numBuf[:], uint32(len(value)))
	if _, err := w.kvWriter.Write(numBuf[:]); err != nil {
		return err
	}
	if _, err := w.kvWriter.Write(value); err != nil {
		return err
	}

	w.kvsOffset += 4 + 4 + uint64(len(key)) + uint64(len(value))
	return nil
}

// writeRecursive write the node recursively in depth-first post-order,
// returns `(nodeIndex, err)`.
func (w *snapshotWriter) writeRecursive(node Node) (uint32, error) {
	var buf [SizeNodeWithoutHash]byte

	buf[OffsetHeight] = node.Height()
	binary.LittleEndian.PutUint32(buf[OffsetVersion:], node.Version())

	if isLeaf(node) {
		keyOffset := w.kvsOffset
		if err := w.writeKeyValue(node.Key(), node.Value()); err != nil {
			return 0, err
		}

		binary.LittleEndian.PutUint64(buf[OffsetKeyOffset:], keyOffset)
	} else {
		binary.LittleEndian.PutUint32(buf[OffsetSize:], uint32(node.Size()))

		// store the minimal key from right subtree, but propagate the one from left subtree
		leftIndex, err := w.writeRecursive(node.Left())
		if err != nil {
			return 0, err
		}
		if _, err = w.writeRecursive(node.Right()); err != nil {
			return 0, err
		}
		binary.LittleEndian.PutUint32(buf[OffsetKeyNode:], leftIndex+1)
	}

	if _, err := w.nodesWriter.Write(buf[:]); err != nil {
		return 0, err
	}
	if _, err := w.nodesWriter.Write(node.Hash()); err != nil {
		return 0, err
	}

	i := w.nodeIndex
	w.nodeIndex++
	return i, nil
}

// writeOffsets writes the keys/values offsets with elias-fano encoding.
func (w *snapshotWriter) writeOffsets() error {

	return nil
}

func writePadding(w io.Writer, offset uint64) (uint64, error) {
	// align the beginning of EliasFano buffer to multiples of 8
	aligned := roundUp(offset, Alignment)
	if aligned > offset {
		// write padding zeroes
		if _, err := w.Write(make([]byte, aligned-offset)); err != nil {
			return 0, err
		}
	}
	return aligned, nil
}

// writeEliasFano writes the elias-fano encoded offset table
func writeEliasFano(w io.Writer, bitmap *roaring64.Bitmap) error {
	ef := eliasfano32.NewEliasFano(bitmap.GetCardinality(), bitmap.Maximum())
	it := bitmap.Iterator()
	for it.HasNext() {
		v := it.Next()
		ef.AddOffset(v)
	}
	ef.Build()
	if err := ef.Write(w); err != nil {
		return err
	}

	return nil
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

func roundUp(a, n uint64) uint64 {
	return ((a + n - 1) / n) * n
}

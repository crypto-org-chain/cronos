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
	nodesMap  *MmapFile
	keysMap   *MmapFile
	valuesMap *MmapFile

	nodes  []byte
	keys   []byte
	values []byte

	// parsed from metadata file
	version   uint32
	rootIndex uint32

	// plain offset table for keys
	keysOffsets PlainOffsetTable
	// elias-fano encoded offsets for values
	valuesOffsets *eliasfano32.EliasFano

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

	// Read the plain offsets table from the end of keys file
	keys := keysMap.Data()
	offset := binary.LittleEndian.Uint64(keys[len(keys)-8:])
	keysOffsets, err := NewPlainOffsetTable(keys[offset:])
	if err != nil {
		return nil, err
	}

	// Read the elias-fano offsets table from the end of values file
	values := valuesMap.Data()
	offset = binary.LittleEndian.Uint64(values[len(values)-8:])
	valuesOffsets, _ := eliasfano32.ReadEliasFano(values[offset:])

	nodesData, err := NewNodes(nodes)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		nodesMap:  nodesMap,
		keysMap:   keysMap,
		valuesMap: valuesMap,

		// cache the pointers
		nodes:  nodes,
		keys:   keys,
		values: values,

		version:   version,
		rootIndex: rootIndex,

		keysOffsets:   keysOffsets,
		valuesOffsets: valuesOffsets,

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

// Key returns a zero-copy slice of key by index
func (snapshot *Snapshot) Key(i uint64) []byte {
	start, end := snapshot.keysOffsets.Get2(i)
	return snapshot.keys[start:end]
}

// Value returns a zero-copy slice of value by index
func (snapshot *Snapshot) Value(i uint64) []byte {
	begin, end := snapshot.valuesOffsets.Get2(i)
	return snapshot.values[begin:end]
}

// WriteSnapshot save the IAVL tree to a new snapshot directory.
func (t *Tree) WriteSnapshot(snapshotDir string) (returnErr error) {
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
		defer func() {
			if err := fpNodes.Close(); returnErr == nil {
				returnErr = err
			}
		}()

		fpKeys, err := createFile(keysFile)
		if err != nil {
			return err
		}
		defer func() {
			if err := fpKeys.Close(); returnErr == nil {
				returnErr = err
			}
		}()

		fpValues, err := createFile(valuesFile)
		if err != nil {
			return err
		}
		defer func() {
			if err := fpValues.Close(); returnErr == nil {
				returnErr = err
			}
		}()
		nodesWriter := bufio.NewWriter(fpNodes)
		keysWriter := bufio.NewWriter(fpKeys)
		valuesWriter := bufio.NewWriter(fpValues)

		w := newSnapshotWriter(nodesWriter, keysWriter, valuesWriter)
		rootIndex, _, err = w.writeRecursive(t.root)
		if err != nil {
			return err
		}
		if err := w.writeOffsets(); err != nil {
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
	nodesWriter, keysWriter, valuesWriter io.Writer

	// record the current node index and leaf node index
	nodeIndex, leafIndex uint32

	// record the current writing offset in keys/values file
	keysOffset, valuesOffset uint64

	// record the offsets for keys and values
	keysOffsetBM, valuesOffsetBM *roaring64.Bitmap
}

func newSnapshotWriter(nodesWriter, keysWriter, valuesWriter io.Writer) *snapshotWriter {
	return &snapshotWriter{
		nodesWriter:    nodesWriter,
		keysWriter:     keysWriter,
		valuesWriter:   valuesWriter,
		keysOffsetBM:   roaring64.New(),
		valuesOffsetBM: roaring64.New(),
	}
}

// writeKey append key to keys file and record the offset
func (w *snapshotWriter) writeKey(key []byte) error {
	if _, err := w.keysWriter.Write(key); err != nil {
		return err
	}
	w.keysOffsetBM.Add(w.keysOffset)
	w.keysOffset += uint64(len(key))
	return nil
}

// writeValue append value to values file and record the offset
func (w *snapshotWriter) writeValue(value []byte) error {
	if _, err := w.valuesWriter.Write(value); err != nil {
		return err
	}
	w.valuesOffsetBM.Add(w.valuesOffset)
	w.valuesOffset += uint64(len(value))
	return nil
}

// writeRecursive write the node recursively in depth-first post-order,
// returns `(nodeIndex, offset of minimal key in subtree, err)`.
func (w *snapshotWriter) writeRecursive(node Node) (uint32, uint32, error) {
	var (
		buf [SizeNodeWithoutHash]byte
		// record the minimal leaf node in the current subtree, used to update key field in parent node.
		minimalKeyIndex uint32
	)

	buf[OffsetHeight] = node.Height()
	binary.LittleEndian.PutUint32(buf[OffsetVersion:], node.Version())
	binary.LittleEndian.PutUint32(buf[OffsetSize:], uint32(node.Size()))

	if isLeaf(node) {
		if err := w.writeKey(node.Key()); err != nil {
			return 0, 0, err
		}
		if err := w.writeValue(node.Value()); err != nil {
			return 0, 0, err
		}

		binary.LittleEndian.PutUint32(buf[OffsetLeafIndex:], w.leafIndex)

		minimalKeyIndex = w.nodeIndex
		w.leafIndex++
	} else {
		// store the minimal key from right subtree, but propagate the one from left subtree
		var err error
		if _, minimalKeyIndex, err = w.writeRecursive(node.Left()); err != nil {
			return 0, 0, err
		}
		_, keyNode, err := w.writeRecursive(node.Right())
		if err != nil {
			return 0, 0, err
		}
		binary.LittleEndian.PutUint32(buf[OffsetKeyNode:], keyNode)
	}

	if _, err := w.nodesWriter.Write(buf[:]); err != nil {
		return 0, 0, err
	}
	if _, err := w.nodesWriter.Write(node.Hash()); err != nil {
		return 0, 0, err
	}

	i := w.nodeIndex
	w.nodeIndex++
	return i, minimalKeyIndex, nil
}

// writeOffsets writes the keys/values offsets with elias-fano encoding.
func (w *snapshotWriter) writeOffsets() error {
	// add the ending offset
	w.keysOffsetBM.Add(w.keysOffset)
	w.valuesOffsetBM.Add(w.valuesOffset)

	var (
		err    error
		offset uint64
		numBuf [8]byte
	)

	// align to multiples of 8
	if offset, err = writePadding(w.keysWriter, w.keysOffset); err != nil {
		return err
	}
	// write plain little-endian offsets for keys
	if err := writePlainOffsets(w.keysWriter, w.keysOffsetBM); err != nil {
		return err
	}
	// append the start offset of offset table to the file end
	binary.LittleEndian.PutUint64(numBuf[:], offset)
	if _, err := w.keysWriter.Write(numBuf[:]); err != nil {
		return err
	}

	// align to multiples of 8
	if offset, err = writePadding(w.valuesWriter, w.valuesOffset); err != nil {
		return err
	}
	// write elias-fano encoded offset table for values file
	if err := writeEliasFano(w.valuesWriter, w.valuesOffsetBM); err != nil {
		return err
	}
	// append the start offset of offset table to the file end
	binary.LittleEndian.PutUint64(numBuf[:], offset)
	if _, err := w.valuesWriter.Write(numBuf[:]); err != nil {
		return err
	}

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

// writePlainOffsets writes the offset table in plain little-endian format
func writePlainOffsets(w io.Writer, bitmap *roaring64.Bitmap) error {
	var numBuf [8]byte
	it := bitmap.Iterator()
	var counter, restart uint64
	for it.HasNext() {
		v := it.Next()
		if counter%OffsetRestartInteval == 0 {
			binary.LittleEndian.PutUint64(numBuf[:], v)
			restart = v

			if _, err := w.Write(numBuf[:]); err != nil {
				return err
			}
		} else {
			binary.LittleEndian.PutUint32(numBuf[:], uint32(v-restart))
			if _, err := w.Write(numBuf[:4]); err != nil {
				return err
			}
		}
		counter++
	}
	return nil
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

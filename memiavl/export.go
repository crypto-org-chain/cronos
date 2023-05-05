package memiavl

import (
	"context"
	"fmt"
	"math"

	"cosmossdk.io/errors"
	snapshottypes "github.com/cosmos/cosmos-sdk/snapshots/types"
	"github.com/cosmos/iavl"
	protoio "github.com/gogo/protobuf/io"
)

// exportBufferSize is the number of nodes to buffer in the exporter. It improves throughput by
// processing multiple nodes per context switch, but take care to avoid excessive memory usage,
// especially since callers may export several IAVL stores in parallel (e.g. the Cosmos SDK).
const exportBufferSize = 32

func (db *DB) Snapshot(height uint64, protoWriter protoio.Writer) error {
	if height > math.MaxUint32 {
		return fmt.Errorf("height overflows uint32: %d", height)
	}

	mtree, err := LoadMultiTree(snapshotPath(db.dir, uint32(height)), true)
	if err != nil {
		return errors.Wrapf(err, "invalid snapshot height: %d", height)
	}

	for _, tree := range mtree.trees {
		if err := protoWriter.WriteMsg(&snapshottypes.SnapshotItem{
			Item: &snapshottypes.SnapshotItem_Store{
				Store: &snapshottypes.SnapshotStoreItem{
					Name: tree.name,
				},
			},
		}); err != nil {
			return err
		}

		exporter := tree.tree.snapshot.Export()
		for {
			node, err := exporter.Next()
			if err == iavl.ExportDone {
				break
			} else if err != nil {
				return err
			}
			if err := protoWriter.WriteMsg(&snapshottypes.SnapshotItem{
				Item: &snapshottypes.SnapshotItem_IAVL{
					IAVL: &snapshottypes.SnapshotIAVLItem{
						Key:     node.Key,
						Value:   node.Value,
						Height:  int32(node.Height),
						Version: node.Version,
					},
				},
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

type Exporter struct {
	snapshot *Snapshot
	ch       chan *iavl.ExportNode
	cancel   context.CancelFunc
}

func newExporter(snapshot *Snapshot) *Exporter {
	ctx, cancel := context.WithCancel(context.Background())
	exporter := &Exporter{
		snapshot: snapshot,
		ch:       make(chan *iavl.ExportNode, exportBufferSize),
		cancel:   cancel,
	}
	go exporter.export(ctx)
	return exporter
}

func (e *Exporter) export(ctx context.Context) {
	defer close(e.ch)

	if e.snapshot.leavesLen() == 0 {
		return
	}

	if e.snapshot.leavesLen() == 1 {
		leaf := e.snapshot.Leaf(0)
		e.ch <- &iavl.ExportNode{
			Height:  0,
			Version: int64(leaf.Version()),
			Key:     leaf.Key(),
			Value:   leaf.Value(),
		}
		return
	}

	var pendingTrees int
	var i, j uint32
	for ; i < uint32(e.snapshot.nodesLen()); i++ {
		// pending branch node
		node := e.snapshot.nodesLayout.Node(i)
		for pendingTrees < int(node.PreTrees())+2 {
			// add more leaf nodes
			leaf := e.snapshot.leavesLayout.Leaf(j)
			key, value := e.snapshot.KeyValue(leaf.KeyOffset())
			enode := &iavl.ExportNode{
				Height:  0,
				Version: int64(leaf.Version()),
				Key:     key,
				Value:   value,
			}
			j++
			pendingTrees++

			select {
			case e.ch <- enode:
			case <-ctx.Done():
				return
			}
		}
		enode := &iavl.ExportNode{
			Height:  int8(node.Height()),
			Version: int64(node.Version()),
			Key:     e.snapshot.LeafKey(node.KeyLeaf()),
		}
		pendingTrees--

		select {
		case e.ch <- enode:
		case <-ctx.Done():
			return
		}
	}
}

func (e *Exporter) Next() (*iavl.ExportNode, error) {
	if exportNode, ok := <-e.ch; ok {
		return exportNode, nil
	}
	return nil, iavl.ExportDone

}

// Close closes the exporter. It is safe to call multiple times.
func (e *Exporter) Close() {
	e.cancel()
	for range e.ch { // drain channel
	}
	e.snapshot = nil
}

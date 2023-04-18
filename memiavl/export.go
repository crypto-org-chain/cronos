package memiavl

import (
	"fmt"
	"math"

	snapshottypes "github.com/cosmos/cosmos-sdk/snapshots/types"
	"github.com/cosmos/iavl"
	protoio "github.com/gogo/protobuf/io"
)

func (db *DB) Snapshot(height uint64, protoWriter protoio.Writer) error {
	if height > math.MaxUint32 {
		return fmt.Errorf("height overflows uint32: %d", height)
	}

	mtree, err := LoadMultiTree(snapshotPath(db.dir, uint32(height)))
	if err != nil {
		return err
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
	i        uint32
	count    int
}

func (e *Exporter) Next() (*iavl.ExportNode, error) {
	if int(e.i) >= e.count {
		return nil, iavl.ExportDone
	}
	node := e.snapshot.Node(e.i)
	e.i++

	height := node.Height()
	var value []byte
	if height == 0 {
		value = node.Value()
	}
	return &iavl.ExportNode{
		Height:  int8(height),
		Version: int64(node.Version()),
		Key:     node.Key(),
		Value:   value,
	}, nil
}

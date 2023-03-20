package memiavl

import (
	"github.com/cosmos/iavl"
)

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

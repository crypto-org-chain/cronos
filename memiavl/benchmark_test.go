package memiavl

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/btree"
)

func BenchmarkByteCompare(b *testing.B) {
	var x, y [32]byte
	for i := 0; i < b.N; i++ {
		_ = bytes.Compare(x[:], y[:])
	}
}

func BenchmarkRandomGet(b *testing.B) {
	items := genRandItems(1000000)
	targetKey := items[500].key
	targetValue := items[500].value
	targetItem := itemT{key: targetKey}

	tree := New()
	for _, item := range items {
		tree.Set(item.key, item.value)
	}

	bt2 := btree.NewBTreeGOptions(lessG, btree.Options{
		NoLocks: true,
		Degree:  2,
	})
	for _, item := range items {
		bt2.Set(item)
	}

	bt32 := btree.NewBTreeGOptions(lessG, btree.Options{
		NoLocks: true,
		Degree:  32,
	})
	for _, item := range items {
		bt32.Set(item)
	}

	snapshotDir := b.TempDir()
	err := tree.WriteSnapshot(snapshotDir)
	require.NoError(b, err)
	snapshot, err := OpenSnapshot(snapshotDir)
	require.NoError(b, err)
	defer snapshot.Close()
	diskTree := NewFromSnapshot(snapshot)

	require.Equal(b, targetValue, tree.Get(targetKey))
	require.Equal(b, targetValue, diskTree.Get(targetKey))
	require.Equal(b, targetValue, snapshot.Get(targetKey))
	v, _ := bt2.Get(targetItem)
	require.Equal(b, targetValue, v.value)
	v, _ = bt32.Get(targetItem)
	require.Equal(b, targetValue, v.value)

	b.ResetTimer()
	b.Run("memiavl", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = tree.Get(targetKey)
		}
	})
	b.Run("memiavl-disk", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = diskTree.Get(targetKey)
		}
	})
	b.Run("snapshot-get", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = snapshot.Get(targetKey)
		}
	})
	b.Run("btree-degree-2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = bt2.Get(targetItem)
		}
	})
	b.Run("btree-degree-32", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = bt32.Get(targetItem)
		}
	})
}

func BenchmarkRandomSet(b *testing.B) {
	items := genRandItems(1000000)
	b.ResetTimer()
	b.Run("memiavl", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tree := New()
			for _, item := range items {
				tree.Set(item.key, item.value)
			}
		}
	})
	b.Run("tree2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			bt := btree.NewBTreeGOptions(lessG, btree.Options{
				NoLocks: true,
				Degree:  2,
			})
			for _, item := range items {
				bt.Set(item)
			}
		}
	})
	b.Run("tree32", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			bt := btree.NewBTreeGOptions(lessG, btree.Options{
				NoLocks: true,
				Degree:  32,
			})
			for _, item := range items {
				bt.Set(item)
			}
		}
	})
}

type itemT struct {
	key, value []byte
}

func lessG(a, b itemT) bool {
	return bytes.Compare(a.key, b.key) == -1
}

func int64ToItemT(n uint64) itemT {
	var key, value [8]byte
	binary.BigEndian.PutUint64(key[:], n)
	binary.LittleEndian.PutUint64(value[:], n)
	return itemT{
		key:   key[:],
		value: value[:],
	}
}

func genRandItems(n int) []itemT {
	r := rand.New(rand.NewSource(0))
	items := make([]itemT, n)
	itemsM := make(map[uint64]bool)
	for i := 0; i < n; i++ {
		for {
			key := uint64(r.Int63n(10000000000000000))
			if !itemsM[key] {
				itemsM[key] = true
				items[i] = int64ToItemT(key)
				break
			}
		}
	}
	return items
}

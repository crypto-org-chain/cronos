package memiavl

import (
	"math/rand"
	"testing"

	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/ledgerwatch/erigon-lib/recsplit/eliasfano32"
	"github.com/stretchr/testify/require"
)

const TestN = 1000000

func genTestData(n int) []uint64 {
	rand.Seed(0)
	result := make([]uint64, n)
	var offset uint64
	for i := 0; i < n; i++ {
		result[i] = offset
		offset += 32 + uint64(rand.Int63n(15))
	}
	return result
}

func BenchmarkOffsetsRoaring(b *testing.B) {
	data := genTestData(1000000)
	builder := roaring64.New()
	for _, n := range data {
		builder.Add(n)
	}
	builder.RunOptimize()
	bz, err := builder.ToBytes()
	require.NoError(b, err)
	// b.Logf("RoaringBitmap size %d", len(bz))

	bm := roaring64.New()
	require.NoError(b, bm.UnmarshalBinary(bz))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// need two offsets to construct the slice
		idx := uint64(i % (len(data) - 1))
		_, err := bm.Select(idx)
		require.NoError(b, err)
		_, err = bm.Select(idx + 1)
		require.NoError(b, err)
	}
}

func BenchmarkOffsetsEliasfano32(b *testing.B) {
	data := genTestData(1000000)
	builder := eliasfano32.NewEliasFano(uint64(len(data)), data[len(data)-1])
	for _, n := range data {
		builder.AddOffset(n)
	}
	builder.Build()
	bz := builder.AppendBytes(nil)
	// b.Logf("EliasFano size %d", len(bz))

	ef, _ := eliasfano32.ReadEliasFano(bz)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// need two offsets to construct the slice
		offset1, offset2 := ef.Get2(uint64(i % (len(data) - 1)))
		_ = offset2 - offset1
	}
}

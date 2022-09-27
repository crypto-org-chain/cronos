package versiondb

import (
	"encoding/binary"
	"sort"

	"github.com/RoaringBitmap/roaring/roaring64"
)

var ChunkLimit = uint64(1950) // threshold beyond which MDBX overflow pages appear: 4096 / 2 - (keySize + 8)

// CutLeft - cut from bitmap `targetSize` bytes from left
// removing lft part from `bm`
// returns nil on zero cardinality
func CutLeft64(bm *roaring64.Bitmap, sizeLimit uint64) *roaring64.Bitmap {
	if bm.GetCardinality() == 0 {
		return nil
	}

	sz := bm.GetSerializedSizeInBytes()
	if sz <= sizeLimit {
		lft := roaring64.New()
		lft.AddRange(bm.Minimum(), bm.Maximum()+1)
		lft.And(bm)
		lft.RunOptimize()
		bm.Clear()
		return lft
	}

	from := bm.Minimum()
	minMax := bm.Maximum() - bm.Minimum()
	to := sort.Search(int(minMax), func(i int) bool { // can be optimized to avoid "too small steps", but let's leave it for readability
		lft := roaring64.New() // bitmap.Clear() method intentionally not used here, because then serialized size of bitmap getting bigger
		lft.AddRange(from, from+uint64(i)+1)
		lft.And(bm)
		lft.RunOptimize()
		return lft.GetSerializedSizeInBytes() > sizeLimit
	})

	lft := roaring64.New()
	lft.AddRange(from, from+uint64(to)) // no +1 because sort.Search returns element which is just higher threshold - but we need lower
	lft.And(bm)
	bm.RemoveRange(from, from+uint64(to))
	lft.RunOptimize()
	return lft
}

func WalkChunks64(bm *roaring64.Bitmap, sizeLimit uint64, f func(chunk *roaring64.Bitmap, isLast bool) error) error {
	for bm.GetCardinality() > 0 {
		if err := f(CutLeft64(bm, sizeLimit), bm.GetCardinality() == 0); err != nil {
			return err
		}
	}
	return nil
}

func WalkChunkWithKeys64(k []byte, m *roaring64.Bitmap, sizeLimit uint64, f func(chunkKey []byte, chunk *roaring64.Bitmap) error) error {
	return WalkChunks64(m, sizeLimit, func(chunk *roaring64.Bitmap, isLast bool) error {
		chunkKey := make([]byte, len(k)+8)
		copy(chunkKey, k)
		if isLast {
			binary.BigEndian.PutUint64(chunkKey[len(k):], ^uint64(0))
		} else {
			binary.BigEndian.PutUint64(chunkKey[len(k):], chunk.Maximum())
		}
		return f(chunkKey, chunk)
	})
}

// SeekInBitmap64 - returns value in bitmap which is >= n
func SeekInBitmap64(m *roaring64.Bitmap, n uint64) (found uint64, ok bool) {
	if m == nil || m.IsEmpty() {
		return 0, false
	}
	if n == 0 {
		return m.Minimum(), true
	}
	searchRank := m.Rank(n - 1)
	if searchRank >= m.GetCardinality() {
		return 0, false
	}
	found, _ = m.Select(searchRank)
	return found, true
}

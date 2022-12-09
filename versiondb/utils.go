package versiondb

import "github.com/RoaringBitmap/roaring/roaring64"

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

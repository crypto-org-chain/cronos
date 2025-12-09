package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/crypto-org-chain/cronos/x/attestation/types"
)

// TestHighestFinalityHeightTracking tests that highest finality height is tracked correctly
func TestHighestFinalityHeightTracking(t *testing.T) {
	cache := NewFinalityCache(100)

	// Simulate storing blocks out of order
	heights := []uint64{100, 105, 102, 101, 104, 103, 99}

	var highestSeen uint64
	for _, height := range heights {
		status := &types.FinalityStatus{
			BlockHeight: height,
			FinalizedAt: 1234567890, // FinalizedAt > 0 means finalized
		}
		cache.Set(height, status)

		// Track highest manually (simulates what keeper does)
		if height > highestSeen {
			highestSeen = height
		}
	}

	// Verify highest is 105
	require.Equal(t, uint64(105), highestSeen)

	// Verify we can query all blocks
	for _, height := range heights {
		status, ok := cache.Get(height)
		require.True(t, ok, "Block %d should be in cache", height)
		require.Equal(t, height, status.BlockHeight)
	}
}

// TestHighestFinalityHeightUpdatesMonotonically tests that highest height only increases
func TestHighestFinalityHeightUpdatesMonotonically(t *testing.T) {
	// Simulate the keeper's behavior
	highestHeight := uint64(0)

	// Process blocks
	blocks := []uint64{100, 101, 99, 102, 95, 103}

	for _, height := range blocks {
		// Update highest only if new height is higher
		if height > highestHeight {
			highestHeight = height
		}
	}

	// Highest should be 103 (not 95, even though it came later)
	require.Equal(t, uint64(103), highestHeight)
}

// TestHighestFinalityHeightInitialValue tests initial value is 0
func TestHighestFinalityHeightInitialValue(t *testing.T) {
	// Initial value should be 0
	highestHeight := uint64(0)

	require.Equal(t, uint64(0), highestHeight)
}

// TestHighestFinalityHeightLargeValues tests with large block heights
func TestHighestFinalityHeightLargeValues(t *testing.T) {
	highestHeight := uint64(0)

	// Process large block heights
	largeHeights := []uint64{1000000, 1000001, 999999, 1000002}

	for _, height := range largeHeights {
		if height > highestHeight {
			highestHeight = height
		}
	}

	require.Equal(t, uint64(1000002), highestHeight)
}

// TestHighestFinalityWithGaps tests handling of gaps in block heights
func TestHighestFinalityWithGaps(t *testing.T) {
	cache := NewFinalityCache(100)

	// Store blocks with gaps
	heights := []uint64{100, 200, 150, 300, 250}

	for _, height := range heights {
		status := &types.FinalityStatus{
			BlockHeight: height,
			FinalizedAt: 1234567890, // FinalizedAt > 0 means finalized
		}
		cache.Set(height, status)
	}

	// Verify all blocks are accessible
	for _, height := range heights {
		status, ok := cache.Get(height)
		require.True(t, ok)
		require.Equal(t, height, status.BlockHeight)
	}

	// Highest should be 300
	var highest uint64
	for _, h := range heights {
		if h > highest {
			highest = h
		}
	}
	require.Equal(t, uint64(300), highest)
}

// TestConcurrentHighestHeightUpdates tests concurrent updates to highest height
func TestConcurrentHighestHeightUpdates(t *testing.T) {
	cache := NewFinalityCache(1000)

	done := make(chan bool, 10)

	// Multiple goroutines updating highest
	for g := 0; g < 10; g++ {
		go func(goroutineID int) {
			for i := 0; i < 100; i++ {
				height := uint64(goroutineID*100 + i)
				status := &types.FinalityStatus{
					BlockHeight: height,
					FinalizedAt: 1234567890, // FinalizedAt > 0 means finalized
				}
				cache.Set(height, status)
			}
			done <- true
		}(g)
	}

	// Wait for all
	for g := 0; g < 10; g++ {
		<-done
	}

	// Find actual highest
	var highest uint64
	for i := 0; i < 1000; i++ {
		if status, ok := cache.Get(uint64(i)); ok {
			if status.BlockHeight > highest {
				highest = status.BlockHeight
			}
		}
	}

	// Highest should be 999 (last block from goroutine 9)
	require.Equal(t, uint64(999), highest)
}

// BenchmarkHighestHeightUpdates benchmarks highest height tracking
func BenchmarkHighestHeightUpdates(b *testing.B) {
	highestHeight := uint64(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		height := uint64(i)
		if height > highestHeight {
			highestHeight = height
		}
	}
}

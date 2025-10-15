package memiavl

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alitto/pond"
	"github.com/stretchr/testify/require"
)

func TestMultiTreeWriteSnapshotWithContextCancellation(t *testing.T) {
	mtree := NewEmptyMultiTree(0, 0)

	stores := []string{"store1", "store2", "store3", "store4", "store5"}
	var upgrades []*TreeNameUpgrade
	for _, name := range stores {
		upgrades = append(upgrades, &TreeNameUpgrade{Name: name})
	}
	require.NoError(t, mtree.ApplyUpgrades(upgrades))

	for _, storeName := range stores {
		tree := mtree.TreeByName(storeName)
		require.NotNil(t, tree)

		for i := 0; i < 1000; i++ {
			tree.set([]byte(string(rune('a'+i%26))+string(rune('a'+(i/26)%26))), []byte("value"))
		}
	}

	_, err := mtree.SaveVersion(true)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	pool := pond.New(2, 10)
	defer pool.StopAndWait()

	snapshotDir := t.TempDir()

	cancel()

	err = mtree.WriteSnapshotWithContext(ctx, snapshotDir, pool)

	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestMultiTreeWriteSnapshotWithTimeoutContext(t *testing.T) {
	mtree := NewEmptyMultiTree(0, 0)

	stores := []string{"store1", "store2", "store3"}
	var upgrades []*TreeNameUpgrade
	for _, name := range stores {
		upgrades = append(upgrades, &TreeNameUpgrade{Name: name})
	}
	require.NoError(t, mtree.ApplyUpgrades(upgrades))

	for _, storeName := range stores {
		tree := mtree.TreeByName(storeName)
		require.NotNil(t, tree)

		for i := 0; i < 500; i++ {
			tree.set([]byte(string(rune('a'+i%26))+string(rune('a'+(i/26)%26))), []byte("value"))
		}
	}

	_, err := mtree.SaveVersion(true)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond)

	pool := pond.New(2, 10)
	defer pool.StopAndWait()

	snapshotDir := t.TempDir()

	err = mtree.WriteSnapshotWithContext(ctx, snapshotDir, pool)

	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestMultiTreeWriteSnapshotSuccessWithContext(t *testing.T) {
	mtree := NewEmptyMultiTree(0, 0)

	stores := []string{"store1", "store2", "store3"}
	var upgrades []*TreeNameUpgrade
	for _, name := range stores {
		upgrades = append(upgrades, &TreeNameUpgrade{Name: name})
	}
	require.NoError(t, mtree.ApplyUpgrades(upgrades))

	for _, storeName := range stores {
		tree := mtree.TreeByName(storeName)
		require.NotNil(t, tree)

		for i := 0; i < 100; i++ {
			key := []byte(storeName + string(rune('a'+i%26)))
			value := []byte("value" + string(rune('0'+i%10)))
			tree.set(key, value)
		}
	}

	_, err := mtree.SaveVersion(true)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool := pond.New(4, 10)
	defer pool.StopAndWait()

	snapshotDir := t.TempDir()

	err = mtree.WriteSnapshotWithContext(ctx, snapshotDir, pool)
	require.NoError(t, err)

	// Verify all stores were written
	for _, storeName := range stores {
		storeDir := filepath.Join(snapshotDir, storeName)
		require.DirExists(t, storeDir)

		// Verify metadata file exists
		metadataFile := filepath.Join(storeDir, FileNameMetadata)
		require.FileExists(t, metadataFile)
	}

	// Verify metadata file was written at root
	metadataFile := filepath.Join(snapshotDir, MetadataFileName)
	require.FileExists(t, metadataFile)

	// Verify we can load the snapshot back
	mtree2, err := LoadMultiTree(snapshotDir, false, 0)
	require.NoError(t, err)
	defer mtree2.Close()

	require.Equal(t, mtree.Version(), mtree2.Version())
	require.Equal(t, len(mtree.trees), len(mtree2.trees))

	// Verify data integrity
	for _, storeName := range stores {
		tree1 := mtree.TreeByName(storeName)
		tree2 := mtree2.TreeByName(storeName)
		require.NotNil(t, tree1)
		require.NotNil(t, tree2)
		require.Equal(t, tree1.RootHash(), tree2.RootHash())
	}
}

func TestMultiTreeWriteSnapshotConcurrentCancellation(t *testing.T) {
	mtree := NewEmptyMultiTree(0, 0)

	stores := []string{"store1", "store2", "store3", "store4", "store5", "store6", "store7", "store8"}
	var upgrades []*TreeNameUpgrade
	for _, name := range stores {
		upgrades = append(upgrades, &TreeNameUpgrade{Name: name})
	}
	require.NoError(t, mtree.ApplyUpgrades(upgrades))

	for _, storeName := range stores {
		tree := mtree.TreeByName(storeName)
		require.NotNil(t, tree)

		for i := 0; i < 2000; i++ {
			key := []byte(storeName + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)))
			value := []byte("value" + string(rune('0'+i%10)))
			tree.set(key, value)
		}
	}

	_, err := mtree.SaveVersion(true)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	pool := pond.New(2, 10)
	defer pool.StopAndWait()

	snapshotDir := t.TempDir()

	errChan := make(chan error, 1)
	go func() {
		errChan <- mtree.WriteSnapshotWithContext(ctx, snapshotDir, pool)
	}()

	time.Sleep(5 * time.Millisecond)
	cancel()

	err = <-errChan

	// Should return context.Canceled error
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)

	// Verify that the snapshot directory might be partially written or not at all
	// This is acceptable - the important part is that we got the error and stopped
	_, statErr := os.Stat(snapshotDir)
	if statErr == nil {
		// Directory exists, but may be incomplete - this is fine
		// The important thing is we stopped and returned an error
		t.Logf("this is acceptable")
	}
}

func TestMultiTreeWriteSnapshotEmptyTree(t *testing.T) {
	mtree := NewEmptyMultiTree(0, 0)

	stores := []string{"empty1", "empty2"}
	var upgrades []*TreeNameUpgrade
	for _, name := range stores {
		upgrades = append(upgrades, &TreeNameUpgrade{Name: name})
	}
	require.NoError(t, mtree.ApplyUpgrades(upgrades))

	_, err := mtree.SaveVersion(true)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool := pond.New(4, 10)
	defer pool.StopAndWait()

	snapshotDir := t.TempDir()

	err = mtree.WriteSnapshotWithContext(ctx, snapshotDir, pool)
	require.NoError(t, err)

	mtree2, err := LoadMultiTree(snapshotDir, false, 0)
	require.NoError(t, err)
	defer mtree2.Close()

	require.Equal(t, mtree.Version(), mtree2.Version())
}

func TestMultiTreeWriteSnapshotParallelWrites(t *testing.T) {
	mtree := NewEmptyMultiTree(0, 0)

	stores := []string{"store1", "store2", "store3", "store4", "store5", "store6", "store7", "store8", "store9", "store10"}
	var upgrades []*TreeNameUpgrade
	for _, name := range stores {
		upgrades = append(upgrades, &TreeNameUpgrade{Name: name})
	}
	require.NoError(t, mtree.ApplyUpgrades(upgrades))

	for _, storeName := range stores {
		tree := mtree.TreeByName(storeName)
		require.NotNil(t, tree)

		for i := 0; i < 100; i++ {
			key := []byte(storeName + string(rune('a'+i%26)))
			value := []byte("value" + string(rune('0'+i%10)))
			tree.set(key, value)
		}
	}

	_, err := mtree.SaveVersion(true)
	require.NoError(t, err)

	ctx := context.Background()

	poolSizes := []int{1, 2, 4, 8}
	for _, poolSize := range poolSizes {
		t.Run("PoolSize"+string(rune('0'+poolSize)), func(t *testing.T) {
			pool := pond.New(poolSize, poolSize*10)
			defer pool.StopAndWait()

			snapshotDir := t.TempDir()

			start := time.Now()
			err = mtree.WriteSnapshotWithContext(ctx, snapshotDir, pool)
			duration := time.Since(start)

			require.NoError(t, err)
			t.Logf("Pool size %d completed in %v", poolSize, duration)

			mtree2, err := LoadMultiTree(snapshotDir, false, 0)
			require.NoError(t, err)
			defer mtree2.Close()

			require.Equal(t, mtree.Version(), mtree2.Version())
			for _, storeName := range stores {
				tree1 := mtree.TreeByName(storeName)
				tree2 := mtree2.TreeByName(storeName)
				require.Equal(t, tree1.RootHash(), tree2.RootHash())
			}
		})
	}
}

// TestMultiTreeWorkerPoolQueuedTasksShouldNotStart tests that when context is
// canceled, tasks that are queued but haven't started executing should NOT run.
// This test DEMONSTRATES THE BUG at line 374 where context.Background() is used
// instead of the passed ctx, causing all queued tasks to execute even after cancellation.
func TestMultiTreeWorkerPoolQueuedTasksShouldNotStart(t *testing.T) {
	mtree := NewEmptyMultiTree(0, 0)

	// Create many stores to ensure tasks will be queued
	numStores := 20
	var stores []string
	var upgrades []*TreeNameUpgrade
	for i := 0; i < numStores; i++ {
		name := "store" + string(rune('0'+i%10)) + string(rune('a'+i/10))
		stores = append(stores, name)
		upgrades = append(upgrades, &TreeNameUpgrade{Name: name})
	}
	require.NoError(t, mtree.ApplyUpgrades(upgrades))

	// Don't add any data - use empty trees so writeLeaf won't be called
	// This means tasks won't check ctx.Done() internally
	_, err := mtree.SaveVersion(true)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	// Create worker pool with only 1 worker but capacity for all tasks
	// This ensures most tasks will be queued waiting for the worker
	pool := pond.New(1, numStores)
	defer pool.StopAndWait()

	// Track how many tasks actually executed
	var tasksExecuted atomic.Int32

	// We need to slow down task execution so we can cancel while tasks are queued
	// We'll patch this by checking the execution count after cancellation

	snapshotDir := t.TempDir()

	// Cancel context immediately
	cancel()

	// Now call WriteSnapshotWithContext
	// BUG: Because line 374 uses context.Background(), the worker pool group
	// doesn't know about the cancellation. All 20 tasks will be submitted to the pool.
	// With only 1 worker, they'll execute one by one.

	// Since we're using empty trees, tree.WriteSnapshotWithContext doesn't actually
	// check ctx (no data to write means no ctx.Done() check in writeLeaf).
	// So all tasks will complete successfully despite ctx being canceled.

	err = mtree.WriteSnapshotWithContext(ctx, snapshotDir, pool)

	// With the BUG (context.Background() at line 374):
	// - All tasks get queued
	// - Worker executes them one by one
	// - Empty trees don't trigger context checks
	// - Result: err == nil (SUCCESS despite canceled context)
	//
	// With the FIX (using ctx at line 374):
	// - Worker pool's group context would be canceled
	// - Queued tasks wouldn't start
	// - Result: err == context.Canceled

	if err == nil {
		// This proves the bug exists!
		t.Logf("BUG REPRODUCED: All %d tasks completed despite canceled context!", numStores)
		t.Logf("Tasks executed: %d", tasksExecuted.Load())
		t.Logf("This happens because line 374 uses context.Background() instead of ctx")

		// Verify all stores were actually written (proving tasks ran)
		for _, storeName := range stores {
			storeDir := filepath.Join(snapshotDir, storeName)
			if _, err := os.Stat(storeDir); err == nil {
				tasksExecuted.Add(1)
			}
		}

		t.Logf("Verified: %d stores were written to disk", tasksExecuted.Load())
		t.Fatal("Expected context.Canceled error but got nil - this proves the bug at line 374")
	} else {
		// If we get here, the bug has been fixed
		t.Logf("Bug is FIXED: Got expected error: %v", err)
		require.ErrorIs(t, err, context.Canceled)
	}
}

package mempool

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// newAsyncRecheckFixture builds a recheckFixture whose Manager runs the async
// recheck worker. Registers t.Cleanup(Close) so each test is self-contained.
func newAsyncRecheckFixture(t *testing.T, failBytes ...string) *recheckFixture {
	t.Helper()
	f := newRecheckFixture(failBytes...)
	f.a.async.trigger = make(chan chan struct{}, 1)
	f.a.async.stop = make(chan struct{})
	f.a.async.done = make(chan struct{})
	f.a.async.ready = make(chan struct{})
	close(f.a.async.ready) // idle at start
	go f.a.recheckWorker()
	t.Cleanup(func() { f.a.Close() })
	return f
}

// startAsyncWorker wires the async fields onto an existing recheckFixture and
// starts the worker. Does NOT register a Cleanup — caller owns Close().
func startAsyncWorker(f *recheckFixture) {
	f.a.async.trigger = make(chan chan struct{}, 1)
	f.a.async.stop = make(chan struct{})
	f.a.async.done = make(chan struct{})
	f.a.async.ready = make(chan struct{})
	close(f.a.async.ready) // idle at start
	go f.a.recheckWorker()
}

// TestTriggerRecheck_WakesWorker verifies that TriggerRecheck causes the async
// worker to run RecheckTxs and evict stale txs without blocking the caller.
func TestTriggerRecheck_WakesWorker(t *testing.T) {
	f := newAsyncRecheckFixture(t, "alice-0")
	stale := f.add(1, "alice", 0, "alice-0")

	f.a.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.TriggerRecheck()

	// Poll until the worker evicts the stale tx (t.Cleanup calls Close).
	deadline := time.After(2 * time.Second)
	for poolHas(f.pool, stale) {
		select {
		case <-deadline:
			t.Fatal("timeout: async worker did not evict stale tx")
		case <-time.After(time.Millisecond):
		}
	}
}

// TestTriggerRecheck_CoalescedPreservesSenders fires TriggerRecheck many times
// rapidly. Coalescing may collapse several triggers into one run, but staging
// merges across blocks so no senders are lost.
func TestTriggerRecheck_CoalescedPreservesSenders(t *testing.T) {
	f := newAsyncRecheckFixture(t, "alice-0")
	stale := f.add(1, "alice", 0, "alice-0")
	survivor := f.add(2, "alice", 1, "alice-1")

	f.a.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.lastCommittedHeight = 2
	// Fire more triggers than the buffer; all but one are dropped by the coalescing
	// select, but the single run still sees all staged senders.
	for i := 0; i < 10; i++ {
		f.a.TriggerRecheck()
	}

	// Poll until stale tx is gone; t.Cleanup calls Close.
	deadline := time.After(2 * time.Second)
	for poolHas(f.pool, stale) {
		select {
		case <-deadline:
			t.Fatal("timeout: async worker did not evict stale tx")
		case <-time.After(time.Millisecond):
		}
	}
	if !poolHas(f.pool, survivor) {
		t.Fatal("valid tx must survive recheck")
	}
}

// TestTriggerRecheck_ConcurrentCommits exercises concurrent StageRecheckSenders
// and TriggerRecheck from multiple goroutines. Run with -race.
func TestTriggerRecheck_ConcurrentCommits(t *testing.T) {
	f := newAsyncRecheckFixture(t)
	f.add(1, "alice", 0, "alice-0")
	f.add(2, "bob", 0, "bob-0")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(height int64) {
			defer wg.Done()
			f.a.StageRecheckSenders(height, nil)
			f.a.TriggerRecheck()
		}(int64(i + 1))
	}
	wg.Wait()
	// Close drains any in-flight recheck cleanly.
	f.a.Close()
}

// TestClose_WaitsForInFlight verifies Close blocks until an in-progress
// RecheckTxs call completes, preventing store teardown races.
func TestClose_WaitsForInFlight(t *testing.T) {
	// blockingRunner delays RunTx until unblocked.
	unblock := make(chan struct{})
	var inFlight atomic.Bool
	runner := &stubRunner{
		runTx: func(_ []byte) error {
			inFlight.Store(true)
			<-unblock
			return nil
		},
	}

	f := newRecheckFixture()
	f.a.runner = runner
	startAsyncWorker(f)

	f.add(1, "alice", 0, "alice-0")
	f.a.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.TriggerRecheck()

	// Wait until the worker is actually inside RunTx.
	deadline := time.After(2 * time.Second)
	for !inFlight.Load() {
		select {
		case <-deadline:
			t.Fatal("timeout: worker never entered RunTx")
		case <-time.After(time.Millisecond):
		}
	}

	closed := make(chan struct{})
	go func() {
		f.a.Close()
		close(closed)
	}()

	// Close must not return while RunTx is still blocking.
	select {
	case <-closed:
		t.Fatal("Close returned before in-flight RecheckTxs finished")
	case <-time.After(50 * time.Millisecond):
	}

	close(unblock) // let RunTx return

	select {
	case <-closed:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: Close did not return after in-flight RecheckTxs finished")
	}
}

// TestWaitForRecheck_BlocksUntilWorkerDone verifies that WaitForRecheck blocks
// until the in-progress RecheckTxs call completes, then returns.
func TestWaitForRecheck_BlocksUntilWorkerDone(t *testing.T) {
	unblock := make(chan struct{})
	var inFlight atomic.Bool
	runner := &stubRunner{
		runTx: func(_ []byte) error {
			inFlight.Store(true)
			<-unblock
			return nil
		},
	}

	f := newRecheckFixture()
	f.a.runner = runner
	startAsyncWorker(f)
	defer f.a.Close()

	f.add(1, "alice", 0, "alice-0")
	f.a.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.TriggerRecheck()

	// Wait until the worker is actually inside RunTx.
	deadline := time.After(2 * time.Second)
	for !inFlight.Load() {
		select {
		case <-deadline:
			t.Fatal("timeout: worker never entered RunTx")
		case <-time.After(time.Millisecond):
		}
	}

	waited := make(chan struct{})
	go func() {
		f.a.WaitForRecheck(context.Background())
		close(waited)
	}()

	// WaitForRecheck must not return while RunTx is still blocking.
	select {
	case <-waited:
		t.Fatal("WaitForRecheck returned before in-flight RecheckTxs finished")
	case <-time.After(50 * time.Millisecond):
	}

	close(unblock) // let RunTx return

	select {
	case <-waited:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: WaitForRecheck did not return after RecheckTxs finished")
	}
}

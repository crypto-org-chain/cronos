package mempool

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func newAsyncRecheckFixture(t *testing.T, failBytes ...string) *recheckFixture {
	t.Helper()
	f := newRecheckFixture(failBytes...)
	startAsyncWorker(f)
	t.Cleanup(func() { f.a.Close() })
	return f
}

// startAsyncWorker does NOT register a Cleanup — caller owns Close().
func startAsyncWorker(f *recheckFixture) {
	f.a.worker.trigger = make(chan chan struct{}, 1)
	f.a.worker.quit = make(chan struct{})
	f.a.worker.done = make(chan struct{})
	f.a.worker.ready = make(chan struct{})
	close(f.a.worker.ready) // idle at start
	go f.a.worker.run(f.a.RecheckTxs)
}

func waitUntil(t *testing.T, cond func() bool, timeout time.Duration, msg string) {
	t.Helper()
	deadline := time.After(timeout)
	for !cond() {
		select {
		case <-deadline:
			t.Fatal(msg)
		case <-time.After(time.Millisecond):
		}
	}
}

func TestTriggerRecheck_WakesWorker(t *testing.T) {
	f := newAsyncRecheckFixture(t, "alice-0")
	stale := f.add(1, "alice", 0, "alice-0")

	f.a.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.TriggerRecheck()

	waitUntil(t, func() bool { return !poolHas(f.pool, stale) }, 2*time.Second,
		"timeout: async worker did not evict stale tx")
}

func TestTriggerRecheck_CoalescedPreservesSenders(t *testing.T) {
	f := newAsyncRecheckFixture(t, "alice-0")
	stale := f.add(1, "alice", 0, "alice-0")
	survivor := f.add(2, "alice", 1, "alice-1")

	f.a.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.lastCommittedHeight = 2
	// many triggers coalesce to one run; staging merges so no senders are lost.
	for i := 0; i < 10; i++ {
		f.a.TriggerRecheck()
	}

	waitUntil(t, func() bool { return !poolHas(f.pool, stale) }, 2*time.Second,
		"timeout: async worker did not evict stale tx")
	if !poolHas(f.pool, survivor) {
		t.Fatal("valid tx must survive recheck")
	}
}

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
	f.a.Close()
}

func TestClose_WaitsForInFlight(t *testing.T) {
	unblock := make(chan struct{})
	var unblockOnce sync.Once
	unblockRunner := func() { unblockOnce.Do(func() { close(unblock) }) }
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
	// unblock before Close so a failed assertion can't hang the cleanup.
	t.Cleanup(func() {
		unblockRunner()
		f.a.Close()
	})

	f.add(1, "alice", 0, "alice-0")
	f.a.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.TriggerRecheck()

	waitUntil(t, inFlight.Load, 2*time.Second, "timeout: worker never entered RunTx")

	closed := make(chan struct{})
	go func() {
		f.a.Close()
		close(closed)
	}()

	select {
	case <-closed:
		t.Fatal("Close returned before in-flight RecheckTxs finished")
	case <-time.After(50 * time.Millisecond):
	}

	unblockRunner() // let RunTx return

	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: Close did not return after in-flight RecheckTxs finished")
	}
}

func TestWaitForRecheck_BlocksUntilWorkerDone(t *testing.T) {
	unblock := make(chan struct{})
	var unblockOnce sync.Once
	unblockRunner := func() { unblockOnce.Do(func() { close(unblock) }) }
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
	// unblock before Close so a failed assertion can't hang the cleanup.
	defer func() {
		unblockRunner()
		f.a.Close()
	}()

	f.add(1, "alice", 0, "alice-0")
	f.a.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.TriggerRecheck()

	waitUntil(t, inFlight.Load, 2*time.Second, "timeout: worker never entered RunTx")

	waited := make(chan struct{})
	go func() {
		f.a.WaitForRecheck(context.Background())
		close(waited)
	}()

	select {
	case <-waited:
		t.Fatal("WaitForRecheck returned before in-flight RecheckTxs finished")
	case <-time.After(50 * time.Millisecond):
	}

	unblockRunner() // let RunTx return

	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: WaitForRecheck did not return after RecheckTxs finished")
	}
}

// TestWaitForRecheck_CtxTimeoutUnblocks guards the fix in app.go's
// PrepareProposal wrapper: cosmos-sdk's ctx carries no deadline of its own,
// so the caller must wrap it with context.WithTimeout. This proves
// WaitForRecheck actually honors that deadline instead of blocking forever
// on a stuck recheck.
func TestWaitForRecheck_CtxTimeoutUnblocks(t *testing.T) {
	unblock := make(chan struct{})
	var unblockOnce sync.Once
	unblockRunner := func() { unblockOnce.Do(func() { close(unblock) }) }
	var inFlight atomic.Bool
	runner := &stubRunner{
		runTx: func(_ []byte) error {
			inFlight.Store(true)
			<-unblock // never unblocked until cleanup: simulates a stuck recheck
			return nil
		},
	}

	f := newRecheckFixture()
	f.a.runner = runner
	startAsyncWorker(f)
	t.Cleanup(func() {
		unblockRunner()
		f.a.Close()
	})

	f.add(1, "alice", 0, "alice-0")
	f.a.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.TriggerRecheck()

	waitUntil(t, inFlight.Load, 2*time.Second, "timeout: worker never entered RunTx")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	f.a.WaitForRecheck(ctx)
	elapsed := time.Since(start)

	if ctx.Err() == nil {
		t.Fatal("expected ctx to be timed out; recheck is still stuck")
	}
	if elapsed > 1*time.Second {
		t.Fatalf("WaitForRecheck did not respect ctx deadline, blocked for %v", elapsed)
	}
}

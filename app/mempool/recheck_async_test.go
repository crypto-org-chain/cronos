package mempool

import (
	"context"
	"strconv"
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
	f.a.sched.worker = newRecheckWorker(f.a.sched.RecheckTxs)
	f.a.sched.worker.start()
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

	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.TriggerRecheck()

	waitUntil(t, func() bool { return !poolHas(f.pool, stale) }, 2*time.Second,
		"timeout: async worker did not evict stale tx")
}

func TestTriggerRecheck_CoalescedPreservesSenders(t *testing.T) {
	f := newAsyncRecheckFixture(t, "alice-0")
	stale := f.add(1, "alice", 0, "alice-0")
	survivor := f.add(2, "alice", 1, "alice-1")

	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.sched.lastCommittedHeight = 2
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
	// admit races commit + recheck through the same admission-mutex-guarded path
	// (RunTx's shared base), exercised together under -race.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			f.a.adm.admit([]byte("concurrent-" + strconv.Itoa(i)))
		}(i)
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
	f.a.exec.runner = runner
	startAsyncWorker(f)
	// unblock before Close so a failed assertion can't hang the cleanup.
	t.Cleanup(func() {
		unblockRunner()
		f.a.Close()
	})

	f.add(1, "alice", 0, "alice-0")
	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
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
	f.a.exec.runner = runner
	startAsyncWorker(f)
	// unblock before Close so a failed assertion can't hang the cleanup.
	defer func() {
		unblockRunner()
		f.a.Close()
	}()

	f.add(1, "alice", 0, "alice-0")
	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
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

// WaitForRecheck must honor a ctx deadline even when the recheck itself never returns.
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
	f.a.exec.runner = runner
	startAsyncWorker(f)
	t.Cleanup(func() {
		unblockRunner()
		f.a.Close()
	})

	f.add(1, "alice", 0, "alice-0")
	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
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

func TestWaitForRecheckTimedOut_ReturnsFalseWhenCompletedInTime(t *testing.T) {
	f := newAsyncRecheckFixture(t, "alice-0")
	stale := f.add(1, "alice", 0, "alice-0")

	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.TriggerRecheck()

	if f.a.WaitForRecheckTimedOut(context.Background(), 2*time.Second) {
		t.Fatal("expected timedOut=false; recheck completed well within the deadline")
	}
	if poolHas(f.pool, stale) {
		t.Fatal("stale tx should have been evicted by the completed recheck")
	}
}

func TestWaitForRecheckTimedOut_ReturnsTrueWhenStuck(t *testing.T) {
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
	f.a.exec.runner = runner
	startAsyncWorker(f)
	t.Cleanup(func() {
		unblockRunner()
		f.a.Close()
	})

	f.add(1, "alice", 0, "alice-0")
	f.a.sched.recheckSenders = map[string]struct{}{sdk.AccAddress("alice").String(): {}}
	f.a.TriggerRecheck()

	waitUntil(t, inFlight.Load, 2*time.Second, "timeout: worker never entered RunTx")

	if !f.a.WaitForRecheckTimedOut(context.Background(), 50*time.Millisecond) {
		t.Fatal("expected timedOut=true; recheck is still stuck")
	}
}

// A pass that aborts mid-flight (gen advanced) still returns from RunTx's
// perspective having reached no further candidates; WaitForRecheck must not
// mistake that early return for a still-in-flight recheck and hang.
func TestWaitForRecheck_ReturnsAfterGenerationAbortedPass(t *testing.T) {
	f := newAsyncRecheckFixture(t)
	f.add(1, "alice", 0, aliceSeq0Bytes)
	f.add(2, "bob", 0, "bob-0")

	// bob's group runs first (pool priority order); bump gen from inside it so
	// alice's group aborts before it starts.
	f.runner.onCall = func(txBytes []byte) {
		if string(txBytes) == "bob-0" {
			f.a.exec.gen.Add(1) // simulates a concurrent Commit landing mid-pass
		}
	}
	f.a.sched.recheckSenders = map[string]struct{}{
		sdk.AccAddress("alice").String(): {},
		sdk.AccAddress("bob").String():   {},
	}
	f.a.TriggerRecheck()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	f.a.WaitForRecheck(ctx)

	if ctx.Err() != nil {
		t.Fatal("WaitForRecheck did not return promptly after a generation-aborted pass")
	}
	if !f.runner.seen["bob-0"] {
		t.Fatal("the candidate validated before the bump must still have run")
	}
	if f.runner.seen[aliceSeq0Bytes] {
		t.Fatal("the group after the bump must have been skipped, not rechecked against a superseded base")
	}
}

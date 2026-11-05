package mempool

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// admissionGate is the RunTx mutex with Commit priority: sync.Mutex goes FIFO
// after 1ms of starvation, which made Commit wait out every queued admission.
type admissionGate struct {
	mu sync.Mutex
	// commit is non-nil from lockForCommit until its unlock, which closes the channel.
	commit atomic.Pointer[chan struct{}]
	// commits counts released Commits, so a holder can tell whether one ran since
	// it last held mu. Guarded by mu.
	commits uint64
	// waiting counts lock callers not yet holding mu; acquired counts lock acquisitions.
	waiting  atomic.Int64
	acquired atomic.Uint64
}

// commitPending reports whether a Commit is waiting for mu.
func (g *admissionGate) commitPending() bool {
	return g.commit.Load() != nil
}

// lock acquires mu unless a Commit is pending, in which case it waits for that Commit first.
func (g *admissionGate) lock() {
	g.waiting.Add(1)
	defer g.waiting.Add(-1)
	for {
		g.mu.Lock()
		ch := g.commit.Load()
		if ch == nil {
			g.acquired.Add(1)
			return
		}
		g.mu.Unlock()
		<-*ch
	}
}

// lockAfterWaiters lets the callers already queued in lock go first, for up to
// budget, then locks. sync.Mutex lets a goroutine that just unlocked re-take mu
// ahead of woken waiters until they have starved for 1ms, so a loop re-locking
// between chunks would otherwise make every admission wait out that 1ms.
func (g *admissionGate) lockAfterWaiters(budget time.Duration) {
	if n := g.waiting.Load(); n > 0 {
		target := g.acquired.Load() + uint64(n)
		deadline := time.Now().Add(budget)
		for g.waiting.Load() > 0 && g.acquired.Load() < target && time.Now().Before(deadline) {
			runtime.Gosched()
		}
	}
	g.lock()
}

func (g *admissionGate) unlock() {
	g.mu.Unlock()
}

// lockForCommit acquires mu before any queued admission runs RunTx and returns
// its release. Calls must not overlap; ABCI serializes Commit.
func (g *admissionGate) lockForCommit() (unlock func()) {
	ch := make(chan struct{})
	g.commit.Store(&ch)
	g.mu.Lock()
	return func() {
		g.commits++
		// Clear before releasing mu so woken admissions don't spin on a stale marker.
		g.commit.Store(nil)
		close(ch)
		g.mu.Unlock()
	}
}

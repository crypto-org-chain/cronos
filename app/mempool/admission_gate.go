package mempool

import (
	"sync"
	"sync/atomic"
)

// admissionGate wraps the RunTx mutex with the two fairness controls a plain
// sync.Mutex lacks: Commit acquires ahead of every queued admission, and the
// number of admissions allowed to queue is bounded.
//
// sync.Mutex hands off FIFO once a waiter has starved for 1ms, so under a
// burst Commit would otherwise wait behind every admission already queued:
// seconds at a 10^5 backlog. An admission that observes a pending Commit
// parks on its channel instead, before or right after acquiring, so Commit
// waits for at most one in-flight RunTx plus the handoff through the queue.
type admissionGate struct {
	mu sync.Mutex
	// commit is non-nil from lockForCommit until its unlock, which closes the
	// channel; parked admissions resume from it.
	commit atomic.Pointer[chan struct{}]
	// inflight counts admissions between enter and leave. maxInflight bounds it;
	// <=0 disables the bound.
	inflight    atomic.Int64
	maxInflight int64
}

// enter reserves an admission slot; false means the caller must shed the tx
// without calling leave.
func (g *admissionGate) enter() bool {
	n := g.inflight.Add(1)
	if g.maxInflight > 0 && n > g.maxInflight {
		g.inflight.Add(-1)
		return false
	}
	return true
}

func (g *admissionGate) leave() {
	g.inflight.Add(-1)
}

// lock acquires mu, parking behind a pending Commit instead of ahead of it.
func (g *admissionGate) lock() {
	for {
		if ch := g.commit.Load(); ch != nil {
			<-*ch
			continue
		}
		g.mu.Lock()
		ch := g.commit.Load()
		if ch == nil {
			return
		}
		// Commit arrived while we were queued: hand the lock on rather than
		// spending a RunTx in front of it.
		g.mu.Unlock()
		<-*ch
	}
}

func (g *admissionGate) unlock() {
	g.mu.Unlock()
}

// lockForCommit acquires mu with priority over queued admissions and returns
// its release. Callers must not overlap: consensus serializes Commit.
func (g *admissionGate) lockForCommit() (unlock func()) {
	ch := make(chan struct{})
	g.commit.Store(&ch)
	g.mu.Lock()
	return func() {
		// Clear before releasing mu so a woken admission re-checks against nil
		// and takes the lock instead of bouncing once more.
		g.commit.Store(nil)
		close(ch)
		g.mu.Unlock()
	}
}

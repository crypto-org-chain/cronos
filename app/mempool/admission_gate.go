package mempool

import (
	"sync"
	"sync/atomic"
)

// admissionGate is the RunTx mutex with Commit priority: sync.Mutex goes FIFO
// after 1ms of starvation, which made Commit wait out every queued admission.
type admissionGate struct {
	mu sync.Mutex
	// commit is non-nil from lockForCommit until its unlock, which closes the channel.
	commit atomic.Pointer[chan struct{}]
}

// lock acquires mu unless a Commit is pending, in which case it waits for that Commit first.
func (g *admissionGate) lock() {
	for {
		g.mu.Lock()
		ch := g.commit.Load()
		if ch == nil {
			return
		}
		g.mu.Unlock()
		<-*ch
	}
}

func (g *admissionGate) unlock() {
	g.mu.Unlock()
}

// lockForCommit acquires mu ahead of queued admissions and returns its release.
func (g *admissionGate) lockForCommit() (unlock func()) {
	ch := make(chan struct{})
	g.commit.Store(&ch)
	g.mu.Lock()
	return func() {
		// Clear before releasing mu so woken admissions don't spin on a stale marker;
		// CAS keeps a newer Commit's marker if calls ever overlap.
		g.commit.CompareAndSwap(&ch, nil)
		close(ch)
		g.mu.Unlock()
	}
}

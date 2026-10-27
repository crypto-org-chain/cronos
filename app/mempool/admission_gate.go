package mempool

import (
	"sync"
	"sync/atomic"
)

// admissionGate is the RunTx mutex with Commit priority. sync.Mutex goes FIFO
// once a waiter starves for 1ms, so under a burst Commit would wait behind every
// queued admission. Here an admission that gets the mutex while Commit is pending
// releases it and waits, so Commit waits for one in-flight RunTx plus a
// wake-and-yield per queued admission.
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
// Calls must not overlap; consensus serializes Commit.
func (g *admissionGate) lockForCommit() (unlock func()) {
	ch := make(chan struct{})
	g.commit.Store(&ch)
	g.mu.Lock()
	return func() {
		// Must clear before releasing mu, or woken admissions park again.
		g.commit.Store(nil)
		close(ch)
		g.mu.Unlock()
	}
}

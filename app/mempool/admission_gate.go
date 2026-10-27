package mempool

import (
	"sync"
	"sync/atomic"
)

// admissionGate is the RunTx mutex with Commit priority. sync.Mutex goes FIFO
// once a waiter starves for 1ms, so under a burst Commit would wait behind every
// queued admission; here queued admissions park on the pending Commit instead,
// so Commit waits for one in-flight RunTx plus a wake-and-yield per waiter.
type admissionGate struct {
	mu sync.Mutex
	// commit is non-nil from lockForCommit until its unlock, which closes the channel.
	commit atomic.Pointer[chan struct{}]
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
		// Commit arrived while we were queued: yield instead of running RunTx ahead of it.
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
		// Clear before close so woken admissions take the lock instead of parking again.
		g.commit.Store(nil)
		close(ch)
		g.mu.Unlock()
	}
}

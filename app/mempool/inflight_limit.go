package mempool

import "sync/atomic"

// inflightLimit caps concurrent holders; max <= 0 means unlimited.
type inflightLimit struct {
	n   atomic.Int64
	max atomic.Int64
}

// tryAcquire takes a slot; on false the caller must not call release.
func (l *inflightLimit) tryAcquire() bool {
	limit := l.max.Load()
	if limit <= 0 {
		l.n.Add(1)
		return true
	}
	for {
		n := l.n.Load()
		if n >= limit {
			return false
		}
		if l.n.CompareAndSwap(n, n+1) {
			return true
		}
	}
}

func (l *inflightLimit) release() {
	l.n.Add(-1)
}

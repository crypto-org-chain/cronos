package mempool

import (
	"context"
	"sync"
)

// recheckWorker runs RecheckTxs on one background goroutine, coalescing bursts of
// triggers into a single pending run and letting PrepareProposal wait for it.
type recheckWorker struct {
	trigger  chan chan struct{} // buffered-1: holds the pending run's ready gate
	quit     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	readyMu  sync.Mutex
	ready    chan struct{} // gate of the latest run; closed when idle
}

// init allocates channels and launches the worker goroutine.
func (w *recheckWorker) init(fn func()) {
	w.trigger = make(chan chan struct{}, 1)
	w.quit = make(chan struct{})
	w.done = make(chan struct{})
	w.ready = make(chan struct{})
	close(w.ready) // start idle: a wait with nothing pending returns at once
	go w.run(fn)
}

// recheck requests a run without blocking. Each wakeup carries its own gate rather
// than sharing one, sidestepping close races; wakeups while a run is pending coalesce
// onto it. Skipped once stopped so no gate outlives the worker.
func (w *recheckWorker) recheck() {
	ready := make(chan struct{})
	w.readyMu.Lock()
	defer w.readyMu.Unlock()
	select {
	case <-w.quit:
		return
	default:
	}
	select {
	case w.trigger <- ready:
		w.ready = ready
	default:
	}
}

// run executes queued runs one at a time. quit is re-checked before starting a run so
// stop() is never delayed by a fresh one.
func (w *recheckWorker) run(fn func()) {
	defer close(w.done)
	for {
		select {
		case <-w.quit:
			w.drainTrigger()
			return
		case ready := <-w.trigger:
			select {
			case <-w.quit:
				close(ready)
				w.drainTrigger()
				return
			default:
				fn()
				close(ready)
			}
		}
	}
}

// drainTrigger closes any pending gate so a waiter never blocks on a run that will
// never happen. Correct only on exit: recheck() stops enqueuing once quit is closed.
func (w *recheckWorker) drainTrigger() {
	for {
		select {
		case ready := <-w.trigger:
			close(ready)
		default:
			return
		}
	}
}

// stop signals the worker and waits for it to exit. Idempotent.
func (w *recheckWorker) stop() {
	if w.quit == nil {
		return
	}
	// Close under readyMu so recheck() either enqueues before it (worker still drains
	// the gate) or observes it and skips — the two can't interleave to strand a gate.
	w.stopOnce.Do(func() {
		w.readyMu.Lock()
		close(w.quit)
		w.readyMu.Unlock()
	})
	<-w.done
}

// wait reports whether ctx expired before the pending run finished. On expiry it
// re-checks the gate so a run finishing at the deadline isn't misreported as a timeout.
func (w *recheckWorker) wait(ctx context.Context) bool {
	w.readyMu.Lock()
	ready := w.ready
	w.readyMu.Unlock()
	select {
	case <-ready:
		return false
	case <-ctx.Done():
		select {
		case <-ready:
			return false
		default:
			return true
		}
	}
}

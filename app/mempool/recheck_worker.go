package mempool

import (
	"context"
	"sync"
)

// recheckWorker manages the async recheck lifecycle: coalescing triggers,
// running RecheckTxs on a single goroutine, and gating PrepareProposal.
type recheckWorker struct {
	fn       func()             // bound recheck function, invoked once per drained trigger
	trigger  chan chan struct{} // buffered-1: holds the pending run's ready gate
	quit     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	// readyMu guards ready and also serializes recheck()'s trigger-send against
	// stop()'s quit-close; narrowing it to just guard ready would reopen a race
	// where a gate gets buffered after quit closes and is never closed.
	readyMu sync.Mutex
	ready   chan struct{} // latest queued gate; pre-closed (idle) at construction
}

// newRecheckWorker builds a worker bound to fn. Call start to launch its goroutine.
func newRecheckWorker(fn func()) recheckWorker {
	ready := make(chan struct{})
	close(ready) // idle at start
	return recheckWorker{
		fn:      fn,
		trigger: make(chan chan struct{}, 1),
		quit:    make(chan struct{}),
		done:    make(chan struct{}),
		ready:   ready,
	}
}

// start launches the worker goroutine. Call once, after construction.
func (w *recheckWorker) start() {
	go w.run()
}

// recheck requests a run without blocking. Each wakeup carries its own gate rather
// than sharing one, sidestepping close races; wakeups while a run is pending coalesce
// onto it.
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
func (w *recheckWorker) run() {
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
				w.fn()
				close(ready)
			}
		}
	}
}

// drainTrigger closes any pending gate so a waiter never blocks on a run that will
// never happen.
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
	w.stopOnce.Do(func() {
		w.readyMu.Lock()
		close(w.quit)
		w.readyMu.Unlock()
	})
	<-w.done
}

// wait reports whether ctx expired before the pending run finished.
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

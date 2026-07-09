package mempool

import (
	"context"
	"sync"
)

// recheckWorker manages the async recheck lifecycle: coalescing triggers,
// running RecheckTxs on a single goroutine, and gating PrepareProposal.
type recheckWorker struct {
	fn       func()             // bound recheck function, invoked once per drained trigger
	trigger  chan chan struct{} // buffered-1 coalescing; each value is the caller's ready gate
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

// recheck coalesces an async wakeup (non-blocking); own gate per trigger avoids a shared-close race.
func (w *recheckWorker) recheck() {
	ready := make(chan struct{})
	w.readyMu.Lock()
	select {
	case w.trigger <- ready:
		w.ready = ready // track the latest gate for wait
	default:
		// coalesced: trigger already buffered; existing gate covers this wakeup
	}
	w.readyMu.Unlock()
}

// run is the worker loop — single goroutine, no concurrent rechecks.
func (w *recheckWorker) run() {
	defer close(w.done)
	for {
		select {
		case <-w.quit:
			// Drain a racing buffered trigger so its gate isn't left unclosed.
			select {
			case ready := <-w.trigger:
				close(ready)
			default:
			}
			return
		case ready := <-w.trigger:
			// Check quit again: stop() may have raced in after a trigger was already buffered.
			select {
			case <-w.quit:
				close(ready) // unblock any wait()
				return
			default:
				w.fn()
				close(ready)
			}
		}
	}
}

// stop signals the worker and waits for it to exit. Idempotent.
func (w *recheckWorker) stop() {
	if w.quit == nil {
		return
	}
	w.stopOnce.Do(func() { close(w.quit) })
	<-w.done
}

// wait blocks until the gate closes or ctx is done, returning whether ctx won.
// Rechecks ready after ctx fires so a tie isn't misreported as a timeout.
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

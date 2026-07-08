package mempool

import (
	"context"
	"sync"
)

// recheckWorker manages the async recheck lifecycle: coalescing triggers,
// running RecheckTxs on a single goroutine, and gating PrepareProposal.
type recheckWorker struct {
	trigger  chan chan struct{} // buffered-1 coalescing; each value is the caller's ready gate
	quit     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	readyMu  sync.Mutex
	ready    chan struct{} // latest queued gate; pre-closed (idle) at init
}

// init allocates channels and launches the worker goroutine.
func (w *recheckWorker) init(fn func()) {
	w.trigger = make(chan chan struct{}, 1)
	w.quit = make(chan struct{})
	w.done = make(chan struct{})
	w.ready = make(chan struct{})
	close(w.ready) // idle at start
	go w.run(fn)
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
func (w *recheckWorker) run(fn func()) {
	defer close(w.done)
	for {
		select {
		case <-w.quit:
			// drain a racing buffered trigger so its gate isn't orphaned unclosed.
			select {
			case ready := <-w.trigger:
				close(ready)
			default:
			}
			return
		case ready := <-w.trigger:
			// re-check quit: stop() may race a buffered trigger.
			select {
			case <-w.quit:
				close(ready) // unblock any wait()
				return
			default:
				fn()
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

// wait blocks until the latest queued gate closes, or ctx is done.
func (w *recheckWorker) wait(ctx context.Context) {
	w.readyMu.Lock()
	ready := w.ready
	w.readyMu.Unlock()
	select {
	case <-ready:
	case <-ctx.Done():
	}
}

package mempool

import (
	"context"
	"sync"
)

// recheckWorker is use to manage the recheck workflow.
type recheckWorker struct {
	trigger  chan chan struct{} // holds the pending run's ready gate
	quit     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
	readyMu  sync.Mutex
	ready    chan struct{}
}

// init allocates channels and launches the worker goroutine.
func (w *recheckWorker) init(fn func()) {
	w.trigger = make(chan chan struct{}, 1)
	w.quit = make(chan struct{})
	w.done = make(chan struct{})
	w.ready = make(chan struct{})
	close(w.ready)
	go w.run(fn)
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

// run executes the recheck workflow.
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

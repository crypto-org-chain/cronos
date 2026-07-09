package mempool

import (
	"context"
	"testing"
	"time"
)

// quit racing an already-buffered trigger must still close that gate.
func TestRun_QuitRaceClosesBufferedGate(t *testing.T) {
	w := &recheckWorker{
		trigger: make(chan chan struct{}, 1),
		quit:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	ready := make(chan struct{})
	w.trigger <- ready
	close(w.quit)

	var fnCalled bool
	w.run(func() { fnCalled = true })

	select {
	case <-ready:
	default:
		t.Fatal("buffered gate was not closed when quit raced a pending trigger")
	}
	if fnCalled {
		t.Fatal("fn must not run once quit has been signaled")
	}
	select {
	case <-w.done:
	default:
		t.Fatal("done was not closed on return")
	}
}

func TestStop_Idempotent(t *testing.T) {
	w := &recheckWorker{}
	w.init(func() {})
	w.stop()
	w.stop() // must not panic or hang
}

// recheck() after stop() must not strand a gate in trigger with no worker to close it,
// which would make every later wait() block the full timeout.
func TestRecheckAfterStop_DoesNotStrandGate(t *testing.T) {
	w := &recheckWorker{}
	w.init(func() {})
	w.stop()

	w.recheck() // worker is gone; this must be a no-op, not a buffered orphan gate

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if w.wait(ctx) {
		t.Fatal("wait timed out: recheck() after stop stranded a gate with no worker to close it")
	}
}

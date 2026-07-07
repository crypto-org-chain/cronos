package mempool

import "testing"

// TestRun_QuitRaceClosesBufferedGate covers the outer select in run(): when
// stop() closes quit while a trigger is already buffered, Go's select may
// pick either case. Both must end with the buffered gate closed — otherwise
// a wait() blocked on it (e.g. WaitForRecheck in PrepareProposal) hangs
// forever after the worker has already exited.
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

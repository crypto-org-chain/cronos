package mempool

import "testing"

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

package keeper

import "sync/atomic"

var ReplayBlockSemaphore = replayBlockSem

const ReplayBlockConcurrencyLimit = replayBlockConcurrency

const ReplayBlockMaxQueued = replayBlockMaxQueued

// SetReplayBlockQueued sets the waiter counter directly.
func SetReplayBlockQueued(n int32) {
	atomic.StoreInt32(&replayBlockQueued, n)
}

// ResetReplayBlockQueued clears the waiter counter.
func ResetReplayBlockQueued() {
	SetReplayBlockQueued(0)
}

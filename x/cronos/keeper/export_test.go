package keeper

import "sync/atomic"

// Exposed for the external keeper_test package to exercise the ReplayBlock
// concurrency guard without a broader refactor of the Keeper.
var ReplayBlockSemaphore = replayBlockSem

// ReplayBlockConcurrencyLimit exposes the configured ReplayBlock concurrency
// bound for the external keeper_test package.
const ReplayBlockConcurrencyLimit = replayBlockConcurrency

// ReplayBlockMaxQueued exposes the configured ReplayBlock waiter bound for
// the external keeper_test package.
const ReplayBlockMaxQueued = replayBlockMaxQueued

// SetReplayBlockQueued sets the ReplayBlock waiter counter directly, so
// tests can exercise the queue bound without spinning up real concurrent
// callers.
func SetReplayBlockQueued(n int32) {
	atomic.StoreInt32(&replayBlockQueued, n)
}

// ResetReplayBlockQueued clears the ReplayBlock waiter counter, so tests
// don't interfere with each other when run in the same process.
func ResetReplayBlockQueued() {
	SetReplayBlockQueued(0)
}

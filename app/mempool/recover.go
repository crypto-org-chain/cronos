package mempool

import (
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// stackLogInterval caps stack traces per path; the one-line error still logs every panic.
const stackLogInterval = time.Minute

// errRecoveredPanic stays below abci.CodeTypeRetry so CometBFT doesn't soon re-admit the tx, and hides the panic value from clients.
var errRecoveredPanic = sdkerrors.ErrInvalidRequest.Wrap("recovered panic")

var (
	// stackLogStart makes stack timestamps monotonic, so a wall-clock step can't suppress them.
	stackLogStart = time.Now()
	// lastStackLog maps a path to its last logged stack, in nanos since stackLogStart.
	lastStackLog sync.Map
)

// logRecoveredPanic records a panic that CometBFT would turn into a hung RPC, stopped gossip or dropped peer.
func logRecoveredPanic(logger log.Logger, path string, r any) {
	keyvals := []any{"path", path, "panic", r}
	if shouldLogStack(path, int64(time.Since(stackLogStart))) {
		keyvals = append(keyvals, "stack", string(debug.Stack()))
	}
	logger.Error("recovered panic in app mempool", keyvals...)
	telemetry.IncrCounter(1, "cronos", "mempool", "panic", path)
}

// recoveredPanic logs a recovered panic and returns errRecoveredPanic.
func recoveredPanic(logger log.Logger, path string, r any) error {
	logRecoveredPanic(logger, path, r)
	return errRecoveredPanic
}

// shouldLogStack lets one caller per stackLogInterval log a stack for path; CAS losers get false.
func shouldLogStack(path string, now int64) bool {
	v, _ := lastStackLog.LoadOrStore(path, new(atomic.Int64))
	last, ok := v.(*atomic.Int64)
	if !ok {
		return true
	}
	prev := last.Load()
	if prev != 0 && now-prev < int64(stackLogInterval) {
		return false
	}
	return last.CompareAndSwap(prev, now)
}

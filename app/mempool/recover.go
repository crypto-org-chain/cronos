package mempool

import (
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	tmtypes "github.com/cometbft/cometbft/types"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Each path also names its cronos_mempool_panic_<path> counter.
const (
	panicPathAdmit     = "admit"
	panicPathCheckTx   = "check_tx"
	panicPathReap      = "reap"       // whole reap dropped
	panicPathReapTx    = "reap_tx"    // one tx skipped
	panicPathRecheck   = "recheck"    // cycle aborted, its senders retried next cycle
	panicPathRecheckTx = "recheck_tx" // one tx evicted
)

// Only stacks are rate-limited; every panic still logs a line.
const stackLogInterval = time.Minute

// Panic values can embed tx contents.
const maxPanicLogLen = 1024

// Below abci.CodeTypeRetry so CometBFT won't soon re-admit the tx; hides the panic value from clients.
var errRecoveredPanic = sdkerrors.ErrInvalidRequest.Wrap("recovered panic")

var (
	// Monotonic base, so a wall-clock step can't suppress stacks.
	stackLogStart = time.Now()
	// path -> last stack time, in nanos since stackLogStart.
	lastStackLog sync.Map
)

func logRecoveredPanic(logger log.Logger, path string, r any, keyvals ...any) {
	value := fmt.Sprint(r)
	if len(value) > maxPanicLogLen {
		value = value[:maxPanicLogLen] + "...(truncated)"
	}
	keyvals = append([]any{"path", path, "panic", value}, keyvals...)
	if shouldLogStack(path, int64(time.Since(stackLogStart))) {
		keyvals = append(keyvals, "stack", string(debug.Stack()))
	}
	logger.Error("recovered panic in app mempool", keyvals...)
	telemetry.IncrCounter(1, "cronos", "mempool", "panic", path)
}

func recoveredPanic(logger log.Logger, path string, r any, keyvals ...any) error {
	logRecoveredPanic(logger, path, r, keyvals...)
	return errRecoveredPanic
}

func txHash(bz []byte) string {
	return fmt.Sprintf("%X", tmtypes.Tx(bz).Hash())
}

// Only the CAS winner logs, so concurrent panics log one stack per interval.
func shouldLogStack(path string, now int64) bool {
	v, _ := lastStackLog.LoadOrStore(path, new(atomic.Int64))
	last := v.(*atomic.Int64)
	prev := last.Load()
	if prev != 0 && now-prev < int64(stackLogInterval) {
		return false
	}
	return last.CompareAndSwap(prev, now)
}

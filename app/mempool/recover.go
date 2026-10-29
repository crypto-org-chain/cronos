package mempool

import (
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// stackLogInterval bounds full stack traces per path, so a panic an attacker
// can repeat with crafted txs can't flood the log.
const stackLogInterval = time.Minute

// lastStackLog maps a recovery path to the unix nanos of its last logged stack.
var lastStackLog sync.Map

// logRecoveredPanic logs and counts a panic recovered on an app-mempool path.
// CometBFT's own recovers on these paths only log: a CheckTx panic leaves the
// RPC caller waiting, a ReapTxs panic ends gossip until restart, and a peer
// InsertTx panic drops the peer.
func logRecoveredPanic(logger log.Logger, path string, r any) {
	keyvals := []any{"path", path, "panic", r}
	if shouldLogStack(path, time.Now().UnixNano()) {
		keyvals = append(keyvals, "stack", string(debug.Stack()))
	}
	logger.Error("recovered panic in app mempool", keyvals...)
	telemetry.IncrCounter(1, "cronos", "mempool", "panic", path)
}

// recoveredPanic logs a recovered panic and returns it as ErrPanic.
func recoveredPanic(logger log.Logger, path string, r any) error {
	logRecoveredPanic(logger, path, r)
	return errorsmod.Wrap(sdkerrors.ErrPanic, fmt.Sprint(r))
}

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

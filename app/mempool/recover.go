package mempool

import (
	"fmt"
	"runtime/debug"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// recoveredPanic logs and counts a panic recovered on an app-mempool ABCI path
// and returns it as ErrPanic. CometBFT doesn't recover these without
// cometbft#72: a CheckTx panic hangs the RPC broadcast and a ReapTxs panic
// stops gossip until restart.
func recoveredPanic(logger log.Logger, path string, r any) error {
	logger.Error("recovered panic in app mempool", "path", path, "panic", r, "stack", string(debug.Stack()))
	telemetry.IncrCounter(1, "cronos", "mempool", "panic", path)
	return errorsmod.Wrap(sdkerrors.ErrPanic, fmt.Sprint(r))
}

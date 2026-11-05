package mempool

import (
	"context"
	"errors"

	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

var (
	_ sdkmempool.ExtMempool       = FinalizeNeutralMempool{}
	_ sdkmempool.UnorderedMempool = FinalizeNeutralMempool{}
)

// FinalizeNeutralMempool keeps a block tx's result independent of the local pool. runTx fails the
// tx on any finalize-mode removal error but ErrTxNotFound, and NoOp-mempool nodes never fail, so
// any other outcome here would make results depend on mempool config.
type FinalizeNeutralMempool struct {
	*sdkmempool.PriorityNonceMempool[int64]
	logger log.Logger
}

func NewFinalizeNeutralMempool(pool *sdkmempool.PriorityNonceMempool[int64], logger log.Logger) FinalizeNeutralMempool {
	if logger == nil { // a nil logger would re-panic inside the recover handler
		logger = log.NewNopLogger()
	}
	return FinalizeNeutralMempool{PriorityNonceMempool: pool, logger: logger}
}

func (p FinalizeNeutralMempool) RemoveWithReason(ctx context.Context, tx sdk.Tx, reason sdkmempool.RemoveReason) error {
	if reason.Caller != sdkmempool.CallerRunTxFinalize {
		return p.PriorityNonceMempool.RemoveWithReason(ctx, tx, reason)
	}
	defer func() {
		if r := recover(); r != nil {
			logRecoveredPanic(p.logger, panicPathFinalizeRemove, r)
		}
	}()
	// A pooled entry left behind is stale (ante consumed its nonce), so recheck evicts it.
	if removeErr := p.Remove(tx); removeErr != nil && !errors.Is(removeErr, sdkmempool.ErrTxNotFound) {
		p.logger.Debug("block tx not removed from mempool", "err", removeErr)
	}
	return nil
}

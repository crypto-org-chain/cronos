package mempool

import (
	"context"
	"errors"

	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

var _ sdkmempool.ExtMempool = FinalizeNeutralMempool{}

// FinalizeNeutralMempool keeps a block tx's result independent of the local pool. runTx fails the
// tx on any finalize-mode removal error but ErrTxNotFound, and NoOp-mempool nodes never fail, so
// any other outcome here would make results depend on mempool config.
type FinalizeNeutralMempool struct {
	*sdkmempool.PriorityNonceMempool[int64]
	logger log.Logger
}

func NewFinalizeNeutralMempool(pool *sdkmempool.PriorityNonceMempool[int64], logger log.Logger) FinalizeNeutralMempool {
	return FinalizeNeutralMempool{PriorityNonceMempool: pool, logger: logger}
}

func (p FinalizeNeutralMempool) RemoveWithReason(ctx context.Context, tx sdk.Tx, reason sdkmempool.RemoveReason) (err error) {
	if reason.Caller != sdkmempool.CallerRunTxFinalize {
		return p.PriorityNonceMempool.RemoveWithReason(ctx, tx, reason)
	}
	defer func() {
		if r := recover(); r != nil {
			logRecoveredPanic(p.logger, panicPathFinalizeRemove, r)
			err = nil
		}
	}()
	// A pooled entry left behind is stale (ante consumed its nonce), so recheck evicts it.
	if err := p.Remove(tx); err != nil && !errors.Is(err, sdkmempool.ErrTxNotFound) {
		p.logger.Debug("block tx not removed from mempool", "err", err)
	}
	return nil
}

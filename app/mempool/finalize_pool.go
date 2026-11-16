package mempool

import (
	"context"
	"errors"
	"sync"

	evmtypes "github.com/evmos/ethermint/x/evm/types"

	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

var (
	_ sdkmempool.ExtMempool       = FinalizeNeutralMempool{}
	_ sdkmempool.UnorderedMempool = FinalizeNeutralMempool{}
)

// FinalizeNeutralMempool keeps a block tx's result independent of the local pool. runTx fails the
// tx on any finalize-mode removal error but ErrTxNotFound, and NoOp-mempool nodes never fail, so
// any other outcome here would make results depend on mempool config.
//
// It also rejects an exact resend of a pooled eth tx with ErrTxInMempoolCache instead of treating
// it as a replacement, so RPC clients can tell a resend from a failed submission.
type FinalizeNeutralMempool struct {
	*sdkmempool.PriorityNonceMempool[int64]
	logger log.Logger
	resend *resendDetector
}

// resendDetector carries the replacement callback's verdict out of PriorityNonceMempool.Insert,
// which reports a rejected replacement only as an untyped error.
type resendDetector struct {
	mu   sync.Mutex
	seen bool
}

// NewFinalizeNeutralMempool builds the priority pool from cfg, wrapping cfg.TxReplacement so an
// exact resend never reaches it.
func NewFinalizeNeutralMempool(cfg sdkmempool.PriorityNonceMempoolConfig[int64], logger log.Logger) FinalizeNeutralMempool {
	if logger == nil { // a nil logger would re-panic inside the recover handler
		logger = log.NewNopLogger()
	}
	resend := &resendDetector{}
	replace := cfg.TxReplacement
	cfg.TxReplacement = func(op, np int64, oTx, nTx sdk.Tx) bool {
		if sameEthTxs(oTx, nTx) {
			resend.seen = true
			return false
		}
		return replace == nil || replace(op, np, oTx, nTx)
	}
	return FinalizeNeutralMempool{
		PriorityNonceMempool: sdkmempool.NewPriorityMempool(cfg),
		logger:               logger,
		resend:               resend,
	}
}

// Insert returns ErrTxInMempoolCache when tx is an exact resend of a pooled eth tx.
func (p FinalizeNeutralMempool) Insert(ctx context.Context, tx sdk.Tx) error {
	p.resend.mu.Lock()
	defer p.resend.mu.Unlock()
	p.resend.seen = false
	err := p.PriorityNonceMempool.Insert(ctx, tx)
	if p.resend.seen {
		return sdkerrors.ErrTxInMempoolCache
	}
	return err
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
	// A pooled entry left behind is stale (ante consumed its nonce); recheck, when enabled, evicts it.
	if removeErr := p.Remove(tx); removeErr != nil && !errors.Is(removeErr, sdkmempool.ErrTxNotFound) {
		p.logger.Debug("block tx not removed from mempool", "err", removeErr)
	}
	return nil
}

// sameEthTxs reports whether a and b carry the same signed eth txs; the eth hash covers the
// signature, so equal hashes mean an exact resend. Txs with any non-eth message never match.
func sameEthTxs(a, b sdk.Tx) bool {
	am, bm := a.GetMsgs(), b.GetMsgs()
	if len(am) == 0 || len(am) != len(bm) {
		return false
	}
	for i := range am {
		ae, ok := am[i].(*evmtypes.MsgEthereumTx)
		if !ok {
			return false
		}
		be, ok := bm[i].(*evmtypes.MsgEthereumTx)
		if !ok {
			return false
		}
		at, bt := ae.AsTransaction(), be.AsTransaction()
		if at == nil || bt == nil || at.Hash() != bt.Hash() {
			return false
		}
	}
	return true
}

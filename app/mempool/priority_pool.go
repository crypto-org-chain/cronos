package mempool

import (
	"context"
	"errors"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

var (
	_ sdkmempool.ExtMempool       = (*PriorityMempool)(nil)
	_ sdkmempool.UnorderedMempool = (*PriorityMempool)(nil)
	_ sdkmempool.TxLookupMempool  = (*PriorityMempool)(nil)
)

// PriorityMempool is the SDK priority-nonce mempool with two adjustments:
//   - Insert reports an exact resend of a pooled eth tx as ErrTxInMempoolCache, so RPC clients can
//     tell a resend from a failed submission.
//   - Removing a block tx during finalize never fails; see removeBlockTx.
type PriorityMempool struct {
	*sdkmempool.PriorityNonceMempool[int64]
	logger log.Logger

	// insertMu serializes Insert so resendSeen, set from the TxReplacement callback, belongs to
	// the current call: the SDK reports a rejected replacement only as an untyped error.
	insertMu   sync.Mutex
	resendSeen bool
}

// NewPriorityMempool returns a PriorityMempool built from cfg.
func NewPriorityMempool(cfg sdkmempool.PriorityNonceMempoolConfig[int64], logger log.Logger) *PriorityMempool {
	if logger == nil { // a nil logger would re-panic inside the recover handler
		logger = log.NewNopLogger()
	}
	p := &PriorityMempool{logger: logger}
	cfg.TxReplacement = p.rejectResend(cfg.TxReplacement)
	p.PriorityNonceMempool = sdkmempool.NewPriorityMempool(cfg)
	return p
}

func (p *PriorityMempool) rejectResend(replace func(op, np int64, oTx, nTx sdk.Tx) bool) func(op, np int64, oTx, nTx sdk.Tx) bool {
	return func(op, np int64, oTx, nTx sdk.Tx) bool {
		if sameEthTxs(oTx, nTx) {
			p.resendSeen = true
			return false
		}
		return replace == nil || replace(op, np, oTx, nTx)
	}
}

func (p *PriorityMempool) Insert(ctx context.Context, tx sdk.Tx) error {
	p.insertMu.Lock()
	defer p.insertMu.Unlock()
	p.resendSeen = false
	err := p.PriorityNonceMempool.Insert(ctx, tx)
	// A full pool still runs TxReplacement for a pooled sender-nonce, so resendSeen covers it too.
	if p.resendSeen {
		return sdkerrors.ErrTxInMempoolCache
	}
	return err
}

func (p *PriorityMempool) RemoveWithReason(ctx context.Context, tx sdk.Tx, reason sdkmempool.RemoveReason) error {
	if reason.Caller == sdkmempool.CallerRunTxFinalize {
		p.removeBlockTx(tx)
		return nil
	}
	return p.PriorityNonceMempool.RemoveWithReason(ctx, tx, reason)
}

// removeBlockTx swallows every error and panic: runTx fails a block tx on any finalize-mode
// removal error but ErrTxNotFound, and NoOp-mempool nodes never fail, so surfacing one would make
// block results depend on local mempool config.
func (p *PriorityMempool) removeBlockTx(tx sdk.Tx) {
	defer func() {
		if r := recover(); r != nil {
			logRecoveredPanic(p.logger, panicPathFinalizeRemove, r)
		}
	}()
	// A pooled entry left behind is stale (ante consumed its nonce); recheck, when enabled, evicts it.
	if err := p.Remove(tx); err != nil && !errors.Is(err, sdkmempool.ErrTxNotFound) {
		p.logger.Debug("block tx not removed from mempool", "err", err)
	}
}

// The eth hash covers the signature, so equal hashes mean an exact resend.
func sameEthTxs(a, b sdk.Tx) bool {
	aMsgs, bMsgs := a.GetMsgs(), b.GetMsgs()
	if len(aMsgs) == 0 || len(aMsgs) != len(bMsgs) {
		return false
	}
	for i := range aMsgs {
		aHash, ok := ethTxHash(aMsgs[i])
		if !ok {
			return false
		}
		if bHash, ok := ethTxHash(bMsgs[i]); !ok || aHash != bHash {
			return false
		}
	}
	return true
}

func ethTxHash(msg sdk.Msg) (common.Hash, bool) {
	ethMsg, ok := msg.(*evmtypes.MsgEthereumTx)
	if !ok {
		return common.Hash{}, false
	}
	ethTx := ethMsg.AsTransaction()
	if ethTx == nil {
		return common.Hash{}, false
	}
	return ethTx.Hash(), true
}

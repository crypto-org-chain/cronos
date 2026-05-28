package mempool

import (
	"context"

	abci "github.com/cometbft/cometbft/abci/types"

	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

// TypeApp matches CometBFT's MempoolTypeApp ("app") config value. Mirrored
// here to avoid pulling cometbft/config into app/app.go just for one string.
const TypeApp = "app"

// codeRejectDecodeFail is returned from InsertTx for txs that fail to decode.
// ABCI semantics: any code in (0, CodeTypeRetry) is a permanent reject; the
// peer should drop the tx and not retry.
const codeRejectDecodeFail uint32 = 1

// NewReapTxsHandler returns a sdk.ReapTxsHandler that drains the
// priority-ordered mempool until the byte or gas hint passed by the
// CometBFT AppReactor is reached. A hint value of 0 is treated as
// "no cap" per CometBFT convention. Used when mempool.type=app.
//
// Encoder errors are logged but do not abort the reap; the offending tx
// is skipped so the rest of the snapshot can still ship.
func NewReapTxsHandler(mpool mempool.Mempool, txEncoder sdk.TxEncoder, logger log.Logger) sdk.ReapTxsHandler {
	return func(req *abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
		// Pre-size the snapshot to the current pool count to avoid
		// repeated slice growth under the pool lock.
		snapshot := make([]sdk.Tx, 0, mpool.CountTx())
		mempool.SelectBy(context.Background(), mpool, nil, func(tx sdk.Tx) bool {
			snapshot = append(snapshot, tx)
			return true
		})

		var (
			txs        = make([][]byte, 0, len(snapshot))
			totalBytes uint64
			totalGas   uint64
		)
		for _, tx := range snapshot {
			bz, err := txEncoder(tx)
			if err != nil {
				logger.Error("reap encode failed; skipping tx", "err", err)
				continue
			}
			size := uint64(len(bz))
			if req.MaxBytes > 0 && totalBytes+size > req.MaxBytes {
				break
			}
			var gas uint64
			if feeTx, ok := tx.(sdk.FeeTx); ok {
				gas = feeTx.GetGas()
			}
			if req.MaxGas > 0 && totalGas+gas > req.MaxGas {
				break
			}
			txs = append(txs, bz)
			totalBytes += size
			totalGas += gas
		}
		return &abci.ResponseReapTxs{Txs: txs}, nil
	}
}

// CheckTxRunner is the subset of *baseapp.BaseApp that NewInsertTxHandler
// needs. Defined as an interface so tests can stub it without spinning up
// a full BaseApp.
type CheckTxRunner interface {
	CheckTx(*abci.RequestCheckTx) (*abci.ResponseCheckTx, error)
}

// NewInsertTxHandler returns a sdk.InsertTxHandler that routes peer-relayed
// txs through BaseApp.CheckTx so AnteHandler validation runs before
// mempool admission. CheckTx itself calls mempool.Insert on success
// (RunTx execModeCheck), so the handler does not insert separately.
//
// Code mapping follows ABCI semantics:
//   - 0 (CodeTypeOK)   accepted
//   - >= CodeTypeRetry retryable (e.g. mempool full)
//   - other non-zero   permanent reject (decode/AnteHandler failure)
//
// This is the cronos-side stopgap for the cosmos-sdk v0.54 gap where the
// default InsertTx hook does not run AnteHandler. Once the SDK ships a
// patch that wires AnteHandler into the default InsertTx path, drop this
// wrapper and rely on the SDK default.
func NewInsertTxHandler(app CheckTxRunner) sdk.InsertTxHandler {
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		res, err := app.CheckTx(&abci.RequestCheckTx{
			Tx:   req.Tx,
			Type: abci.CheckTxType_New,
		})
		if err != nil {
			return &abci.ResponseInsertTx{Code: codeRejectDecodeFail}, nil
		}
		switch {
		case res.Code == abci.CodeTypeOK:
			return &abci.ResponseInsertTx{Code: abci.CodeTypeOK}, nil
		case res.Codespace == sdkerrors.RootCodespace && res.Code == sdkerrors.ErrMempoolIsFull.ABCICode():
			return &abci.ResponseInsertTx{Code: abci.CodeTypeRetry}, nil
		default:
			return &abci.ResponseInsertTx{Code: res.Code}, nil
		}
	}
}

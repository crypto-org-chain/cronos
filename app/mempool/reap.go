package mempool

import (
	"context"

	abci "github.com/cometbft/cometbft/abci/types"

	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
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

// NewInsertTxHandler returns a sdk.InsertTxHandler that decodes the tx
// and inserts it into the mempool. Code mapping follows ABCI semantics:
//   - 0 (CodeTypeOK)   accepted
//   - 1                permanent reject (decode failure)
//   - >= CodeTypeRetry retryable (insert failure, e.g. capacity)
//
// SECURITY: this path does NOT run AnteHandler — peer-relayed txs enter
// the mempool without sig/fee/nonce validation. MaxTx caps memory, and
// PrepareProposal-time validateTx still filters block-listed addresses,
// but the priority mempool will hold unverified txs until block commit.
// Until cosmos-sdk wires AnteHandler into InsertTx, mempool.type=app is
// not equivalent to mempool.type=flood w.r.t. peer trust.
func NewInsertTxHandler(mpool mempool.Mempool, txDecoder sdk.TxDecoder) sdk.InsertTxHandler {
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		tx, err := txDecoder(req.Tx)
		if err != nil {
			return &abci.ResponseInsertTx{Code: codeRejectDecodeFail}, nil
		}
		ctx := sdk.Context{}.WithContext(context.Background()).WithPriority(0)
		if err := mpool.Insert(ctx, tx); err != nil {
			return &abci.ResponseInsertTx{Code: abci.CodeTypeRetry}, nil
		}
		return &abci.ResponseInsertTx{Code: abci.CodeTypeOK}, nil
	}
}

package mempool

import (
	"context"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

// TypeApp matches CometBFT's MempoolTypeApp ("app") config value. Mirrored
// here to avoid pulling cometbft/config into app/app.go just for one string.
const TypeApp = "app"

// NewReapTxsHandler returns a sdk.ReapTxsHandler that drains the
// priority-ordered mempool until the byte or gas hint passed by the
// CometBFT AppReactor is reached. A hint value of 0 is treated as
// "no cap" per CometBFT convention. Used when mempool.type=app.
func NewReapTxsHandler(mpool mempool.Mempool, txEncoder sdk.TxEncoder) sdk.ReapTxsHandler {
	return func(req *abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
		var (
			txs        [][]byte
			totalBytes uint64
			totalGas   uint64
		)
		ctx := context.Background()
		for it := mpool.Select(ctx, nil); it != nil; it = it.Next() {
			tx := it.Tx()
			if tx == nil {
				break
			}
			bz, err := txEncoder(tx)
			if err != nil {
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
// The PriorityNonceMempool's default tx-priority extractor calls
// sdk.UnwrapSDKContext(ctx).Priority(), which panics on a plain
// context.Background. We therefore pass a zero-value sdk.Context with
// Priority=0; CheckTx-based priority lands separately via the AppMempool
// CheckTx flow.
func NewInsertTxHandler(mpool mempool.Mempool, txDecoder sdk.TxDecoder) sdk.InsertTxHandler {
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		tx, err := txDecoder(req.Tx)
		if err != nil {
			return &abci.ResponseInsertTx{Code: 1}, nil
		}
		ctx := sdk.Context{}.WithPriority(0)
		if err := mpool.Insert(ctx, tx); err != nil {
			return &abci.ResponseInsertTx{Code: abci.CodeTypeRetry}, nil
		}
		return &abci.ResponseInsertTx{Code: abci.CodeTypeOK}, nil
	}
}

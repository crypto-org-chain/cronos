package mempool

import (
	"context"

	abci "github.com/cometbft/cometbft/abci/types"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

// TypeApp mirrors CometBFT's MempoolTypeApp ("app") config value, avoiding a
// cometbft/config import for one string.
const TypeApp = "app"

// NewReapTxsHandler drains the priority mempool for mempool.type=app, stopping
// at MaxBytes/MaxGas (0 = no cap, per CometBFT convention). Prefix scan: breaks
// at the first tx over a cap (not best-fit), so a large high-priority tx may
// leave budget unused. Encoder errors skip the tx; encCache hits skip proto.Marshal.
func NewReapTxsHandler(mpool mempool.Mempool, txEncoder sdk.TxEncoder, encCache *EncoderCache, logger log.Logger) sdk.ReapTxsHandler {
	return func(req *abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
		// Pre-size to pool count to avoid slice growth under the pool lock.
		snapshot := make([]sdk.Tx, 0, mpool.CountTx())
		mempool.SelectBy(context.Background(), mpool, nil, func(tx sdk.Tx) bool {
			snapshot = append(snapshot, tx)
			return true
		})

		var (
			txs        = make([][]byte, 0, len(snapshot))
			totalBytes uint64
			totalGas   uint64
			cacheHits  float32
			cacheMiss  float32
		)
		for _, tx := range snapshot {
			bz, ok := encCache.Bytes(tx)
			if !ok {
				cacheMiss++
				var err error
				bz, err = txEncoder(tx)
				if err != nil {
					logger.Error("reap encode failed; skipping tx", "err", err)
					continue
				}
			} else {
				cacheHits++
			}
			size := uint64(ProtoSizeForTx(bz))
			if req.MaxBytes > 0 && totalBytes+size > req.MaxBytes {
				break
			}
			var gas uint64
			if feeTx, ok := tx.(sdk.FeeTx); ok {
				gas = feeTx.GetGas()
			}
			// Overflow-safe: totalGas <= req.MaxGas by induction, so
			// MaxGas-totalGas can't underflow even for attacker-set gas.
			if req.MaxGas > 0 && gas > req.MaxGas-totalGas {
				break
			}
			txs = append(txs, bz)
			totalBytes += size
			totalGas += gas
		}
		// Emit cache hit/miss once per reap (not per tx) so operators can watch
		// the proto.Marshal fallback rate climb when pool depth exceeds the
		// encoder-cache size. No-op unless telemetry is enabled.
		if cacheHits > 0 {
			telemetry.IncrCounter(cacheHits, "cronos", "mempool", "reap", "encode_cache", "hit") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
		}
		if cacheMiss > 0 {
			telemetry.IncrCounter(cacheMiss, "cronos", "mempool", "reap", "encode_cache", "miss") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
		}
		return &abci.ResponseReapTxs{Txs: txs}, nil
	}
}

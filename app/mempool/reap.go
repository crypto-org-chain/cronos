package mempool

import (
	"context"

	abci "github.com/cometbft/cometbft/abci/types"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/telemetry"
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
//
// Caps are applied as a prefix scan: the loop breaks at the first tx
// that exceeds either MaxBytes or MaxGas, not the best-fit later in
// the snapshot. This matches the SDK's default proposal handler
// behavior and avoids O(n^2) scanning, but means a large high-priority
// tx early in the snapshot can leave budget headroom unused even if
// smaller txs further down would fit.
//
// If encCache is non-nil, it is consulted before calling txEncoder so
// that txs inserted via InsertTxHandler skip proto.Marshal entirely on
// the reap hot path.
//
// Encoder errors are logged but do not abort the reap; the offending tx
// is skipped so the rest of the snapshot can still ship.
func NewReapTxsHandler(mpool mempool.Mempool, txEncoder sdk.TxEncoder, encCache *EncoderCache, logger log.Logger) sdk.ReapTxsHandler {
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
			size := uint64(protoSizeForTx(bz))
			if req.MaxBytes > 0 && totalBytes+size > req.MaxBytes {
				break
			}
			var gas uint64
			if feeTx, ok := tx.(sdk.FeeTx); ok {
				gas = feeTx.GetGas()
			}
			// Overflow-safe: gas (attacker-controlled) may be near MaxUint64;
			// totalGas <= req.MaxGas by induction, so the subtraction can't underflow.
			if req.MaxGas > 0 && gas > req.MaxGas-totalGas {
				break
			}
			txs = append(txs, bz)
			totalBytes += size
			totalGas += gas
		}
		// Emit encoder-cache hit/miss once per reap (not per tx) so operators can
		// watch the fallback-to-proto.Marshal rate climb when sustained pool depth
		// exceeds the encoder-cache size. No-op unless telemetry is enabled.
		if cacheHits > 0 {
			telemetry.IncrCounter(cacheHits, "cronos", "mempool", "reap", "encode_cache", "hit") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
		}
		if cacheMiss > 0 {
			telemetry.IncrCounter(cacheMiss, "cronos", "mempool", "reap", "encode_cache", "miss") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
		}
		return &abci.ResponseReapTxs{Txs: txs}, nil
	}
}

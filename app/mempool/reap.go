package mempool

import (
	"context"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"

	"cosmossdk.io/log/v2"

	metrics "github.com/hashicorp/go-metrics"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

const TypeApp = "app"

// NewReapTxsHandler scans the app mempool to gather txs to be gossiped to other peers,
// stopping at MaxBytes/MaxGas (0 = no cap, per CometBFT convention). Prefix scan: breaks
// at the first tx over a cap (not best-fit), so a large high-priority tx may
// leave budget unused. Encoder errors skip the tx; encCache hits skip proto.Marshal.
func NewReapTxsHandler(mpool mempool.Mempool, txEncoder sdk.TxEncoder, encCache *EncoderCache, ttl time.Duration, maxPerReap int, logger log.Logger) sdk.ReapTxsHandler {
	tracker := newGossipTracker(ttl)
	return func(req *abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
		snapshot := PoolSnapshot(context.Background(), mpool)

		now := time.Now().UnixNano()
		var (
			txs        = make([][]byte, 0, len(snapshot))
			totalBytes uint64
			totalGas   uint64
			cacheHits  float32
			cacheMiss  float32
			deduped    float32
		)
		for _, tx := range snapshot {
			bz, hit, err := EncodeTx(encCache, txEncoder, tx)
			if hit {
				cacheHits++
			} else {
				cacheMiss++
			}
			if err != nil {
				logger.Error("reap encode failed; skipping tx", "err", err)
				continue
			}
			size := uint64(ProtoSizeForTx(bz))
			if req.MaxBytes > 0 && totalBytes+size > req.MaxBytes {
				break
			}
			var gas uint64
			if feeTx, ok := tx.(sdk.FeeTx); ok {
				gas = feeTx.GetGas()
			}

			if req.MaxGas > 0 && gas > req.MaxGas-totalGas {
				break
			}
			if !tracker.gossiped(encCache.HashTx(tx, bz), now) {
				deduped++
				continue
			}
			txs = append(txs, bz)
			totalBytes += size
			totalGas += gas
			if maxPerReap > 0 && len(txs) >= maxPerReap {
				break
			}
		}
		tracker.prune(now)
		if cacheHits > 0 {
			metrics.IncrCounterWithLabels([]string{"cronos", "mempool", "reap", "encode_cache", "hit"}, cacheHits, nil)
		}
		if cacheMiss > 0 {
			metrics.IncrCounterWithLabels([]string{"cronos", "mempool", "reap", "encode_cache", "miss"}, cacheMiss, nil)
		}
		if len(txs) > 0 {
			metrics.IncrCounterWithLabels([]string{"cronos", "mempool", "reap", "gossip", "sent"}, float32(len(txs)), nil)
		}
		if deduped > 0 {
			metrics.IncrCounterWithLabels([]string{"cronos", "mempool", "reap", "gossip", "deduped"}, deduped, nil)
		}
		return &abci.ResponseReapTxs{Txs: txs}, nil
	}
}

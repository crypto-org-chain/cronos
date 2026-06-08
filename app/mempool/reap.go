package mempool

import (
	"context"
	"crypto/sha256"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

const TypeApp = "app"

// NewReapTxsHandler scans the app mempool to gather txs to be gossiped to other peers,
// stopping at MaxBytes/MaxGas (0 = no cap, per CometBFT convention). Prefix scan: breaks
// at the first tx over a cap (not best-fit), so a large high-priority tx may
// leave budget unused. Encoder errors skip the tx; encCache hits skip proto.Marshal.
func NewReapTxsHandler(mpool mempool.Mempool, txEncoder sdk.TxEncoder, encCache *EncoderCache, ttl time.Duration, maxPerReap int, logger log.Logger) sdk.ReapTxsHandler {
	tracker := newGossipTracker(ttl, nil)
	return func(req *abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
		snapshot := SnapshotPool(context.Background(), mpool)

		now := tracker.now()
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
			if !tracker.gossip(sha256.Sum256(bz), now) {
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
			telemetry.IncrCounter(cacheHits, "cronos", "mempool", "reap", "encode_cache", "hit") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
		}
		if cacheMiss > 0 {
			telemetry.IncrCounter(cacheMiss, "cronos", "mempool", "reap", "encode_cache", "miss") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
		}
		if len(txs) > 0 {
			telemetry.IncrCounter(float32(len(txs)), "cronos", "mempool", "reap", "gossip", "sent") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
		}
		if deduped > 0 {
			telemetry.IncrCounter(deduped, "cronos", "mempool", "reap", "gossip", "deduped") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
		}
		return &abci.ResponseReapTxs{Txs: txs}, nil
	}
}

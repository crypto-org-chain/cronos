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

// NewReapTxsHandler drains the priority mempool for mempool.type=app, stopping
// at MaxBytes/MaxGas (0 = no cap, per CometBFT convention). Prefix scan: breaks
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
			// Skip txs gossiped within ttl. Hash matches CometBFT types.Tx.Key()
			// (sha256 of the wire bytes) so send/receive dedup agree on identity.
			// Only included txs are marked; a tx dropped below by the count cap
			// stays eligible next tick.
			if !tracker.markAndAllow(sha256.Sum256(bz), now) {
				deduped++
				continue
			}
			txs = append(txs, bz)
			totalBytes += size
			totalGas += gas
			// Count cap spreads a large pool across reap ticks. byte/gas caps
			// above still bound the batch; this bounds the count.
			if maxPerReap > 0 && len(txs) >= maxPerReap {
				break
			}
		}
		tracker.prune(now)
		// Emit cache hit/miss once per reap (not per tx) so operators can watch
		// the proto.Marshal fallback rate climb when pool depth exceeds the
		// encoder-cache size. No-op unless telemetry is enabled.
		if cacheHits > 0 {
			telemetry.IncrCounter(cacheHits, "cronos", "mempool", "reap", "encode_cache", "hit") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
		}
		if cacheMiss > 0 {
			telemetry.IncrCounter(cacheMiss, "cronos", "mempool", "reap", "encode_cache", "miss") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
		}
		// gossip.sent vs gossip.deduped: steady-state deduped should dominate
		// once the pool is resident (re-gossip only after ttl).
		if len(txs) > 0 {
			telemetry.IncrCounter(float32(len(txs)), "cronos", "mempool", "reap", "gossip", "sent") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
		}
		if deduped > 0 {
			telemetry.IncrCounter(deduped, "cronos", "mempool", "reap", "gossip", "deduped") //nolint:staticcheck // telemetry wrapper deprecated upstream but is the canonical metrics API
		}
		return &abci.ResponseReapTxs{Txs: txs}, nil
	}
}
